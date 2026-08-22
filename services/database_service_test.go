package services

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
