package database

import "testing"

func TestMariaDBDialect(t *testing.T) {
	d, err := DialectOf(MariaDB)
	if err != nil || d.Label() != "MariaDB" || d.Type() != MariaDB {
		t.Fatalf("%v %v", d, err)
	}
	if !MariaDB.MySQLCompatible() || !IsMySQLCompatible(MariaDB) {
		t.Fatal("MariaDB 应走 MySQL 协议")
	}
	c := CapsOf(MariaDB)
	if !c.SessionUseDB || !c.AlterTable || !c.Explain || c.ExplainAnalyze || c.UserResourceLimits {
		t.Fatalf("能力不对: %+v", c)
	}
	if d.DefaultPort() != 3306 || d.DefaultUser() != "root" || d.Driver() != "mysql" {
		t.Fatal("默认值应与 MySQL 一致")
	}
	if !MariaDB.Valid() {
		t.Fatal("应是有效类型")
	}
	if got := QuoteIdent(MariaDB, "a`b"); got != "`a``b`" {
		t.Fatal(got)
	}
	if got := Placeholder(MariaDB, 2); got != "?" {
		t.Fatal(got)
	}
}

func TestTraditionalPlanRoots(t *testing.T) {
	cols := []string{"id", "select_type", "table", "type", "possible_keys", "key", "key_len", "ref", "rows", "Extra"}
	rows := [][]string{
		{"1", "SIMPLE", "it_orders", "ref", "idx_a", "idx_a", "9", "const", "12", "Using where"},
		{"1", "SIMPLE", "it_users", "ALL", "", "", "", "", "1,500", ""},
	}
	roots := traditionalPlanRoots(cols, rows)
	if len(roots) != 2 || roots[0].Op != "ref" || roots[0].Object != "it_orders" || roots[0].Rows != 12 || roots[0].Extra != "key=idx_a Using where" {
		t.Fatalf("%+v", roots[0])
	}
	if roots[1].Op != "ALL" || roots[1].Rows != 1500 || roots[1].Extra != "" {
		t.Fatalf("%+v", roots[1])
	}
	if got := traditionalPlanRoots([]string{"x"}, [][]string{{"1"}}); len(got) != 1 || got[0].Op != "" {
		t.Fatalf("缺列时应留空: %+v", got)
	}
}
