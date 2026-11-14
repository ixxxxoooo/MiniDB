package app

import (
	"compress/gzip"
	"context"
	"fmt"
	"minidb/internal/database"
	"minidb/internal/export"
	"minidb/internal/logger"

	"os"

	"path/filepath"
	"strings"
)

// exportJob 是一个要写出的文件：一张表或一条查询。
type exportJob struct {
	table string
	query string
	path  string
}

// run 在后台依次写出每个文件，并推送进度与结果。
func (s *ExportService) run(ctx context.Context, taskID string, req ExportRequest, cfg *database.ConnectionConfig, jobs []exportJob, label string, total int64) {
	defer func() {
		s.mu.Lock()
		delete(s.cancelMap, taskID)
		s.mu.Unlock()
	}()
	// 多表导出时「打开」指向所在目录，单文件指向文件本身
	reveal := jobs[0].path
	if len(jobs) > 1 {
		reveal = filepath.Dir(reveal)
	}
	emit := func(status string, cur int64, errMsg string) {
		s.emitExportProgress(ExportProgressEvent{
			TaskID: taskID, Status: status, Current: cur, Total: total,
			FileName: label, FilePath: reveal, Error: errMsg,
		})
	}
	emit("progress", 0, "")

	db, err := s.manager.GetDBForDatabase(req.ConnID, req.Database)
	if err != nil {
		emit("error", 0, fmt.Sprintf("获取连接失败: %v", err))
		return
	}
	session, err := database.OpenSession(ctx, db, cfg.Type, req.Database)
	if err != nil {
		emit("error", 0, err.Error())
		return
	}
	defer session.Close()

	var done int64
	for _, j := range jobs {
		err := s.exportJob(ctx, session, cfg, req, j, func(n int64) {
			done += n
			emit("progress", done, "")
		})
		if ctx.Err() != nil {
			emit("cancelled", done, "")
			return
		}
		if err != nil {
			emit("error", done, err.Error())
			return
		}
	}
	logger.Info("[ExportService] 导出完成: taskID=%s files=%d rows=%d", taskID, len(jobs), done)
	s.emitExportProgress(ExportProgressEvent{
		TaskID: taskID, Status: "done", Current: done, Total: done,
		FileName: label, FilePath: reveal,
	})
}

// openWriter 建立文件（按需套 gzip）和格式写入器，返回收尾函数。
func openWriter(path, table string, opts export.Options) (export.Writer, func() error, error) {
	file, err := createPrivateFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("创建文件失败: %w", err)
	}
	var out interface {
		Write([]byte) (int, error)
	} = file
	var gz *gzip.Writer
	if opts.Gzip && opts.Format != "xlsx" {
		gz = gzip.NewWriter(file)
		out = gz
	}
	w, err := export.NewWriter(out, table, opts)
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	closeAll := func() error {
		err := w.Close()
		if gz != nil {
			if e := gz.Close(); err == nil {
				err = e
			}
		}
		if e := file.Close(); err == nil {
			err = e
		}
		return err
	}
	return w, closeAll, nil
}

// exportJob 写出一个文件；progress 每写完一批调用一次。
func (s *ExportService) exportJob(ctx context.Context, session *database.Session, cfg *database.ConnectionConfig, req ExportRequest, j exportJob, progress func(int64)) (err error) {
	dbName := req.Database
	opts := req.Options
	opts.SQL.QuoteIdent = func(n string) string { return database.QuoteIdent(cfg.Type, n) }
	opts.SQL.BackslashEscape = database.IsMySQLCompatible(cfg.Type)

	var columns []string
	var fetch func(offset int, last interface{}) (string, []interface{})
	var cursorCol string
	if j.query != "" {
		clean := strings.TrimRight(strings.TrimSpace(j.query), ";")
		probe, err := session.Query(fmt.Sprintf("SELECT * FROM (%s) AS __probe__ LIMIT 1", clean))
		if err != nil {
			return fmt.Errorf("探测列失败: %w", err)
		}
		columns, _ = probe.Columns()
		probe.Close()
		if len(columns) == 0 {
			return fmt.Errorf("查询无返回列")
		}
		fetch = func(offset int, _ interface{}) (string, []interface{}) {
			return fmt.Sprintf("SELECT * FROM (%s) AS __batch__ LIMIT %d OFFSET %d", clean, exportBatchSize, offset), nil
		}
	} else {
		columns, err = s.getTableColumns(session, cfg, dbName, j.table)
		if err != nil {
			return fmt.Errorf("获取列信息失败: %w", err)
		}
		if opts.SQL.Structure {
			ddl, err := database.GetDDL(session.Context(), session, cfg.Type, dbName, j.table)
			if err != nil {
				return fmt.Errorf("获取建表语句失败: %w", err)
			}
			opts.SQL.DDL = ddl
		}
		quoted := database.QuoteTableName(cfg.Type, j.table)
		if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
			quoted = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quoted)
		}
		// 单列主键时用 keyset 分页，避免 OFFSET 越翻越慢；否则回退 OFFSET。
		cursorCol = s.detectCursorColumn(session, cfg, dbName, j.table)
		placeholder := "?"
		if cfg.Type == database.Postgres {
			placeholder = "$1"
		}
		fetch = func(offset int, last interface{}) (string, []interface{}) {
			if cursorCol == "" {
				return fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", quoted, exportBatchSize, offset), nil
			}
			qc := database.QuoteIdent(cfg.Type, cursorCol)
			if last == nil {
				return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", quoted, qc, exportBatchSize), nil
			}
			return fmt.Sprintf("SELECT * FROM %s WHERE %s > %s ORDER BY %s LIMIT %d", quoted, qc, placeholder, qc, exportBatchSize), []interface{}{last}
		}
		// 多表导出时每张表都导出全部列
		if len(req.Tables) <= 1 {
			columns = export.PickColumns(columns, opts.Columns)
		}
	}

	w, closeAll, err := openWriter(j.path, j.table, opts)
	if err != nil {
		return err
	}
	defer func() {
		cerr := closeAll()
		if err == nil {
			err = cerr
		}
		if err != nil || ctx.Err() != nil {
			os.Remove(j.path) // 失败或取消时不留下半截文件
		}
	}()
	if err = w.Begin(columns); err != nil {
		return err
	}

	offset := 0
	var last interface{}
	var written int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if req.Limit > 0 && written >= req.Limit {
			return nil
		}
		sqlStr, args := fetch(offset, last)
		var result *database.QueryResult
		if j.query != "" {
			result, err = database.ExecuteQueryRaw(session, sqlStr)
		} else {
			result, err = database.QueryRawWithArgs(session, sqlStr, args...)
		}
		if err != nil {
			return fmt.Errorf("查询失败(offset=%d): %w", offset, err)
		}
		if result.Error != "" {
			return fmt.Errorf("查询错误: %s", result.Error)
		}
		if result.Truncated {
			// 截断后 fetched < exportBatchSize 会被误判成"已读完"，导出文件悄悄少数据，必须报错
			return fmt.Errorf("单批结果超过内存上限(offset=%d)，请减少导出列或排除超大字段", offset)
		}
		if len(result.Rows) == 0 {
			return nil
		}
		batch := result.Rows
		fetched := len(batch)
		if req.Limit > 0 && written+int64(len(batch)) > req.Limit {
			batch = batch[:req.Limit-written] // 最后一批只要凑够上限的那几行
		}
		if err = w.WriteRows(columns, batch); err != nil {
			return fmt.Errorf("写入失败: %w", err)
		}
		written += int64(len(batch))
		progress(int64(len(batch)))
		if cursorCol != "" {
			last = result.Rows[fetched-1][cursorCol]
		}
		offset += exportBatchSize
		if fetched < exportBatchSize {
			return nil
		}
	}
}
