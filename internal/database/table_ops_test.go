package database

import (
	"strings"
	"testing"
)

func TestBuildRenameTableSQL(t *testing.T) {
	for db, want := range map[DBType]string{
		"mysql": "RENAME TABLE `a` TO `b`", "postgres": `ALTER TABLE "a" RENAME TO "b"`, "sqlite": `ALTER TABLE "a" RENAME TO "b"`,
	} {
		got, err := BuildRenameTableSQL(db, "a", "b")
		if err != nil || got != want {
			t.Fatalf("%s: %q %v", db, got, err)
		}
	}
	if _, err := BuildRenameTableSQL("mysql", "a", " a "); err == nil {
		t.Fatal("同名应报错")
	}
	if _, err := BuildRenameTableSQL("mysql", "a", ""); err == nil {
		t.Fatal("空名应报错")
	}
}

func TestBuildCreateTableStatements(t *testing.T) {
	cols := []StructureColumnEdit{
		{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true},
		{Name: "name", Type: "varchar(40)", Nullable: true, Comment: "昵称"},
	}
	idx := []StructureIndexEdit{{Name: "idx_name", Columns: []string{"name"}}, {Name: "", Columns: nil}}
	my, err := BuildCreateTableStatements("mysql", "t", cols, idx)
	if err != nil || len(my) != 2 {
		t.Fatalf("mysql: %v %v", my, err)
	}
	if !strings.Contains(my[0], "`id` bigint NOT NULL AUTO_INCREMENT") || !strings.Contains(my[0], "PRIMARY KEY (`id`)") || !strings.Contains(my[0], "COMMENT '昵称'") || !strings.Contains(my[1], "ADD INDEX `idx_name`") {
		t.Fatalf("mysql: %v", my)
	}
	pg, err := BuildCreateTableStatements("postgres", "t", cols, idx)
	if err != nil || len(pg) != 3 || !strings.Contains(pg[0], `"id" BIGSERIAL NOT NULL`) || !strings.HasPrefix(pg[1], "COMMENT ON COLUMN") || !strings.Contains(pg[2], "CREATE INDEX") {
		t.Fatalf("postgres: %v %v", pg, err)
	}
	sq, err := BuildCreateTableStatements("sqlite", "t", cols, nil)
	if err != nil || !strings.Contains(sq[0], `"id" INTEGER PRIMARY KEY AUTOINCREMENT`) || strings.Contains(sq[0], "PRIMARY KEY (") {
		t.Fatalf("sqlite: %v %v", sq, err)
	}
	// 联合主键
	multi := []StructureColumnEdit{{Name: "a", Type: "int", PrimaryKey: true}, {Name: "b", Type: "int", PrimaryKey: true}}
	if s, err := BuildCreateTableStatements("sqlite", "t", multi, nil); err != nil || !strings.Contains(s[0], `PRIMARY KEY ("a", "b")`) {
		t.Fatalf("联合主键: %v %v", s, err)
	}
	// 错误
	for name, c := range map[string][]StructureColumnEdit{
		"无列": nil, "空名": {{Name: "", Type: "int"}}, "缺类型": {{Name: "a"}},
		"重名":          {{Name: "a", Type: "int"}, {Name: "A", Type: "int"}},
		"sqlite自增非主键": {{Name: "a", Type: "int", AutoIncrement: true}},
	} {
		if _, err := BuildCreateTableStatements("sqlite", "t", c, nil); err == nil {
			t.Fatalf("%s 应报错", name)
		}
	}
	if _, err := BuildCreateTableStatements("sqlite", " ", cols, nil); err == nil {
		t.Fatal("空表名应报错")
	}
}

func TestBuildTruncateAndDropOptions(t *testing.T) {
	o := TableOpOptions{Cascade: true, RestartIdentity: true}
	if got := BuildTruncateTableSQL("postgres", "t", o); got != `TRUNCATE TABLE "t" RESTART IDENTITY CASCADE` {
		t.Fatal(got)
	}
	if got := BuildTruncateTableSQL("mysql", "t", o); got != "TRUNCATE TABLE `t`" {
		t.Fatalf("MySQL 不支持这两个选项: %s", got)
	}
	if got := BuildTruncateTableSQL("sqlite", "t", o); got != `DELETE FROM "t"` {
		t.Fatal(got)
	}
	if got := BuildDropTableSQL("postgres", "t", o); got != `DROP TABLE "t" CASCADE` {
		t.Fatal(got)
	}
	if got := BuildDropTableSQL("mysql", "t", o); got != "DROP TABLE `t`" {
		t.Fatal(got)
	}
}
