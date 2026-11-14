package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"minidb/internal/sqltext"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

// RestoreRequest 开始一次还原。
type RestoreRequest struct {
	ConnID   string
	Database string // 还原到的库（可以是刚新建的库）
	FilePath string
	Tool     string
	// ExtraArgs 同 DumpOptions.ExtraArgs。
	ExtraArgs string
}

// dumpManager 是备份还原用到的那部分连接管理器，测试里可以换成假的。
type dumpManager interface {
	GetConfig(connID string) (*database.ConnectionConfig, bool)
	Endpoint(connID string) (string, int, bool)
	GetDBForDatabase(connID, dbName string) (*sql.DB, error)
}

// DumpService 调用 mysqldump、pg_dump 等外部工具做备份与还原。
type DumpService struct {
	env     *Env
	manager dumpManager

	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

func NewDumpService(manager dumpManager) *DumpService {
	return &DumpService{manager: manager, cancelMap: map[string]context.CancelFunc{}}
}

func (s *DumpService) SetEnv(env *Env) { s.env = env }

func (s *DumpService) emit(ev DumpProgressEvent) {
	if s.env != nil {
		s.env.Emit(TopicDump, ev)
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
	if s.env == nil || s.env.Host == nil {
		return "", errors.New("应用未初始化，无法打开文件选择对话框")
	}
	return s.env.Host.OpenFile(OpenOptions{
		Title:   "选择备份文件",
		Filters: []FileFilter{{Name: "SQL 备份 (.sql / .sql.gz)", Exts: []string{"sql", "gz"}}},
	})
}

// ---- 工具查找与命令行 ----

// ErrToolMissing 表示没找到外部工具。
type ErrToolMissing struct {
	Tool string
	Hint string
}

func (e *ErrToolMissing) Error() string {
	return fmt.Sprintf("没有找到 %s。%s", e.Tool, e.Hint)
}

// toolDirs 是图形程序的 PATH 里常常缺少、但装了客户端工具后会出现的目录。
func toolDirs() []string {
	dirs := []string{
		"/opt/homebrew/bin", "/usr/local/bin",
		"/opt/homebrew/opt/mysql-client/bin", "/usr/local/opt/mysql-client/bin",
		"/opt/homebrew/opt/libpq/bin", "/usr/local/opt/libpq/bin",
		"/usr/local/mysql/bin", "/Applications/Postgres.app/Contents/Versions/latest/bin",
		"/usr/lib/postgresql/16/bin", "/usr/lib/postgresql/15/bin", "/usr/lib/postgresql/14/bin",
	}
	if runtime.GOOS == "windows" {
		dirs = nil
	}
	return dirs
}

// findTool 找到工具的完整路径。override 可以是文件，也可以是所在目录。
func findTool(names []string, override string) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		if st, err := os.Stat(override); err == nil {
			if !st.IsDir() {
				return override, nil
			}
			for _, n := range names {
				if p := filepath.Join(override, n); fileExists(p) {
					return p, nil
				}
			}
		}
		return "", &ErrToolMissing{Tool: names[0], Hint: "请检查设置里指定的路径：" + override}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	for _, d := range toolDirs() {
		for _, n := range names {
			if p := filepath.Join(d, n); fileExists(p) {
				return p, nil
			}
		}
	}
	return "", &ErrToolMissing{Tool: names[0], Hint: toolInstallHint(names[0])}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func toolInstallHint(tool string) string {
	switch tool {
	case "mysqldump", "mysql":
		return "请安装 MySQL 客户端（macOS：brew install mysql-client），或在备份选项里指定工具的位置。"
	case "pg_dump", "psql":
		return "请安装 PostgreSQL 客户端（macOS：brew install libpq），或在备份选项里指定工具的位置。"
	}
	return "请安装对应的客户端工具，或在备份选项里指定它的位置。"
}

// splitArgs 把一行参数按空白拆开，引号里的空白保留。
func splitArgs(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// dumpCommand 是要执行的外部命令。
type dumpCommand struct {
	names []string // 工具可能的文件名，按先后找
	args  []string
	env   []string // 追加的环境变量（密码走这里，不放命令行）
}

// buildDumpCommand 拼出备份命令。
func buildDumpCommand(cfg *database.ConnectionConfig, host string, port int, dbName string, o DumpOptions) (*dumpCommand, error) {
	extra := splitArgs(o.ExtraArgs)
	switch {
	case cfg.Type == database.MySQL || cfg.Type == database.TiDB:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--user", cfg.User,
			"--single-transaction", "--routines", "--triggers", "--default-character-set=utf8mb4"}
		if o.SchemaOnly {
			args = append(args, "--no-data")
		}
		args = append(append(args, extra...), dbName)
		return &dumpCommand{names: []string{"mysqldump", "mariadb-dump"}, args: args, env: []string{"MYSQL_PWD=" + cfg.Password}}, nil
	case cfg.Type == database.Postgres:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--username", cfg.User, "--no-password", "--format=plain"}
		if o.SchemaOnly {
			args = append(args, "--schema-only")
		}
		args = append(append(args, extra...), "--dbname", dbName)
		env := []string{"PGPASSWORD=" + cfg.Password}
		if cfg.SSLMode != "" {
			env = append(env, "PGSSLMODE="+cfg.SSLMode)
		}
		return &dumpCommand{names: []string{"pg_dump"}, args: args, env: env}, nil
	}
	return nil, fmt.Errorf("%s 不支持用外部工具备份", cfg.Type)
}

// buildRestoreCommand 拼出还原命令；SQL 从标准输入读入。
func buildRestoreCommand(cfg *database.ConnectionConfig, host string, port int, dbName, extraArgs string) (*dumpCommand, error) {
	extra := splitArgs(extraArgs)
	switch {
	case cfg.Type == database.MySQL || cfg.Type == database.TiDB:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--user", cfg.User, "--default-character-set=utf8mb4"}
		args = append(append(args, extra...), dbName)
		return &dumpCommand{names: []string{"mysql", "mariadb"}, args: args, env: []string{"MYSQL_PWD=" + cfg.Password}}, nil
	case cfg.Type == database.Postgres:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--username", cfg.User, "--no-password",
			"--quiet", "-v", "ON_ERROR_STOP=1"}
		args = append(append(args, extra...), "--dbname", dbName)
		env := []string{"PGPASSWORD=" + cfg.Password}
		if cfg.SSLMode != "" {
			env = append(env, "PGSSLMODE="+cfg.SSLMode)
		}
		return &dumpCommand{names: []string{"psql"}, args: args, env: env}, nil
	}
	return nil, fmt.Errorf("%s 不支持还原", cfg.Type)
}

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
		return "", fmt.Errorf("连接配置不存在: %s", req.ConnID)
	}
	dbName := req.Database
	if dbName == "" {
		dbName = cfg.Database
	}
	if dbName == "" && cfg.Type != database.SQLite {
		return "", errors.New("没有选中的数据库")
	}
	ext := "sql"
	if cfg.Type == database.SQLite {
		ext = "sqlite"
	} else if req.Options.Gzip {
		ext = "sql.gz"
	}
	path := req.Path
	if path == "" {
		name := DefaultDumpName(req.Pattern, filepath.Base(dbName), time.Now())
		var err error
		if s.env == nil || s.env.Host == nil {
			return "", errors.New("应用未初始化，无法打开保存对话框")
		}
		path, err = s.env.Host.SaveFile(SaveOptions{Title: "备份数据库", Filename: name + "." + ext})
		if err != nil || path == "" {
			return "", err
		}
	}

	var run func(ctx context.Context, w io.Writer) error
	if cfg.Type != database.SQLite {
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

	if cfg.Type == database.SQLite {
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

	file, err := os.Create(path)
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

// runTool 运行外部工具。stdin 非空时作为标准输入；stdout 写到 out；失败时返回带 stderr 的错误。
func runTool(ctx context.Context, tool string, c *dumpCommand, stdin io.Reader, out io.Writer) error {
	cmd := exec.CommandContext(ctx, tool, c.args...)
	cmd.Env = append(os.Environ(), c.env...)
	cmd.Stdin = stdin
	cmd.Stdout = out
	// 取消后外部工具若还有子进程攥着管道，也只等这么久就放手
	cmd.WaitDelay = 500 * time.Millisecond
	var stderr bytes.Buffer
	cmd.Stderr = &limitedBuffer{buf: &stderr, max: 8 << 10}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		// 工具对命令行密码的提醒不是错误原因
		var lines []string
		for _, l := range strings.Split(msg, "\n") {
			if !strings.Contains(l, "Using a password on the command line") {
				lines = append(lines, l)
			}
		}
		msg = strings.TrimSpace(strings.Join(lines, "\n"))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s 失败: %s", filepath.Base(tool), msg)
	}
	return nil
}

// limitedBuffer 只留开头的 max 字节，多的丢掉。
type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

// ---- 还原 ----

// restorePlan 是还原前检查出来的结果。
type restorePlan struct {
	tool string
	cmd  *dumpCommand
	size int64
}

// prepareRestore 检查还原能不能做：连接可写、类型支持、文件在、工具找得到。
func (s *DumpService) prepareRestore(req RestoreRequest) (*restorePlan, error) {
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", req.ConnID)
	}
	if err := ensureWritable(cfg); err != nil {
		return nil, err
	}
	if cfg.Type == database.SQLite {
		return nil, errors.New("SQLite 的备份就是数据库文件：把它添加为新连接即可打开；要导入 SQL 文件请用“导入文件”")
	}
	if req.Database == "" {
		return nil, errors.New("没有选中的数据库")
	}
	st, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, err
	}
	host, port, _ := s.manager.Endpoint(req.ConnID)
	cmd, err := buildRestoreCommand(cfg, host, port, req.Database, req.ExtraArgs)
	if err != nil {
		return nil, err
	}
	tool, err := findTool(cmd.names, req.Tool)
	if err != nil {
		return nil, err
	}
	return &restorePlan{tool: tool, cmd: cmd, size: st.Size()}, nil
}

// CheckRestore 只做还原前的检查，不动数据库；界面用它在新建数据库之前先发现问题。
func (s *DumpService) CheckRestore(req RestoreRequest) error {
	_, err := s.prepareRestore(req)
	return err
}

// StartRestore 开始还原：把 SQL 文件（可以是 .gz）交给 mysql 或 psql 执行。
func (s *DumpService) StartRestore(req RestoreRequest) (string, error) {
	plan, err := s.prepareRestore(req)
	if err != nil {
		return "", err
	}
	taskID := fmt.Sprintf("restore_%d", time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.runRestore(ctx, taskID, req, plan.tool, plan.cmd, plan.size)
	return taskID, nil
}

func (s *DumpService) runRestore(ctx context.Context, taskID string, req RestoreRequest, tool string, cmd *dumpCommand, size int64) {
	defer s.finish(taskID)
	name := filepath.Base(req.FilePath)
	ev := func(status string, n int64, errMsg string) {
		s.emit(DumpProgressEvent{TaskID: taskID, Kind: "restore", Status: status, Current: n, Total: size, FileName: name, FilePath: req.FilePath, Error: errMsg})
	}
	ev("progress", 0, "")
	f, err := os.Open(req.FilePath)
	if err != nil {
		ev("error", 0, err.Error())
		return
	}
	defer f.Close()
	cr := &countingReader{r: f}
	var in io.Reader = cr
	if strings.HasSuffix(strings.ToLower(req.FilePath), ".gz") {
		gz, err := gzip.NewReader(cr)
		if err != nil {
			ev("error", 0, fmt.Sprintf("读取压缩文件失败: %v", err))
			return
		}
		defer gz.Close()
		in = gz
	}
	stop := make(chan struct{})
	go watch(stop, cr.n.Load, func(n int64) { ev("progress", n, "") })
	err = runTool(ctx, tool, cmd, in, io.Discard)
	close(stop)
	switch {
	case err == nil:
		logger.Info("[DumpService] 还原完成: %s", name)
		ev("done", size, "")
	case ctx.Err() != nil:
		ev("cancelled", cr.n.Load(), "")
	default:
		logger.Warn("[DumpService] 还原失败: %v", err)
		ev("error", cr.n.Load(), err.Error())
	}
}
