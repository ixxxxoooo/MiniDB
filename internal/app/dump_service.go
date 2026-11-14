package app

import (
	"compress/gzip"
	"context"
	"database/sql"

	"fmt"
	"io"
	"minidb/internal/model"
	"minidb/internal/sqltext"
	"os"

	"path/filepath"

	"strings"
	"sync"
	"sync/atomic"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
)

// DumpProgressEvent 是备份或还原的进度（话题 TopicDump）。
type DumpProgressEvent struct {
	TaskID   string
	Kind     string // "backup" | "restore"
	Status   string // "progress" | "done" | "error" | "cancelled"
	Current  int64  // 备份：已写出的字节；还原：已读入的字节
	Total    int64  // 还原时是文件大小；备份未知为 0
	FileName string
	FilePath string
	Error    string
}

// DumpOptions 是备份的选项。
type DumpOptions struct {
	Tool       string // 备份工具的路径或所在目录，空则自动查找
	ExtraArgs  string // 额外的命令行参数，按空白拆分，支持引号
	Gzip       bool   // 压缩成 .gz
	SchemaOnly bool   // 只备份结构，不含数据
}

// DumpRequest 开始一次备份。
type DumpRequest struct {
	ConnID   string
	Database string
	Options  DumpOptions
	// Path 为空时弹出保存对话框。
	Path string
	// Pattern 是默认文件名模板，可用 {db} {date} {time} {datetime}；空为 {db}_{datetime}。
	Pattern string
}

// dumpManager 是备份还原用到的那部分连接管理器，测试里可以换成假的。
type dumpManager interface {
	GetConfig(connID string) (*database.ConnectionConfig, bool)
	Endpoint(connID string) (string, int, bool)
	GetDBForDatabase(connID, dbName string) (*sql.DB, error)
}

// DumpService 调用 mysqldump、pg_dump 等外部工具做备份与还原。
type DumpService struct {
	env     envRef
	manager dumpManager

	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

func NewDumpService(manager dumpManager) *DumpService {
	return &DumpService{manager: manager, cancelMap: map[string]context.CancelFunc{}}
}

func (s *DumpService) SetEnv(env *Env) { s.env.set(env) }

func (s *DumpService) emit(ev DumpProgressEvent) {
	if s.env.get() != nil {
		s.env.get().Emit(TopicDump, ev)
	}
}

// Cancel 取消备份或还原，并结束外部进程。
func (s *DumpService) Cancel(taskID string) {
	s.mu.Lock()
	cancel, ok := s.cancelMap[taskID]
	s.mu.Unlock()
	if ok {
		cancel()
	}
}

// PickDumpFile 弹出文件选择框选备份文件；用户取消返回空串。
func (s *DumpService) PickDumpFile() (string, error) {
	if s.env.get() == nil || s.env.get().Host == nil {
		return "", model.Errf(model.CodeAppNotReady, "应用未初始化，无法打开文件选择对话框")
	}
	return s.env.get().Host.OpenFile(OpenOptions{
		Title:   "选择备份文件",
		Filters: []FileFilter{{Name: "SQL 备份 (.sql / .sql.gz)", Exts: []string{"sql", "gz"}}},
	})
}

// ---- 工具查找与命令行 ----

// ---- 备份 ----

// DefaultDumpName 是备份文件的默认名字（不含扩展名）。
func DefaultDumpName(pattern, db string, now time.Time) string {
	if strings.TrimSpace(pattern) == "" {
		pattern = "{db}_{datetime}"
	}
	return ExpandPattern(pattern, "", db, now)
}

// StartBackup 开始备份：先定下保存位置（必要时弹窗），再在后台运行外部工具。
// 返回任务 ID；空串表示用户取消了位置选择。
func (s *DumpService) StartBackup(req DumpRequest) (string, error) {
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	dbName := req.Database
	if dbName == "" {
		dbName = cfg.Database
	}
	if dbName == "" && !database.CapsOf(cfg.Type).LocalFile() {
		return "", model.Errf(model.CodeNoDBSelected, "没有选中的数据库")
	}
	ext := "sql"
	if database.CapsOf(cfg.Type).LocalFile() {
		ext = "sqlite"
	} else if req.Options.Gzip {
		ext = "sql.gz"
	}
	path := req.Path
	if path == "" {
		name := DefaultDumpName(req.Pattern, filepath.Base(dbName), time.Now())
		var err error
		if s.env.get() == nil || s.env.get().Host == nil {
			return "", model.Errf(model.CodeAppNotReady, "应用未初始化，无法打开保存对话框")
		}
		path, err = s.env.get().Host.SaveFile(SaveOptions{Title: "备份数据库", Filename: name + "." + ext})
		if err != nil || path == "" {
			return "", err
		}
	}

	var run func(ctx context.Context, w io.Writer) error
	if !database.CapsOf(cfg.Type).LocalFile() {
		host, port, _ := s.manager.Endpoint(req.ConnID)
		cmd, err := buildDumpCommand(cfg, host, port, dbName, req.Options)
		if err != nil {
			return "", err
		}
		tool, err := findTool(cmd.names, req.Options.Tool)
		if err != nil {
			return "", err
		}
		run = func(ctx context.Context, w io.Writer) error { return runTool(ctx, tool, cmd, nil, w) }
	}

	taskID := fmt.Sprintf("backup_%d", time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.runBackup(ctx, taskID, req, cfg, path, run)
	return taskID, nil
}

// countingWriter 统计写出的字节数。
type countingWriter struct {
	w io.Writer
	n atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))
	return n, err
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// watch 每隔一会儿报告一次计数，直到 stop 被关闭。
func watch(stop <-chan struct{}, count func() int64, report func(int64)) {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			report(count())
		}
	}
}

func (s *DumpService) finish(taskID string) {
	s.mu.Lock()
	if cancel, ok := s.cancelMap[taskID]; ok {
		cancel()
		delete(s.cancelMap, taskID)
	}
	s.mu.Unlock()
}

func (s *DumpService) runBackup(ctx context.Context, taskID string, req DumpRequest, cfg *database.ConnectionConfig, path string, run func(context.Context, io.Writer) error) {
	defer s.finish(taskID)
	name := filepath.Base(path)
	ev := func(status string, n int64, errMsg string) {
		s.emit(DumpProgressEvent{TaskID: taskID, Kind: "backup", Status: status, Current: n, FileName: name, FilePath: path, Error: errMsg})
	}
	ev("progress", 0, "")

	if database.CapsOf(cfg.Type).LocalFile() {
		// SQLite 的备份是一份数据库副本；VACUUM INTO 要求目标文件不存在
		err := s.backupSQLite(ctx, req, path)
		var size int64
		if st, e := os.Stat(path); e == nil {
			size = st.Size()
		}
		switch {
		case err == nil:
			ev("done", size, "")
		case ctx.Err() != nil:
			os.Remove(path)
			ev("cancelled", size, "")
		default:
			os.Remove(path)
			ev("error", size, err.Error())
		}
		return
	}

	file, err := createPrivateFile(path)
	if err != nil {
		ev("error", 0, fmt.Sprintf("创建文件失败: %v", err))
		return
	}
	var out io.Writer = file
	var gz *gzip.Writer
	if req.Options.Gzip {
		gz = gzip.NewWriter(file)
		out = gz
	}
	cw := &countingWriter{w: out}
	stop := make(chan struct{})
	go watch(stop, cw.n.Load, func(n int64) { ev("progress", n, "") })

	err = run(ctx, cw)
	close(stop)
	if gz != nil {
		if e := gz.Close(); err == nil {
			err = e
		}
	}
	if e := file.Close(); err == nil {
		err = e
	}
	size := cw.n.Load()
	switch {
	case err == nil:
		logger.Info("[DumpService] 备份完成: %s bytes=%d", name, size)
		ev("done", size, "")
	case ctx.Err() != nil:
		os.Remove(path)
		ev("cancelled", size, "")
	default:
		os.Remove(path)
		logger.Warn("[DumpService] 备份失败: %v", err)
		ev("error", size, err.Error())
	}
}

// backupSQLite 用 VACUUM INTO 写出一份一致的数据库副本。
func (s *DumpService) backupSQLite(ctx context.Context, req DumpRequest, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := s.manager.GetDBForDatabase(req.ConnID, req.Database)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "VACUUM INTO "+sqltext.QuoteString(path))
	return err
}

// ---- 还原 ----
