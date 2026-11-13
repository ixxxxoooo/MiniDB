package app

import (
	"compress/gzip"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"minidb/internal/database"
)

// fakeDumpManager 让备份还原不需要真连一个 MySQL。
type fakeDumpManager struct {
	cfg *database.ConnectionConfig
}

func (f fakeDumpManager) GetConfig(string) (*database.ConnectionConfig, bool) { return f.cfg, true }
func (f fakeDumpManager) Endpoint(string) (string, int, bool)                 { return "127.0.0.1", 13306, true }
func (f fakeDumpManager) GetDBForDatabase(string, string) (*sql.DB, error)    { return nil, nil }

type dumpEvents struct {
	mu   sync.Mutex
	last DumpProgressEvent
}

func (d *dumpEvents) wait(t *testing.T) DumpProgressEvent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		ev := d.last
		d.mu.Unlock()
		if ev.Status == "done" || ev.Status == "error" || ev.Status == "cancelled" {
			return ev
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("备份/还原超时")
	return DumpProgressEvent{}
}

func newDumpTest(t *testing.T, cfg *database.ConnectionConfig) (*DumpService, *dumpEvents) {
	t.Helper()
	s := NewDumpService(fakeDumpManager{cfg: cfg})
	ev := &dumpEvents{}
	bus := NewBus()
	bus.On(TopicDump, func(p any) {
		ev.mu.Lock()
		ev.last = p.(DumpProgressEvent)
		ev.mu.Unlock()
	})
	s.SetEnv(&Env{Bus: bus})
	return s, ev
}

// writeTool 在目录里放一个假的命令行工具（shell 脚本）。
func writeTool(t *testing.T, name, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("用 shell 脚本模拟外部工具")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildCommandsKeepPasswordOffCommandLine(t *testing.T) {
	my := &database.ConnectionConfig{Type: "mysql", User: "root", Password: "s3cret"}
	c, err := buildDumpCommand(my, "h", 3306, "shop", DumpOptions{SchemaOnly: true, ExtraArgs: `--where="id > 5" --no-create-info`})
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(c.args, " ")
	if strings.Contains(line, "s3cret") || !strings.Contains(strings.Join(c.env, ","), "MYSQL_PWD=s3cret") {
		t.Fatalf("密码不能出现在命令行: %s %v", line, c.env)
	}
	for _, w := range []string{"--host h", "--port 3306", "--no-data", "--where=id > 5", "--no-create-info"} {
		if !strings.Contains(line, w) {
			t.Errorf("缺少 %q: %s", w, line)
		}
	}
	if c.args[len(c.args)-1] != "shop" {
		t.Error("库名应是最后一个参数")
	}
	pg := &database.ConnectionConfig{Type: "postgres", User: "u", Password: "pw", SSLMode: "require"}
	c, _ = buildDumpCommand(pg, "h", 5432, "app", DumpOptions{})
	if c.names[0] != "pg_dump" || !strings.Contains(strings.Join(c.args, " "), "--dbname app") || !strings.Contains(strings.Join(c.env, ","), "PGSSLMODE=require") {
		t.Fatalf("pg 命令: %+v", c)
	}
	if _, err := buildDumpCommand(&database.ConnectionConfig{Type: "starrocks"}, "h", 1, "d", DumpOptions{}); err == nil {
		t.Error("StarRocks 应不支持")
	}
	if got := splitArgs(`-a "b c" 'd e' f=""`); len(got) != 4 || got[1] != "b c" || got[2] != "d e" || got[3] != "f=" {
		t.Fatalf("splitArgs: %q", got)
	}
}

func TestFindToolMissingHasHint(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := findTool([]string{"definitely-not-a-tool"}, ""); err == nil || !strings.Contains(err.Error(), "definitely-not-a-tool") {
		t.Fatalf("err = %v", err)
	}
	if got := toolInstallHint("mysqldump"); !strings.Contains(got, "brew install mysql-client") {
		t.Fatalf("提示: %s", got)
	}
	if _, err := findTool([]string{"mysqldump"}, "/nonexistent/dir"); err == nil || !strings.Contains(err.Error(), "/nonexistent/dir") {
		t.Fatalf("指定路径无效时应说明路径: %v", err)
	}
}

func TestBackupAndRestoreWithFakeTools(t *testing.T) {
	cfg := &database.ConnectionConfig{ID: "c", Type: "mysql", User: "root", Password: "pw", Database: "shop"}
	s, ev := newDumpTest(t, cfg)

	dumpDir := writeTool(t, "mysqldump", `echo "-- dump of $MYSQL_PWD"; echo "CREATE TABLE t (id int);"`)
	out := filepath.Join(t.TempDir(), "shop.sql.gz")
	if _, err := s.StartBackup(DumpRequest{ConnID: "c", Options: DumpOptions{Tool: dumpDir, Gzip: true}, Path: out}); err != nil {
		t.Fatal(err)
	}
	if e := ev.wait(t); e.Status != "done" || e.Kind != "backup" || e.Current == 0 {
		t.Fatalf("备份事件: %+v", e)
	}
	f, _ := os.Open(out)
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(zr)
	f.Close()
	if !strings.Contains(string(b), "CREATE TABLE t") || !strings.Contains(string(b), "-- dump of pw") {
		t.Fatalf("备份内容: %s", b)
	}

	ev.mu.Lock()
	ev.last = DumpProgressEvent{}
	ev.mu.Unlock()
	// 还原：假的 mysql 把标准输入存到文件里，并检查库名参数
	got := filepath.Join(t.TempDir(), "stdin.txt")
	restoreDir := writeTool(t, "mysql", `for last; do :; done; echo "$last" > "`+got+`.db"; cat > "`+got+`"`)
	if _, err := s.StartRestore(RestoreRequest{ConnID: "c", Database: "shop_copy", FilePath: out, Tool: restoreDir}); err != nil {
		t.Fatal(err)
	}
	if e := ev.wait(t); e.Status != "done" || e.Kind != "restore" {
		t.Fatalf("还原事件: %+v", e)
	}
	in, _ := os.ReadFile(got)
	db, _ := os.ReadFile(got + ".db")
	if !strings.Contains(string(in), "CREATE TABLE t") || strings.TrimSpace(string(db)) != "shop_copy" {
		t.Fatalf("还原收到: %q db=%q", in, db)
	}
}

func TestBackupFailureReportsStderrAndCleansUp(t *testing.T) {
	cfg := &database.ConnectionConfig{ID: "c", Type: "mysql", User: "root", Database: "shop"}
	s, ev := newDumpTest(t, cfg)
	dir := writeTool(t, "mysqldump", `echo "Using a password on the command line interface can be insecure." >&2; echo "mysqldump: Got error: 1045: Access denied" >&2; exit 2`)
	out := filepath.Join(t.TempDir(), "x.sql")
	if _, err := s.StartBackup(DumpRequest{ConnID: "c", Options: DumpOptions{Tool: dir}, Path: out}); err != nil {
		t.Fatal(err)
	}
	e := ev.wait(t)
	if e.Status != "error" || !strings.Contains(e.Error, "Access denied") || strings.Contains(e.Error, "password on the command line") {
		t.Fatalf("事件: %+v", e)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("失败后不应留下备份文件")
	}
	// 没装工具：直接返回带安装提示的错误
	t.Setenv("PATH", t.TempDir())
	if _, err := s.StartBackup(DumpRequest{ConnID: "c", Options: DumpOptions{Tool: filepath.Join(t.TempDir(), "nope")}, Path: out}); err == nil {
		t.Fatal("工具不存在应报错")
	}
}

func TestBackupCancelKillsTool(t *testing.T) {
	cfg := &database.ConnectionConfig{ID: "c", Type: "mysql", User: "root", Database: "shop"}
	s, ev := newDumpTest(t, cfg)
	dir := writeTool(t, "mysqldump", `echo start; sleep 30`)
	out := filepath.Join(t.TempDir(), "c.sql")
	id, err := s.StartBackup(DumpRequest{ConnID: "c", Options: DumpOptions{Tool: dir}, Path: out})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	s.Cancel(id)
	if e := ev.wait(t); e.Status != "cancelled" {
		t.Fatalf("事件: %+v", e)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("取消后不应留下备份文件")
	}
}

func TestSQLiteBackupIsADatabaseCopy(t *testing.T) {
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "s.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	db, _ := manager.GetDB("c1")
	db.Exec(`CREATE TABLE a (x INTEGER)`)
	db.Exec(`INSERT INTO a VALUES (1),(2)`)

	s := NewDumpService(manager)
	ev := &dumpEvents{}
	bus := NewBus()
	bus.On(TopicDump, func(p any) {
		ev.mu.Lock()
		ev.last = p.(DumpProgressEvent)
		ev.mu.Unlock()
	})
	s.SetEnv(&Env{Bus: bus})
	out := filepath.Join(t.TempDir(), "copy.sqlite")
	if _, err := s.StartBackup(DumpRequest{ConnID: "c1", Path: out}); err != nil {
		t.Fatal(err)
	}
	if e := ev.wait(t); e.Status != "done" || e.Current == 0 {
		t.Fatalf("事件: %+v", e)
	}
	m2 := database.NewManager()
	if err := m2.Connect(&database.ConnectionConfig{ID: "c2", Name: "c", Type: "sqlite", Database: out}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m2.CloseAll)
	d2, _ := m2.GetDB("c2")
	var n int
	d2.QueryRow(`SELECT COUNT(*) FROM a`).Scan(&n)
	if n != 2 {
		t.Fatalf("副本里有 %d 行", n)
	}
	if _, err := s.StartRestore(RestoreRequest{ConnID: "c1", Database: "x", FilePath: out}); err == nil {
		t.Error("SQLite 不支持用工具还原")
	}
}
