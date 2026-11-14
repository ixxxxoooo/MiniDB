package ui

import (
	"database/sql"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
	_ "modernc.org/sqlite"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/internal/storage"
)

// env is a shell over a temp store and a SQLite database with a users table.
type env struct {
	t      *testing.T
	sh     *Shell
	tt     *mui.Tester
	cfg    model.ConnectionConfig
	dbPath string
}

type fakeHost struct{ open, save string }

func (h fakeHost) OpenFile(app.OpenOptions) (string, error) { return h.open, nil }
func (h fakeHost) SaveFile(app.SaveOptions) (string, error) { return h.save, nil }
func (h fakeHost) OpenPath(string) error                    { return nil }

func newEnv(t *testing.T) *env {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	storage.SetKeyProvider(storage.FileKeyProvider{})
	store, err := storage.OpenStore(filepath.Join(home, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(store, fakeHost{})
	t.Cleanup(func() { a.CloseConnections(); store.Close() })

	dbPath := filepath.Join(home, "data.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT, age INTEGER, bio TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total REAL)`,
		`CREATE INDEX idx_users_name ON users(name)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	// 450 行放进同一个事务里一次提交，避免逐条落盘拖慢每个用例
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 450; i++ {
		var email any = fmt.Sprintf("user%d@example.com", i)
		if i%10 == 0 {
			email = nil
		}
		if _, err := tx.Exec(`INSERT INTO users(name,email,age,bio) VALUES(?,?,?,?)`, fmt.Sprintf("用户%03d", i), email, 20+i%40, "line1\nline2"); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	db.Close()

	sh := NewShell(a, fakeHost{}, true)
	cfg := model.ConnectionConfig{ID: "c1", Name: "local-sqlite", Type: "sqlite", Database: dbPath, Color: "#0a84ff", Tag: "local"}
	if err := a.Connections.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	sh.loadConnections()
	tt := mui.NewTester(sh.view, 1280, 800)
	return &env{t: t, sh: sh, tt: tt, cfg: cfg, dbPath: dbPath}
}

func (e *env) shot(name string) {
	e.t.Helper()
	e.tt.Frame()
	dir := os.Getenv("MINIDB_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		e.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, e.tt.Image()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) has(s string) {
	e.t.Helper()
	e.tt.Frame()
	if !e.tt.HasText(s) {
		e.t.Fatalf("text %q not on screen; have %q", s, e.tt.Texts())
	}
}

func (e *env) hasNot(s string) {
	e.t.Helper()
	e.tt.Frame()
	if e.tt.HasText(s) {
		e.t.Fatalf("text %q should not be on screen", s)
	}
}

func sleepMs(n int) { time.Sleep(time.Duration(n) * time.Millisecond) }

func newCol(name, typ string) database.StructureColumnEdit {
	return database.StructureColumnEdit{UID: "new:" + name, Name: name, Type: typ, Nullable: true}
}

func newIdx(name, col string, unique bool) database.StructureIndexEdit {
	return database.StructureIndexEdit{UID: "new:" + name, Name: name, Columns: []string{col}, IsUnique: unique}
}

// rebuildWithHost returns the shell's app rewired to a host with canned dialogs.
func rebuildWithHost(e *env, h app.Host) *app.App {
	e.sh.app.SetHost(h)
	return e.sh.app
}

func appAIConfig() app.AIConfig { return app.AIConfig{BaseURL: "http://x/v1", APIKey: "k", Model: "m"} }
