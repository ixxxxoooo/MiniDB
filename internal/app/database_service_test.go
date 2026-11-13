package app

import (
	"testing"

	"minidb/internal/database"
)

// @author ygw
// TestEnsureDatabaseListed 验证切换器会保留当前连接配置中的数据库。
func TestEnsureDatabaseListed(t *testing.T) {
	t.Parallel()

	dbs := []database.DatabaseInfo{{Name: "app_db"}}
	got := ensureDatabaseListed(dbs, "mysql")
	if len(got) != 2 || got[0].Name != "mysql" || got[1].Name != "app_db" {
		t.Fatalf("unexpected result: %+v", got)
	}

	unchanged := ensureDatabaseListed(dbs, "app_db")
	if len(unchanged) != 1 || unchanged[0].Name != "app_db" {
		t.Fatalf("expected existing database to remain unchanged: %+v", unchanged)
	}
}

func TestApplyUserStatementsStopsAtFirstError(t *testing.T) {
	s := newSQLiteDatabaseService(t)
	db, _ := s.manager.GetDB("c1")
	n, err := s.ApplyUserStatements("c1", []string{"CREATE TABLE ok1 (x)", "CREATE TABLE ok1 (x)", "CREATE TABLE never (x)"})
	if err == nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var c int
	db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='never'`).Scan(&c)
	if c != 0 {
		t.Fatal("出错后不应继续执行")
	}
	if _, err := s.GetUsers("c1"); err == nil {
		t.Fatal("SQLite 不支持用户管理")
	}
}
