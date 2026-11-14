package app

import (
	"compress/gzip"
	"context"
	"fmt"
	"minidb/internal/database"
	"minidb/internal/export"
	"minidb/internal/logger"
	"minidb/internal/model"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 流式导出每批查询行数
const exportBatchSize = 5000

// ExportProgressEvent 导出进度事件（通过 Bus 推送给界面）
type ExportProgressEvent struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`   // "progress" | "done" | "error" | "cancelled"
	Current  int64  `json:"current"`  // 已导出行数
	Total    int64  `json:"total"`    // 总行数（可能为 0 表示未知）
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 完整路径
	Error    string `json:"error"`    // 错误信息
}

// ExportRequest 描述一次导出：一张或多张表，或一条查询的结果。
type ExportRequest struct {
	ConnID   string
	Database string
	Tables   []string // 导出表；多于一张时每张表一个文件
	Query    string   // 非空时导出这条语句的结果，忽略 Tables
	Options  export.Options
	// Pattern 是多表导出的文件名模板，可用 {table} {db} {date} {time} {datetime}；空为 {table}_{datetime}。
	Pattern string
	// Limit 大于 0 时每个文件最多写这么多行（取前 N 行）。
	Limit int64
	// Path 为空时弹出保存对话框；单文件时是文件路径，多表时是目录。
	Path string
}

// ExportService 导出服务
type ExportService struct {
	env     *Env
	manager *database.Manager

	// 取消导出任务支持
	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

// NewExportService 创建导出服务
func NewExportService(manager *database.Manager) *ExportService {
	return &ExportService{
		manager:   manager,
		cancelMap: make(map[string]context.CancelFunc),
	}
}

// SetEnv 设置平台环境（在启动时调用）
func (s *ExportService) SetEnv(env *Env) {
	s.env = env
}

// CancelExport 取消导出任务
func (s *ExportService) CancelExport(taskID string) {
	s.mu.Lock()
	cancel, ok := s.cancelMap[taskID]
	s.mu.Unlock()
	if ok {
		logger.Info("[ExportService] 取消导出任务: %s", taskID)
		cancel()
	}
}

// OpenExportedFile 打开已导出的文件
func (s *ExportService) OpenExportedFile(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return model.Errf(model.CodeFilePathEmpty, "文件路径为空")
	}
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("[ExportService] 打开导出文件失败，文件不存在: %s err=%v", filePath, err)
		return err
	}
	logger.Info("[ExportService] 打开导出文件: %s", filePath)
	if s.env != nil && s.env.Host != nil {
		return s.env.Host.OpenPath(filePath)
	}
	return exec.Command("open", filePath).Start()
}

// exportJob 是一个要写出的文件：一张表或一条查询。
type exportJob struct {
	table string
	query string
	path  string
}

// ExpandPattern 把文件名模板里的占位符换掉，并去掉路径分隔符。
func ExpandPattern(pattern, table, db string, now time.Time) string {
	if strings.TrimSpace(pattern) == "" {
		pattern = "{table}_{datetime}"
	}
	r := strings.NewReplacer(
		"{table}", table, "{db}", db,
		"{date}", now.Format("20060102"), "{time}", now.Format("150405"),
		"{datetime}", now.Format("20060102_150405"),
	)
	name := r.Replace(pattern)
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(name)
}

// StartExport 开始导出：先定下保存位置（必要时弹窗），再在后台分批查询、写入并推送进度。
// 返回任务 ID；空串表示用户取消了位置选择。
func (s *ExportService) StartExport(req ExportRequest) (string, error) {
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	if req.Query == "" && len(req.Tables) == 0 {
		return "", fmt.Errorf("没有要导出的内容")
	}
	if req.Options.Format == "" {
		req.Options.Format = "csv"
	}
	ext := req.Options.Extension()
	now := time.Now()

	var jobs []exportJob
	var label string
	switch {
	case req.Query != "":
		path, err := s.target(req.Path, "query_result", ext)
		if path == "" || err != nil {
			return "", err
		}
		jobs = []exportJob{{table: "query_result", query: req.Query, path: path}}
		label = filepath.Base(path)
	case len(req.Tables) == 1:
		path, err := s.target(req.Path, req.Tables[0], ext)
		if path == "" || err != nil {
			return "", err
		}
		jobs = []exportJob{{table: req.Tables[0], path: path}}
		label = filepath.Base(path)
	default:
		dir := req.Path
		if dir == "" {
			picked, err := s.getSavePath("export", ext)
			if picked == "" || err != nil {
				return "", err
			}
			dir = filepath.Dir(picked)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		used := map[string]int{}
		for _, t := range req.Tables {
			name := ExpandPattern(req.Pattern, t, req.Database, now)
			used[name]++
			if n := used[name]; n > 1 {
				name = fmt.Sprintf("%s_%d", name, n)
			}
			jobs = append(jobs, exportJob{table: t, path: filepath.Join(dir, name+"."+ext)})
		}
		label = dir
	}

	var total int64
	for _, j := range jobs {
		var n int64
		if j.query != "" {
			n = s.queryRowCount(req.ConnID, req.Database, j.query)
		} else if c, err := s.getTableRowCount(req.ConnID, req.Database, j.table); err == nil {
			n = c
		}
		if req.Limit > 0 && n > req.Limit {
			n = req.Limit
		}
		total += n
	}

	taskID := fmt.Sprintf("export_%s_%d", jobs[0].table, now.UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.run(ctx, taskID, req, cfg, jobs, label, total)
	return taskID, nil
}

// target 返回单文件导出的保存路径：给了就用，否则弹保存对话框。
func (s *ExportService) target(path, name, ext string) (string, error) {
	if path != "" {
		return path, nil
	}
	return s.getSavePath(name, ext)
}

func (s *ExportService) queryRowCount(connID, dbName, sqlStr string) int64 {
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return 0
	}
	db, err := s.manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return 0
	}
	var n int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __export_count__", strings.TrimRight(strings.TrimSpace(sqlStr), ";"))
	err = database.WithSession(context.Background(), db, cfg.Type, dbName, func(session *database.Session) error {
		return session.QueryRow(countSQL).Scan(&n)
	})
	if err != nil {
		logger.Warn("[ExportService] 获取查询结果总行数失败: %v", err)
		return 0
	}
	return n
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
	file, err := os.Create(path)
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

// getTableRowCount 获取表的总行数
func (s *ExportService) getTableRowCount(connID, dbName, tableName string) (int64, error) {
	db, err := s.manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return 0, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return 0, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	session, err := database.OpenSession(context.Background(), db, cfg.Type, dbName)
	if err != nil {
		return 0, err
	}
	defer session.Close()

	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == database.Postgres {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", quotedTable)
	var count int64
	if err := session.QueryRow(countSQL).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// detectCursorColumn 探测可用于 keyset 分页的单列主键。
// 仅当表存在且只存在一个主键列时返回该列名；无主键或组合主键返回空字符串（调用方回退 OFFSET）。
func (s *ExportService) detectCursorColumn(db database.Executor, cfg *database.ConnectionConfig, dbName, tableName string) string {
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	cols, err := database.GetColumns(ctx, db, cfg.Type, dbName, tableName)
	if err != nil {
		logger.Warn("[ExportService] 探测主键失败，回退 OFFSET 分页: %v", err)
		return ""
	}
	var pk string
	for _, c := range cols {
		if !c.IsPrimary {
			continue
		}
		if pk != "" {
			return "" // 组合主键不适合 keyset，回退 OFFSET
		}
		pk = c.Name
	}
	return pk
}

// getTableColumns 获取表的列名列表
func (s *ExportService) getTableColumns(db database.Executor, cfg *database.ConnectionConfig, dbName, tableName string) ([]string, error) {
	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == database.Postgres {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	// LIMIT 0 只取元数据，不传输任何数据行
	probeSQL := fmt.Sprintf("SELECT * FROM %s LIMIT 0", quotedTable)
	rows, err := db.Query(probeSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	return colNames, nil
}

// emitExportProgress 向前端推送导出进度事件
func (s *ExportService) emitExportProgress(event ExportProgressEvent) {
	if s.env == nil {
		logger.Warn("[ExportService] 推送进度时 Wails 应用实例为空")
		return
	}
	s.env.Emit(TopicExport, event)
}

func (s *ExportService) getSavePath(tableName, ext string) (string, error) {
	name := fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext)
	if s.env != nil && s.env.Host != nil {
		return s.env.Host.SaveFile(SaveOptions{Title: "导出数据", Filename: name})
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop", name), nil
}
