// @author ixxxxoooo

package database

import (
	"reflect"
	"testing"
)

// TestSQLGenPerDialect 逐方言校验迁入方言之后的语句写法，与迁移前的输出一致。
func TestSQLGenPerDialect(t *testing.T) {
	check := func(name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s\n got: %s\nwant: %s", name, got, want)
		}
	}
	for typ, want := range map[DBType]string{
		MySQL: "RENAME TABLE `a` TO `b`", MariaDB: "RENAME TABLE `a` TO `b`", TiDB: "RENAME TABLE `a` TO `b`",
		StarRocks: "ALTER TABLE `a` RENAME `b`", ClickHouse: "RENAME TABLE `a` TO `b`",
		Postgres: `ALTER TABLE "a" RENAME TO "b"`, SQLite: `ALTER TABLE "a" RENAME TO "b"`,
		SQLServer: "EXEC sp_rename 'a', 'b'",
	} {
		got, err := BuildRenameTableSQL(typ, "a", "b")
		if err != nil {
			t.Fatal(err)
		}
		check("rename "+string(typ), got, want)
	}
	check("truncate sqlite", BuildTruncateTableSQL(SQLite, "t", TableOpOptions{Cascade: true}), `DELETE FROM "t"`)
	check("truncate pg", BuildTruncateTableSQL(Postgres, "t", TableOpOptions{Cascade: true, RestartIdentity: true}), `TRUNCATE TABLE "t" RESTART IDENTITY CASCADE`)
	check("truncate mysql 忽略不支持的选项", BuildTruncateTableSQL(MySQL, "t", TableOpOptions{Cascade: true}), "TRUNCATE TABLE `t`")
	check("drop mysql 忽略 cascade", BuildDropTableSQL(MySQL, "t", TableOpOptions{Cascade: true}), "DROP TABLE `t`")
	check("drop pg", BuildDropTableSQL(Postgres, "t", TableOpOptions{Cascade: true}), `DROP TABLE "t" CASCADE`)
	check("binary pg", SQLLiteralFor(Postgres, []byte{0xff}), `'\xFF'`)
	check("binary mysql", SQLLiteralFor(MySQL, []byte{0xff}), "X'FF'")
	check("import type ch", importColumnType(ClickHouse, "float"), "Float64")
	check("import type mysql", importColumnType(TiDB, "int"), "BIGINT")
	check("import suffix ch", importTableSuffix(ClickHouse), " ENGINE = MergeTree ORDER BY tuple()")
	check("import suffix pg", importTableSuffix(Postgres), "")

	got, _ := BuildDropForeignKeySQL(Postgres, "t", "f")
	check("drop fk pg", got, `ALTER TABLE "t" DROP CONSTRAINT "f"`)
	got, _ = BuildDropForeignKeySQL(MySQL, "t", "f")
	check("drop fk mysql", got, "ALTER TABLE `t` DROP FOREIGN KEY `f`")
	if _, err := BuildDropForeignKeySQL(SQLite, "t", "f"); err == nil {
		t.Error("SQLite 不能删外键")
	}
	got, _ = BuildAddIndexSQL(SQLite, "t", "ix", []string{"a"}, true)
	check("add index sqlite", got, `CREATE UNIQUE INDEX IF NOT EXISTS "ix" ON "t" ("a")`)
	got, _ = BuildAddIndexSQL(MariaDB, "t", "ix", []string{"a"}, false)
	check("add index mariadb", got, "ALTER TABLE `t` ADD INDEX `ix` (`a`)")
	if _, err := BuildAddIndexSQL(ClickHouse, "t", "ix", []string{"a"}, false); err == nil {
		t.Error("ClickHouse 不支持在界面里加索引")
	}
	got, _ = BuildCreateTriggerSQL(SQLite, "t", TriggerInfo{Name: "x", Timing: "AFTER", Event: "INSERT", Definition: "SELECT 1;"})
	check("trigger sqlite", got, `CREATE TRIGGER "x" AFTER INSERT ON "t" FOR EACH ROW BEGIN SELECT 1; END`)
}

// TestSQLGenUnsupportedByCaps 没有对应能力的方言得到明确的错误，而不是一条在该库里不能执行的语句。
func TestSQLGenUnsupportedByCaps(t *testing.T) {
	tr := TriggerInfo{Name: "x", Timing: "AFTER", Event: "INSERT", Definition: "b"}
	fk := ForeignKeyInfo{Name: "f", Columns: []string{"a"}, RefTable: "r", RefColumns: []string{"id"}}
	for _, typ := range []DBType{StarRocks, ClickHouse, SQLServer} {
		if _, err := BuildCreateTriggerSQL(typ, "t", tr); err == nil {
			t.Errorf("%s 没有触发器能力，应报错", typ)
		}
	}
	for _, typ := range []DBType{StarRocks, ClickHouse, SQLServer, SQLite} {
		if _, err := BuildAddForeignKeySQL(typ, "t", fk); err == nil {
			t.Errorf("%s 不能给已有的表加外键，应报错", typ)
		}
	}
	// 不支持字符集的方言建库只写库名（SQL Server 过去也被拼上了 CHARACTER SET，在它那里是错的）
	if got, _ := BuildCreateDatabaseSQL(SQLServer, "d", "latin1"); got != "CREATE DATABASE [d]" {
		t.Errorf("SQL Server 建库 = %s", got)
	}
}

func TestCloneTableStatementsUsesDDLCloner(t *testing.T) {
	got, err := CloneTableStatements(nil, nil, MySQL, "d", "a", "b", false)
	if err != nil || !reflect.DeepEqual(got, []string{"CREATE TABLE `b` LIKE `a`"}) {
		t.Errorf("MySQL = %v, %v", got, err)
	}
	if _, ok := hookOf[DDLCloner](SQLite); !ok {
		t.Error("SQLite 应实现 DDLCloner")
	}
	if _, ok := hookOf[DDLCloner](MySQL); ok {
		t.Error("MySQL 不应实现 DDLCloner")
	}
}
