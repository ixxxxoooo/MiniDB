package ui

import (
	"reflect"
	"testing"
)

// 样例取自 MySQL 8.0.36 的 SHOW GRANTS FOR 与 PostgreSQL 16 按表合成的 GRANT（见 dialect_postgres_admin.go）。
func TestParseGrantsMySQL(t *testing.T) {
	lines := []string{
		"GRANT USAGE ON *.* TO `app_rw`@`10.0.%`",
		"GRANT SELECT, INSERT, UPDATE, DELETE, INDEX ON `shop`.* TO `app_rw`@`10.0.%`",
		"GRANT SELECT, UPDATE ON `shop`.`orders` TO `app_rw`@`10.0.%` WITH GRANT OPTION",
		"GRANT SELECT (id, name) ON `shop`.`users` TO `app_rw`@`10.0.%`",
		"GRANT PROCESS, ALL PRIVILEGES ON `mysql`.* TO `app_rw`@`10.0.%`",
		"GRANT `reader` TO `app_rw`@`10.0.%`",
	}
	got := parseGrants(lines, mysqlGrantCols)
	if len(got) != 4 { // USAGE 和角色授予都不成行
		t.Fatalf("行数 = %d: %+v", len(got), got)
	}
	if got[0].object != "shop.*" || !got[0].privs["INDEX"] || got[0].privs["CREATE"] || got[0].option {
		t.Fatalf("库级: %+v", got[0])
	}
	if got[1].object != "shop.orders" || !got[1].option || len(got[1].privs) != 2 {
		t.Fatalf("表级 + GRANT OPTION: %+v", got[1])
	}
	if got[2].object != "shop.users" || !got[2].privs["SELECT"] {
		t.Fatalf("列级权限按这一类权限算: %+v", got[2])
	}
	if got[3].object != "mysql.*" || len(got[3].privs) != len(mysqlGrantCols) || !reflect.DeepEqual(got[3].extra, []string{"PROCESS"}) {
		t.Fatalf("ALL PRIVILEGES 展开成每一列，矩阵外的权限进 extra: %+v", got[3])
	}
}

func TestParseGrantsPostgresAndGlobal(t *testing.T) {
	got := parseGrants([]string{
		`GRANT SELECT, INSERT ON TABLE "public"."orders" TO "r"`,
		`GRANT UPDATE ON TABLE "public"."orders" TO "r"`,
		"GRANT ALL PRIVILEGES ON *.* TO `root`@`localhost` WITH GRANT OPTION",
	}, pgGrantCols)
	if len(got) != 2 || got[0].object != "public.orders" || len(got[0].privs) != 3 {
		t.Fatalf("同一对象的多条合并: %+v", got)
	}
	if got[1].object != "*.*" || !got[1].option || len(got[1].privs) != len(pgGrantCols) {
		t.Fatalf("全局: %+v", got[1])
	}
	if parseGrants([]string{"错误：没有权限查看"}, pgGrantCols) != nil && len(parseGrants([]string{"错误：没有权限查看"}, pgGrantCols)) != 0 {
		t.Fatal("读不出来的文字不成行")
	}
}
