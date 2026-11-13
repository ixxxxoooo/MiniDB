package app

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/sqltext"
)

// maxSQLDumpSize 是能导入的 SQL 文件大小上限；文件会整个读进内存来拆分语句。
const maxSQLDumpSize = 512 << 20

// IsSQLFile 按扩展名判断是不是 SQL 转储（含 .sql.gz）。
func IsSQLFile(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".sql") || strings.HasSuffix(p, ".sql.gz")
}

// SQLFilePreview 是 SQL 转储的概况。
type SQLFilePreview struct {
	Count int      // 语句条数
	Head  []string // 前几条语句
	// Risky 是会改动已有数据或结构的语句（DROP、TRUNCATE、DELETE、UPDATE、ALTER），用于安全模式确认。
	Risky []string
}

func readSQLStatements(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > maxSQLDumpSize {
		return nil, fmt.Errorf("SQL 文件超过 %d MB，请用命令行工具导入", maxSQLDumpSize>>20)
	}
	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("读取压缩文件失败: %w", err)
		}
		defer gz.Close()
		r = gz
	}
	data, err := io.ReadAll(io.LimitReader(r, maxSQLDumpSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSQLDumpSize {
		return nil, fmt.Errorf("SQL 文件超过 %d MB，请用命令行工具导入", maxSQLDumpSize>>20)
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	var out []string
	for _, st := range sqltext.Split(text) {
		out = append(out, st.Text)
	}
	return out, nil
}

// PreviewSQLFile 读出 SQL 转储里的语句条数、开头几条和危险语句。
func (s *ImportService) PreviewSQLFile(path string) (*SQLFilePreview, error) {
	stmts, err := readSQLStatements(path)
	if err != nil {
		return nil, err
	}
	if len(stmts) == 0 {
		return nil, errors.New("文件里没有 SQL 语句")
	}
	p := &SQLFilePreview{Count: len(stmts)}
	for i, st := range stmts {
		if i < 5 {
			p.Head = append(p.Head, st)
		}
		switch database.SQLLeadingVerb(st) {
		case "drop", "truncate", "delete", "update", "alter":
			if len(p.Risky) < 50 {
				p.Risky = append(p.Risky, st)
			}
		}
	}
	return p, nil
}

// SQLImportRequest 是执行一个 SQL 转储文件的参数。
type SQLImportRequest struct {
	ConnID   string
	Database string
	FilePath string
}

// StartSQLImport 在后台按顺序执行文件里的每条语句，遇到第一个错误就停。
// 转储里常有 DDL，它们在多数数据库里不能回滚，所以不放进一个事务；已执行的语句会保留。
func (s *ImportService) StartSQLImport(req SQLImportRequest) (string, error) {
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", fmt.Errorf("连接配置不存在: %s", req.ConnID)
	}
	if err := ensureWritable(cfg); err != nil {
		return "", err
	}
	stmts, err := readSQLStatements(req.FilePath)
	if err != nil {
		return "", err
	}
	if len(stmts) == 0 {
		return "", errors.New("文件里没有 SQL 语句")
	}
	taskID := fmt.Sprintf("import_sql_%d", time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.runSQL(ctx, taskID, req, stmts)
	return taskID, nil
}

func (s *ImportService) runSQL(ctx context.Context, taskID string, req SQLImportRequest, stmts []string) {
	defer func() {
		s.mu.Lock()
		if cancel, ok := s.cancelMap[taskID]; ok {
			cancel()
			delete(s.cancelMap, taskID)
		}
		s.mu.Unlock()
	}()
	name := filepath.Base(req.FilePath)
	total := int64(len(stmts))
	s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Total: total, FileName: name})

	var done int64
	var last time.Time
	err := withSession(ctx, s.manager, req.ConnID, req.Database, func(session *database.Session, cfg *database.ConnectionConfig) error {
		for i, st := range stmts {
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, err := session.Exec(st); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("第 %d 条语句执行失败（前面 %d 条已执行）: %w\n%s", i+1, i, err, snippet(st))
			}
			done = int64(i + 1)
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Current: done, Total: total, FileName: name})
			}
		}
		return nil
	})
	switch {
	case err == nil:
		logger.Info("[ImportService] SQL 转储导入完成: statements=%d", done)
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "done", Current: done, Total: total, FileName: name})
	case errors.Is(err, context.Canceled):
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "cancelled", Current: done, Total: total, FileName: name})
	default:
		logger.Warn("[ImportService] SQL 转储导入失败: done=%d err=%v", done, err)
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "error", Current: done, Total: total, FileName: name, Error: err.Error()})
	}
}

// snippet 取语句开头一段用于报错。
func snippet(st string) string {
	st = strings.Join(strings.Fields(st), " ")
	if r := []rune(st); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return st
}
