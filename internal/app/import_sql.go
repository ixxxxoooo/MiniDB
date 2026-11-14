package app

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"minidb/internal/model"
	"os"
	"path/filepath"
	"strings"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/sqltext"
)

// maxSQLStatementSize 是单条语句的大小上限。转储文件本身不限大小——语句是流式一条一条切出来的，
// 内存里只有当前这一条；上限只是防止一条没有结尾分号的"语句"把内存吃光。
const maxSQLStatementSize = 256 << 20

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

// walkSQLStatements 打开转储文件（含 .gz），流式地把每条语句交给 fn；fn 返回错误就停下。
func walkSQLStatements(path string, fn func(sqltext.Statement) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("读取压缩文件失败: %w", err)
		}
		defer gz.Close()
		r = gz
	}
	br := bufio.NewReader(r)
	if head, _ := br.Peek(3); string(head) == "\ufeff" {
		_, _ = br.Discard(3) // UTF-8 BOM
	}
	return sqltext.SplitReader(br, sqltext.EditorLexOptions, maxSQLStatementSize, fn)
}

// PreviewSQLFile 读出 SQL 转储里的语句条数、开头几条和危险语句。
func (s *ImportService) PreviewSQLFile(path string) (*SQLFilePreview, error) {
	defer logger.Perf("app.Imports.PreviewSQLFile")()
	p := &SQLFilePreview{}
	err := walkSQLStatements(path, func(st sqltext.Statement) error {
		if p.Count < 5 {
			p.Head = append(p.Head, st.Text)
		}
		p.Count++
		switch database.SQLLeadingVerb(st.Text) {
		case "drop", "truncate", "delete", "update", "alter":
			if len(p.Risky) < 50 {
				p.Risky = append(p.Risky, st.Text)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if p.Count == 0 {
		return nil, errors.New("文件里没有 SQL 语句")
	}
	return p, nil
}

// countSQLStatements 流式数一遍语句条数（进度条要总数）；只在内存里留当前一条。
func countSQLStatements(path string) (int64, error) {
	var n int64
	err := walkSQLStatements(path, func(sqltext.Statement) error { n++; return nil })
	return n, err
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
	defer logger.Perf("app.Imports.StartSQLImport")()
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	if err := ensureWritable(cfg); err != nil {
		return "", err
	}
	total, err := countSQLStatements(req.FilePath)
	if err != nil {
		return "", err
	}
	if total == 0 {
		return "", errors.New("文件里没有 SQL 语句")
	}
	taskID := fmt.Sprintf("import_sql_%d", time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.runSQL(ctx, taskID, req, total)
	return taskID, nil
}

func (s *ImportService) runSQL(ctx context.Context, taskID string, req SQLImportRequest, total int64) {
	defer logger.Perf("job.import.sql", "conn", req.ConnID)()
	defer func() {
		s.mu.Lock()
		if cancel, ok := s.cancelMap[taskID]; ok {
			cancel()
			delete(s.cancelMap, taskID)
		}
		s.mu.Unlock()
	}()
	name := filepath.Base(req.FilePath)
	s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Total: total, FileName: name})

	var done int64
	var last time.Time
	err := withSession(ctx, s.manager, req.ConnID, req.Database, func(session *database.Session, cfg *database.ConnectionConfig) error {
		return walkSQLStatements(req.FilePath, func(stmt sqltext.Statement) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			st := stmt.Text
			if _, err := session.Exec(st); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("第 %d 条语句执行失败（前面 %d 条已执行）: %w\n%s", done+1, done, err, snippet(st))
			}
			done++
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Current: done, Total: total, FileName: name})
			}
			return nil
		})
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
