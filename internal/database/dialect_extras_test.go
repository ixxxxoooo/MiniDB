// @author ixxxxoooo

package database

import "testing"

func TestURLSchemes(t *testing.T) {
	for _, typ := range AllDBTypes() {
		got, ok := TypeFromURLScheme(URLSchemeOf(typ))
		if !ok || got != typ {
			t.Errorf("%s 的规范协议名 %q 解析回 %q", typ, URLSchemeOf(typ), got)
		}
	}
	cases := map[string]DBType{"postgres": Postgres, "PG": Postgres, "mssql": SQLServer, "ch": ClickHouse, "sqlite3": SQLite, "file": SQLite, "mariadb": MariaDB}
	for scheme, want := range cases {
		if got, ok := TypeFromURLScheme(scheme); !ok || got != want {
			t.Errorf("TypeFromURLScheme(%q) = %q, %v", scheme, got, ok)
		}
	}
	if _, ok := TypeFromURLScheme("oracle"); ok {
		t.Error("未知协议名不应识别")
	}
	if URLSchemeOf(Postgres) != "postgresql" || URLSchemeOf(MySQL) != "mysql" {
		t.Error("规范协议名不对")
	}
}

func TestRefineType(t *testing.T) {
	if got, ok := RefineType(MySQL, "10.11.6-MariaDB-log"); !ok || got != MariaDB {
		t.Errorf("MySQL 连到 MariaDB 应建议改成 MariaDB，得到 %q, %v", got, ok)
	}
	for _, c := range []struct {
		typ DBType
		ver string
	}{{MySQL, "8.0.36"}, {MariaDB, "10.11.6-MariaDB"}, {TiDB, "5.7.25-TiDB-v7"}, {Postgres, "16.1"}} {
		if got, ok := RefineType(c.typ, c.ver); ok {
			t.Errorf("RefineType(%s, %q) 不应给出建议，得到 %q", c.typ, c.ver, got)
		}
	}
}

func TestAutoPKTypeAndDefaults(t *testing.T) {
	if AutoPKType(SQLite) != "INTEGER" || AutoPKType(MySQL) != "bigint" {
		t.Error("自增主键类型不对")
	}
	if !CapsOf(Postgres).RoleUsers || CapsOf(Postgres).DefaultSchema != "public" || !CapsOf(Postgres).QueriesAreTransactions {
		t.Error("PostgreSQL 的账号、schema、监控标签能力没声明")
	}
	if CapsOf(MySQL).RoleUsers || CapsOf(MySQL).DefaultSchema != "" {
		t.Error("MySQL 不应声明这些能力")
	}
	if DefaultDBType() != MySQL {
		t.Error("默认类型应为 MySQL")
	}
}
