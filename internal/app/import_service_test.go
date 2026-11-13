package app

import (
	"os"
	"path/filepath"
	"strings"
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

func TestImportServiceCreateNewTable(t *testing.T) {
	s, manager := newImportTestEnv(t, false)
	file := filepath.Join(t.TempDir(), "n.csv")
	os.WriteFile(file, []byte("id,price,label\n1,1.5,a\n2,2,b\n"), 0o600)
	id, err := s.StartImport(ImportRequest{
		ConnID: "c1", Table: "fresh", FilePath: file, Parse: dataimport.Options{HasHeader: true},
		NewColumns: []ImportNewColumn{{0, "id", "int"}, {1, "price", "float"}, {2, "label", "text"}},
		Import:     database.ImportOptions{Atomic: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, s, id)
	db, _ := manager.GetDB("c1")
	var n int
	var sum float64
	db.QueryRow(`SELECT COUNT(*), SUM(price) FROM fresh`).Scan(&n, &sum)
	if n != 2 || sum != 3.5 {
		t.Fatalf("rows=%d sum=%v", n, sum)
	}
	// 表已存在时不能再建
	if _, err := s.StartImport(ImportRequest{
		ConnID: "c1", Table: "fresh", FilePath: file, Parse: dataimport.Options{HasHeader: true},
		NewColumns: []ImportNewColumn{{0, "id", "int"}},
	}); err == nil {
		t.Fatal("表已存在应报错")
	}
}

func TestImportServiceUpdateUsesPrimaryKey(t *testing.T) {
	s, manager := newImportTestEnv(t, false)
	db, _ := manager.GetDB("c1")
	db.Exec(`INSERT INTO People VALUES (1,'Old',10)`)
	file := filepath.Join(t.TempDir(), "u.csv")
	os.WriteFile(file, []byte("id,name,age\n1,New,11\n2,Two,22\n"), 0o600)
	id, err := s.StartImport(ImportRequest{
		ConnID: "c1", Table: "People", FilePath: file, Parse: dataimport.Options{HasHeader: true},
		Columns: []database.ImportColumn{{Source: 0, Target: "id"}, {Source: 1, Target: "name"}, {Source: 2, Target: "age"}},
		Import:  database.ImportOptions{Atomic: true, Conflict: database.ConflictUpdate},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, s, id)
	var name string
	var n int
	db.QueryRow(`SELECT name FROM People WHERE id=1`).Scan(&name)
	db.QueryRow(`SELECT COUNT(*) FROM People`).Scan(&n)
	if name != "New" || n != 2 {
		t.Fatalf("name=%s n=%d", name, n)
	}
}

func TestImportSQLDump(t *testing.T) {
	s, manager := newImportTestEnv(t, false)
	dump := filepath.Join(t.TempDir(), "d.sql")
	os.WriteFile(dump, []byte("-- dump\nCREATE TABLE a (x INTEGER);\nINSERT INTO a VALUES (1);\nINSERT INTO a VALUES (2); -- ; in comment\nDROP TABLE IF EXISTS gone;\nINSERT INTO a VALUES ('a;b');\n"), 0o600)
	pv, err := s.PreviewSQLFile(dump)
	if err != nil || pv.Count != 5 || len(pv.Risky) != 1 || !IsSQLFile(dump) {
		t.Fatalf("预览 %+v %v", pv, err)
	}
	id, err := s.StartSQLImport(SQLImportRequest{ConnID: "c1", FilePath: dump})
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, s, id)
	db, _ := manager.GetDB("c1")
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM a`).Scan(&n)
	if n != 3 {
		t.Fatalf("rows=%d", n)
	}

	// 出错时停在出错的语句，前面的保留
	bad := filepath.Join(t.TempDir(), "b.sql")
	os.WriteFile(bad, []byte("INSERT INTO a VALUES (10);\nINSERT INTO nope VALUES (1);\nINSERT INTO a VALUES (11);\n"), 0o600)
	var got ImportProgressEvent
	bus := NewBus()
	bus.On(TopicImport, func(p any) { got = p.(ImportProgressEvent) })
	s.SetEnv(&Env{Bus: bus})
	id, err = s.StartSQLImport(SQLImportRequest{ConnID: "c1", FilePath: bad})
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, s, id)
	db.QueryRow(`SELECT COUNT(*) FROM a`).Scan(&n)
	if n != 4 || got.Status != "error" || !strings.Contains(got.Error, "第 2 条") {
		t.Fatalf("rows=%d event=%+v", n, got)
	}
}
