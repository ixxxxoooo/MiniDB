package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"minidb/internal/model"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"minidb/internal/database"
	"minidb/internal/dataimport"
	"minidb/internal/logger"
)

// ImportProgressEvent 导入进度事件（事件名 import:progress）
type ImportProgressEvent struct {
	TaskID     string `json:"taskId"`
	Status     string `json:"status"` // "progress" | "done" | "error" | "cancelled"
	Current    int64  `json:"current"`
	Total      int64  `json:"total"`
	FileName   string `json:"fileName"`
	Error      string `json:"error"`
	RolledBack bool   `json:"rolledBack"` // 失败/取消后事务已整体回滚
}

// ImportRequest 开始导入的参数
type ImportRequest struct {
	ConnID   string                  `json:"connId"`
	Database string                  `json:"database"`
	Table    string                  `json:"table"`
	FilePath string                  `json:"filePath"`
	Parse    dataimport.Options      `json:"parse"`
	Columns  []database.ImportColumn `json:"columns"`
	Import   database.ImportOptions  `json:"import"`
	// NewColumns 非空时先按它建表（表不能已存在），再导入到新表；此时 Columns 被忽略。
	NewColumns []ImportNewColumn `json:"newColumns"`
}

// ImportNewColumn 是新建表的一列：取源文件的第 Source 列，叫 Name，类型为 Kind（int、float、text）。
type ImportNewColumn struct {
	Source int    `json:"source"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

// ImportService 文件导入服务（CSV / TSV / JSON / JSON Lines → 已有表）
type ImportService struct {
	env     envRef
	manager *database.Manager

	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

func NewImportService(manager *database.Manager) *ImportService {
	return &ImportService{manager: manager, cancelMap: make(map[string]context.CancelFunc)}
}

func (s *ImportService) SetEnv(env *Env) { s.env.set(env) }

// PickImportFile 弹出文件选择框，返回路径；用户取消返回空串。
func (s *ImportService) PickImportFile() (string, error) {
	defer logger.Perf("app.Imports.PickImportFile")()
	if s.env.get() == nil || s.env.get().Host == nil {
		return "", model.Errf(model.CodeAppNotReady, "应用未初始化，无法打开文件选择对话框")
	}
	return s.env.get().Host.OpenFile(OpenOptions{
		Title: "选择要导入的文件",
		Filters: []FileFilter{
			{Name: "数据文件 (CSV / TSV / JSON / Excel / SQL)", Exts: []string{"csv", "tsv", "json", "jsonl", "ndjson", "xlsx", "xlsm", "txt", "sql", "gz"}},
		},
	})
}

// PreviewImportFile 读取列名、前 100 行与总行数，供界面做列映射。
func (s *ImportService) PreviewImportFile(filePath string, opts dataimport.Options) (*dataimport.Preview, error) {
	defer logger.Perf("app.Imports.PreviewImportFile")()
	if strings.TrimSpace(filePath) == "" {
		return nil, model.Errf(model.CodeFilePathEmpty, "文件路径为空")
	}
	return dataimport.BuildPreview(filePath, opts, 100)
}

// StartImport 校验后在后台导入，返回任务 ID；进度通过 import:progress 事件推送。
func (s *ImportService) StartImport(req ImportRequest) (string, error) {
	defer logger.Perf("app.Imports.StartImport")()
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	if err := ensureWritable(cfg); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Table) == "" || strings.TrimSpace(req.FilePath) == "" {
		return "", errors.New("表名或文件路径为空")
	}
	if len(req.NewColumns) > 0 {
		def := make([]database.NewTableColumn, len(req.NewColumns))
		req.Columns = make([]database.ImportColumn, len(req.NewColumns))
		for i, c := range req.NewColumns {
			def[i] = database.NewTableColumn{Name: c.Name, Kind: c.Kind}
			req.Columns[i] = database.ImportColumn{Source: c.Source, Target: strings.TrimSpace(c.Name)}
		}
		err := withSession(context.Background(), s.manager, req.ConnID, req.Database, func(session *database.Session, cfg *database.ConnectionConfig) error {
			ddl, e := database.BuildCreateImportTableSQL(cfg.Type, req.Database, req.Table, def)
			if e != nil {
				return e
			}
			_, e = session.Exec(ddl)
			return e
		})
		if err != nil {
			return "", fmt.Errorf("创建表失败: %w", err)
		}
		req.Import.Conflict = database.ConflictStop
	}
	if len(req.Columns) == 0 {
		return "", errors.New("请至少映射一列")
	}

	// 用表的真实列名校验并规范化映射（大小写不敏感匹配）
	var actual []database.ColumnInfo
	err := withSession(context.Background(), s.manager, req.ConnID, req.Database, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var e error
		actual, e = database.GetColumns(session.Context(), session, cfg.Type, req.Database, req.Table)
		return e
	})
	if err != nil {
		return "", fmt.Errorf("读取表结构失败: %w", err)
	}
	byLower := make(map[string]string, len(actual))
	for _, c := range actual {
		byLower[strings.ToLower(c.Name)] = c.Name
	}
	if req.Import.Conflict == database.ConflictUpdate && len(req.Import.Keys) == 0 {
		for _, c := range actual {
			if c.IsPrimary {
				req.Import.Keys = append(req.Import.Keys, c.Name)
			}
		}
	}
	cols := make([]database.ImportColumn, len(req.Columns))
	for i, c := range req.Columns {
		name, ok := byLower[strings.ToLower(c.Target)]
		if !ok {
			return "", fmt.Errorf("目标表不存在列: %s", c.Target)
		}
		cols[i] = database.ImportColumn{Source: c.Source, Target: name}
	}

	taskID := fmt.Sprintf("import_%s_%d", req.Table, time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()

	req.Columns = cols
	logger.Info("[ImportService] 开始导入: connID=%s db=%s table=%s columns=%d atomic=%v", req.ConnID, req.Database, req.Table, len(cols), req.Import.Atomic)
	go s.run(ctx, taskID, req)
	return taskID, nil
}

// CancelImport 取消导入；原子模式下已写入的行会回滚。
func (s *ImportService) CancelImport(taskID string) {
	s.mu.Lock()
	cancel, ok := s.cancelMap[taskID]
	s.mu.Unlock()
	if ok {
		cancel()
	}
}

func (s *ImportService) emit(ev ImportProgressEvent) {
	if s.env.get() != nil {
		s.env.get().Emit(TopicImport, ev)
	}
}

func (s *ImportService) run(ctx context.Context, taskID string, req ImportRequest) {
	defer logger.Perf("job.import", "conn", req.ConnID)()
	defer func() {
		s.mu.Lock()
		if cancel, ok := s.cancelMap[taskID]; ok {
			cancel()
			delete(s.cancelMap, taskID)
		}
		s.mu.Unlock()
	}()
	fileName := filepath.Base(req.FilePath)
	fail := func(msg string) {
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "error", FileName: fileName, Error: msg})
	}

	// 先数总行数用于进度条；再重新打开读取
	total := int64(0)
	if prev, err := dataimport.BuildPreview(req.FilePath, req.Parse, 0); err != nil {
		fail(err.Error())
		return
	} else {
		total = prev.TotalRows
	}
	reader, err := dataimport.Open(req.FilePath, req.Parse)
	if err != nil {
		fail(err.Error())
		return
	}
	defer reader.Close()
	for _, c := range req.Columns {
		if c.Source < 0 || c.Source >= len(reader.Columns()) {
			fail(fmt.Sprintf("源列下标越界: %d", c.Source))
			return
		}
	}

	s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Total: total, FileName: fileName})

	// 事件节流：每 150ms 至多推一次
	var last time.Time
	progress := func(done int64) {
		if time.Since(last) < 150*time.Millisecond && done < total {
			return
		}
		last = time.Now()
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "progress", Current: done, Total: total, FileName: fileName})
	}

	opts := req.Import
	opts.EmptyAsNull = opts.EmptyAsNull && dataimport.DetectFormatOf(req.FilePath, req.Parse) == dataimport.FormatCSV

	var res database.ImportResult
	err = withSession(ctx, s.manager, req.ConnID, req.Database, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var e error
		res, e = database.ImportRows(ctx, session, cfg.Type, req.Database, req.Table, req.Columns, reader.Next, opts, progress)
		return e
	})

	switch {
	case err == nil:
		logger.Info("[ImportService] 导入完成: table=%s rows=%d", req.Table, res.Inserted)
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "done", Current: res.Inserted, Total: total, FileName: fileName})
	case errors.Is(err, context.Canceled):
		logger.Info("[ImportService] 导入已取消: table=%s kept=%d rolledBack=%v", req.Table, res.Inserted, res.RolledBack)
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "cancelled", Current: res.Inserted, Total: total, FileName: fileName, RolledBack: res.RolledBack})
	default:
		if errors.Is(err, io.EOF) {
			err = errors.New("文件意外结束")
		}
		logger.Warn("[ImportService] 导入失败: table=%s kept=%d rolledBack=%v err=%v", req.Table, res.Inserted, res.RolledBack, err)
		s.emit(ImportProgressEvent{TaskID: taskID, Status: "error", Current: res.Inserted, Total: total, FileName: fileName, Error: err.Error(), RolledBack: res.RolledBack})
	}
}
