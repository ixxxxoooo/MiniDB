package app

import (
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"minidb/internal/database"
	"minidb/internal/export"
)

// 收集导出事件，等到任务结束。
type exportCollector struct {
	mu   sync.Mutex
	last ExportProgressEvent
}

func newExportTestEnv(t *testing.T) (*ExportService, *exportCollector) {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "e.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	db, _ := manager.GetDB("c1")
	for _, q := range []string{
		`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT, note TEXT)`,
		`INSERT INTO people VALUES (1,'Ann','it''s'),(2,'Bob',NULL)`,
		`CREATE TABLE pets (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO pets VALUES (1,'Rex')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s := NewExportService(manager)
	col := &exportCollector{}
	bus := NewBus()
	bus.On(TopicExport, func(p any) {
		col.mu.Lock()
		col.last = p.(ExportProgressEvent)
		col.mu.Unlock()
	})
	s.SetEnv(&Env{Bus: bus})
	return s, col
}

func (c *exportCollector) wait(t *testing.T) ExportProgressEvent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		ev := c.last
		c.mu.Unlock()
		if ev.Status == "done" || ev.Status == "error" || ev.Status == "cancelled" {
			return ev
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("导出超时")
	return ExportProgressEvent{}
}

func TestExportSQLGzipWithStructure(t *testing.T) {
	s, col := newExportTestEnv(t)
	path := filepath.Join(t.TempDir(), "p.sql.gz")
	_, err := s.StartExport(ExportRequest{
		ConnID: "c1", Tables: []string{"people"}, Path: path,
		Options: export.Options{Format: "sql", Gzip: true, SQL: export.SQLOptions{Structure: true, DropTable: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev := col.wait(t); ev.Status != "done" || ev.Current != 2 {
		t.Fatalf("事件 %+v", ev)
	}
	f, _ := os.Open(path)
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(zr)
	got := string(b)
	for _, w := range []string{`DROP TABLE IF EXISTS "people";`, "CREATE TABLE", `INSERT INTO "people" ("id", "name", "note") VALUES (1, 'Ann', 'it''s');`, "VALUES (2, 'Bob', NULL);"} {
		if !strings.Contains(got, w) {
			t.Errorf("缺少 %q\n%s", w, got)
		}
	}
}

func TestExportMultiTableXLSXAndColumns(t *testing.T) {
	s, col := newExportTestEnv(t)
	dir := filepath.Join(t.TempDir(), "out")
	_, err := s.StartExport(ExportRequest{
		ConnID: "c1", Tables: []string{"people", "pets"}, Path: dir, Pattern: "{db}x_{table}",
		Options: export.Options{Format: "xlsx"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := col.wait(t)
	if ev.Status != "done" || ev.Current != 3 || ev.FilePath != dir {
		t.Fatalf("事件 %+v", ev)
	}
	for _, name := range []string{"x_people.xlsx", "x_pets.xlsx"} {
		zr, err := zip.OpenReader(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		zr.Close()
	}

	// 单表只导出选中的列
	path := filepath.Join(t.TempDir(), "c.csv")
	if _, err := s.StartExport(ExportRequest{
		ConnID: "c1", Tables: []string{"people"}, Path: path,
		Options: export.Options{Format: "csv", Columns: []string{"name"}, CSV: export.CSVOptions{NoBOM: true}},
	}); err != nil {
		t.Fatal(err)
	}
	col.wait(t)
	time.Sleep(50 * time.Millisecond)
	b, _ := os.ReadFile(path)
	if string(b) != "name\nAnn\nBob\n" {
		t.Fatalf("got %q", b)
	}
}

func TestExportQueryAndFailureLeavesNoFile(t *testing.T) {
	s, col := newExportTestEnv(t)
	path := filepath.Join(t.TempDir(), "q.json")
	if _, err := s.StartExport(ExportRequest{
		ConnID: "c1", Query: "SELECT name FROM people ORDER BY id", Path: path,
		Options: export.Options{Format: "json"},
	}); err != nil {
		t.Fatal(err)
	}
	if ev := col.wait(t); ev.Status != "done" {
		t.Fatalf("事件 %+v", ev)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"name": "Bob"`) {
		t.Fatalf("got %s", b)
	}

	bad := filepath.Join(t.TempDir(), "bad.csv")
	col.mu.Lock()
	col.last = ExportProgressEvent{}
	col.mu.Unlock()
	if _, err := s.StartExport(ExportRequest{ConnID: "c1", Tables: []string{"nope"}, Path: bad, Options: export.Options{Format: "csv"}}); err != nil {
		t.Fatal(err)
	}
	if ev := col.wait(t); ev.Status != "error" {
		t.Fatalf("事件 %+v", ev)
	}
	if _, err := os.Stat(bad); err == nil {
		t.Error("失败后不应留下文件")
	}
}

func TestExpandPattern(t *testing.T) {
	now := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if got := ExpandPattern("{db}/{table}-{date}-{time}", "t", "d", now); got != "d_t-20240304-050607" {
		t.Fatal(got)
	}
	if got := ExpandPattern("", "t", "d", now); got != "t_20240304_050607" {
		t.Fatal(got)
	}
}
