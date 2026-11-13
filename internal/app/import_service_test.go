package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"minidb/internal/database"
	"minidb/internal/dataimport"
)

func newImportTestEnv(t *testing.T, readOnly bool) (*ImportService, *database.Manager) {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "i.db"), ReadOnly: readOnly}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	db, _ := manager.GetDB("c1")
	if _, err := db.Exec(`CREATE TABLE People (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`); err != nil {
		t.Fatal(err)
	}
	return NewImportService(manager), manager
}

func waitImportDone(t *testing.T, s *ImportService, taskID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		_, running := s.cancelMap[taskID]
		s.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("导入任务超时")
}

func TestImportServiceCSVEndToEnd(t *testing.T) {
	s, manager := newImportTestEnv(t, false)
	file := filepath.Join(t.TempDir(), "p.csv")
	os.WriteFile(file, []byte("编号,姓名,年龄\n1,Ann,30\n2,Bob,\n"), 0o600)

	// 目标列用小写 "name"，应被规范成真实列名 name；"AGE" 同理
	id, err := s.StartImport(ImportRequest{
		ConnID: "c1", Table: "People", FilePath: file,
		Parse:   dataimport.Options{HasHeader: true},
		Columns: []database.ImportColumn{{Source: 0, Target: "ID"}, {Source: 1, Target: "name"}, {Source: 2, Target: "AGE"}},
		Import:  database.ImportOptions{Atomic: true, EmptyAsNull: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, s, id)

	db, _ := manager.GetDB("c1")
	var n, nulls int
	db.QueryRow(`SELECT COUNT(*) FROM People`).Scan(&n)
	db.QueryRow(`SELECT COUNT(*) FROM People WHERE age IS NULL`).Scan(&nulls)
	if n != 2 || nulls != 1 {
		t.Fatalf("rows=%d nullAge=%d", n, nulls)
	}
}

func TestImportServiceRejectsUnknownColumnAndReadOnly(t *testing.T) {
	s, _ := newImportTestEnv(t, false)
	file := filepath.Join(t.TempDir(), "p.csv")
	os.WriteFile(file, []byte("a\n1\n"), 0o600)
	_, err := s.StartImport(ImportRequest{
		ConnID: "c1", Table: "People", FilePath: file, Parse: dataimport.Options{HasHeader: true},
		Columns: []database.ImportColumn{{Source: 0, Target: "nope"}},
	})
	if err == nil {
		t.Fatal("不存在的目标列应被拒绝")
	}

	ro, _ := newImportTestEnv(t, true)
	_, err = ro.StartImport(ImportRequest{
		ConnID: "c1", Table: "People", FilePath: file, Parse: dataimport.Options{HasHeader: true},
		Columns: []database.ImportColumn{{Source: 0, Target: "id"}},
	})
	if err == nil {
		t.Fatal("只读连接应拒绝导入")
	}
}
