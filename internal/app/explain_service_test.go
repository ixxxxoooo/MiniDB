package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"minidb/internal/database"
	"minidb/internal/model"
)

func TestCheckExplainable(t *testing.T) {
	cases := []struct {
		sql     string
		analyze bool
		ok      bool
	}{
		{"SELECT * FROM t", false, true},
		{"SELECT * FROM t", true, true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true, true},
		{"UPDATE t SET a = 1", false, true}, // 普通 EXPLAIN 不执行，可以看增删改的计划
		{"DELETE FROM t WHERE a = 1", false, true},
		{"UPDATE t SET a = 1", true, false}, // ANALYZE 会真实执行，不放行写入
		{"DELETE FROM t", true, false},
		{"SELECT * FROM t FOR UPDATE", true, false},
		{"SELECT SLEEP(10)", true, false},
		{"SELECT 1; SELECT 2", false, false},
		{"", false, false},
		{"DROP TABLE t", false, false},
		{"SHOW TABLES", false, false},
		{"CREATE TABLE x (a int)", false, false},
	}
	for _, c := range cases {
		err := checkExplainable(c.sql, c.analyze)
		if (err == nil) != c.ok {
			t.Errorf("checkExplainable(%q, analyze=%v) = %v, 期望可解释=%v", c.sql, c.analyze, err, c.ok)
		}
		if err != nil {
			if code := model.CodeOf(err); code != model.CodeExplainRejected {
				t.Errorf("%q 的错误码 = %q", c.sql, code)
			}
		}
	}
}

func newExplainTestEnv(t *testing.T) *ExplainService {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "e.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	db, _ := manager.GetDB("c1")
	for _, q := range []string{
		`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`,
		`CREATE INDEX idx_people_age ON people(age)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return NewExplainService(manager)
}

func TestExplainServiceSQLite(t *testing.T) {
	s := newExplainTestEnv(t)
	ctx := context.Background()
	res, err := s.Explain(ctx, "c1", "", "SELECT * FROM people WHERE age = 3;", false)
	if err != nil {
		t.Fatal(err)
	}
	flat := res.Flat()
	if len(flat) == 0 || flat[0].Node.Object != "people" || !strings.Contains(flat[0].Node.Extra, "idx_people_age") {
		t.Fatalf("应解析出用到 idx_people_age 的 SEARCH: %+v", res.Roots)
	}
	// 带 EXPLAIN 前缀、UPDATE 也能看计划
	if _, err := s.Explain(ctx, "c1", "", "EXPLAIN UPDATE people SET name = 'x' WHERE id = 1", false); err != nil {
		t.Fatalf("UPDATE 的计划应可看: %v", err)
	}
	// 这条 UPDATE 只是解释，不能真的改数据
	db, _ := s.manager.GetDB("c1")
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM people WHERE name = 'x'`).Scan(&n)
	if n != 0 {
		t.Fatal("EXPLAIN 不应执行语句")
	}

	// SQLite 不支持 ANALYZE
	if _, err := s.Explain(ctx, "c1", "", "SELECT 1", true); err == nil {
		t.Fatal("SQLite 的 ANALYZE 应被拒绝")
	}
	// 多语句、未知连接
	if _, err := s.Explain(ctx, "c1", "", "SELECT 1; DROP TABLE people", false); err == nil {
		t.Fatal("多语句应被拒绝")
	}
	if _, err := s.Explain(ctx, "nope", "", "SELECT 1", false); err == nil {
		t.Fatal("连接不存在应报错")
	}
}

func TestExplainCapabilityMatrix(t *testing.T) {
	cases := []struct {
		typ              database.DBType
		explain, analyze bool
	}{
		{database.MySQL, true, true},
		{database.TiDB, true, true},
		{database.StarRocks, true, false},
		{database.Postgres, true, true},
		{database.SQLite, true, false},
	}
	for _, c := range cases {
		if CanExplain(c.typ) != c.explain || CanExplainAnalyze(c.typ) != c.analyze {
			t.Errorf("%s: explain=%v analyze=%v", c.typ, CanExplain(c.typ), CanExplainAnalyze(c.typ))
		}
	}
}
