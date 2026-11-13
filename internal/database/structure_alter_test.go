package database

import (
	"strings"
	"testing"
)

func TestBuildStructureAlterDDLStatements_MySQLSingleStatement(t *testing.T) {
	working := []StructureColumnEdit{
		{UID: "n1", Status: "new", Name: "age", Type: "int", Nullable: true},
	}
	stmts, err := BuildStructureAlterDDLStatements("mysql", "8.0.33", "users", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 || !strings.Contains(stmts[0], "ALTER TABLE") || !strings.Contains(stmts[0], "ADD COLUMN") {
		t.Fatalf("unexpected: %v", stmts)
	}
}

// TestBuildStructureAlterDDLStatements_MySQLCommentEscapesApostrophe COMMENT 中单引号须按 SQL 标准加倍转义
func TestBuildStructureAlterDDLStatements_MySQLCommentEscapesApostrophe(t *testing.T) {
	working := []StructureColumnEdit{
		{UID: "n1", Status: "new", Name: "age", Type: "int", Nullable: true, Comment: "用户'备注"},
	}
	stmts, err := BuildStructureAlterDDLStatements("mysql", "8.0.33", "users", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("want 1 stmt, got %v", stmts)
	}
	if !strings.Contains(stmts[0], "用户''备注") {
		t.Fatalf("COMMENT should escape single quote as doubled: %q", stmts[0])
	}
}

func TestBuildStructureAlterDDLStatements_PostgresMultiple(t *testing.T) {
	working := []StructureColumnEdit{
		{UID: "n1", Status: "new", Name: "note", Type: "text", Nullable: true, Comment: "备注"},
	}
	stmts, err := BuildStructureAlterDDLStatements("postgres", "14.5", "users", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) < 2 {
		t.Fatalf("postgres 应有 COMMENT ON 独立语句，got %v", stmts)
	}
	if !strings.Contains(stmts[0], "ALTER TABLE") {
		t.Fatal(stmts[0])
	}
	foundComment := false
	for _, s := range stmts {
		if strings.Contains(s, "COMMENT ON COLUMN") {
			foundComment = true
			break
		}
	}
	if !foundComment {
		t.Fatalf("缺少 COMMENT ON COLUMN: %v", stmts)
	}
}

func TestBuildStructureAlterDDLStatements_SQLiteAddColumn(t *testing.T) {
	working := []StructureColumnEdit{
		{UID: "n1", Status: "new", Name: "x", Type: "INTEGER", Nullable: true},
	}
	stmts, err := BuildStructureAlterDDLStatements("sqlite", "3.40.0", "t1", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 || !strings.Contains(stmts[0], "ADD COLUMN") {
		t.Fatalf("got %v", stmts)
	}
}

func TestBuildStructureAlterDDLStatements_SQLiteDropColumnOldVersion(t *testing.T) {
	orig := []StructureColumnEdit{{UID: "o1", Name: "id", Type: "INTEGER", Nullable: false}}
	working := []StructureColumnEdit{{UID: "o1", Status: "deleted", Name: "id", Type: "INTEGER", Nullable: false}}
	_, err := BuildStructureAlterDDLStatements("sqlite", "3.34.0", "t1", working, orig, nil)
	if err == nil || !strings.Contains(err.Error(), "DROP COLUMN") {
		t.Fatalf("expect drop column unsupported error, got %v", err)
	}
}

func TestBuildStructureAlterDDLStatements_SQLiteRenameNeedsVersion(t *testing.T) {
	// 版本未知时校验放行，但 RENAME 需能解析出版本号才允许生成
	orig := []StructureColumnEdit{{UID: "o1", Name: "a", Type: "TEXT", Nullable: true}}
	working := []StructureColumnEdit{{UID: "o1", Name: "b", Type: "TEXT", Nullable: true}}
	_, err := BuildStructureAlterDDLStatements("sqlite", "", "t1", working, orig, nil)
	if err == nil || !strings.Contains(err.Error(), "RENAME COLUMN") {
		t.Fatalf("expect rename version error, got %v", err)
	}
}

func TestBuildAddIndexSQL_Dialects(t *testing.T) {
	mysqlSQL, err := BuildAddIndexSQL("mysql", "users", "idx_email", []string{"email"}, false)
	if err != nil || !strings.Contains(mysqlSQL, "ALTER TABLE") {
		t.Fatalf("mysql: %v %q", err, mysqlSQL)
	}
	pgSQL, err := BuildAddIndexSQL("postgres", "users", "idx_email", []string{"email"}, true)
	if err != nil || !strings.Contains(pgSQL, "CREATE UNIQUE INDEX") {
		t.Fatalf("postgres: %v %q", err, pgSQL)
	}
	_, err = BuildAddIndexSQL("oracle", "t", "i", []string{"a"}, false)
	if err == nil {
		t.Fatal("oracle 应不支持")
	}
}

func TestBuildDropIndexSQL_Dialects(t *testing.T) {
	m, err := BuildDropIndexSQL("mysql", "users", "idx1")
	if err != nil || !strings.Contains(m, "DROP INDEX") {
		t.Fatal(m, err)
	}
	p, err := BuildDropIndexSQL("postgres", "users", "idx1")
	if err != nil || !strings.Contains(p, "DROP INDEX IF EXISTS") {
		t.Fatal(p, err)
	}
}

func mysqlCols(names ...string) []StructureColumnEdit {
	var out []StructureColumnEdit
	for _, n := range names {
		out = append(out, StructureColumnEdit{UID: n, Name: n, Type: "int", Nullable: true})
	}
	return out
}

func mysqlAlter(t *testing.T, working, orig []StructureColumnEdit) string {
	t.Helper()
	stmts, err := BuildStructureAlterDDLStatements("mysql", "8.0.33", "t", working, orig, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) == 0 {
		return ""
	}
	return stmts[0]
}

// 列顺序没变时，不产生任何语句。
func TestMySQLColumnOrderUnchanged(t *testing.T) {
	orig := mysqlCols("a", "b", "c")
	if got := mysqlAlter(t, mysqlCols("a", "b", "c"), orig); got != "" {
		t.Fatalf("顺序没变不该有 SQL: %s", got)
	}
}

// 把最后一列拖到最前：只挪这一列，用 FIRST。
func TestMySQLColumnMoveToFirst(t *testing.T) {
	orig := mysqlCols("a", "b", "c")
	got := mysqlAlter(t, mysqlCols("c", "a", "b"), orig)
	want := "ALTER TABLE `t` CHANGE COLUMN `c` `c` int NULL FIRST"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// 把第一列拖到中间：同样只挪一列，用 AFTER 前一列。
func TestMySQLColumnMoveAfter(t *testing.T) {
	orig := mysqlCols("a", "b", "c")
	got := mysqlAlter(t, mysqlCols("b", "a", "c"), orig)
	// 最长不动子序列是 (a,c) 或 (b,c)，两种挪法都只动一列
	if got != "ALTER TABLE `t` CHANGE COLUMN `b` `b` int NULL FIRST" &&
		got != "ALTER TABLE `t` CHANGE COLUMN `a` `a` int NULL AFTER `b`" {
		t.Fatalf("unexpected: %s", got)
	}
}

// 挪列要把自增和 ON UPDATE 原样带回去，不能丢。
func TestMySQLColumnMoveKeepsAttributes(t *testing.T) {
	id := StructureColumnEdit{UID: "id", Name: "id", Type: "bigint", AutoIncrement: true}
	ts := StructureColumnEdit{UID: "ts", Name: "ts", Type: "timestamp", Nullable: true, OnUpdate: "CURRENT_TIMESTAMP"}
	// 两列互换位置时挪的是后一列，所以两种顺序各验一个
	if got := mysqlAlter(t, []StructureColumnEdit{ts, id}, []StructureColumnEdit{id, ts}); !strings.Contains(got, "`id` `id` bigint NOT NULL AUTO_INCREMENT") {
		t.Fatalf("丢了自增: %s", got)
	}
	if got := mysqlAlter(t, []StructureColumnEdit{id, ts}, []StructureColumnEdit{ts, id}); !strings.Contains(got, "ON UPDATE CURRENT_TIMESTAMP") {
		t.Fatalf("丢了 ON UPDATE: %s", got)
	}
}

// 新增的列插在已有列中间时带 AFTER，追加在末尾时不带。
func TestMySQLNewColumnPlacement(t *testing.T) {
	orig := mysqlCols("a", "b")
	mid := []StructureColumnEdit{orig[0], {UID: "new:1", Status: "new", Name: "x", Type: "int", Nullable: true}, orig[1]}
	if got, want := mysqlAlter(t, mid, orig), "ALTER TABLE `t` ADD COLUMN `x` int NULL AFTER `a`"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	first := []StructureColumnEdit{{UID: "new:1", Status: "new", Name: "x", Type: "int", Nullable: true}, orig[0], orig[1]}
	if got, want := mysqlAlter(t, first, orig), "ALTER TABLE `t` ADD COLUMN `x` int NULL FIRST"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	tail := []StructureColumnEdit{orig[0], orig[1], {UID: "new:1", Status: "new", Name: "x", Type: "int", Nullable: true}}
	if got, want := mysqlAlter(t, tail, orig), "ALTER TABLE `t` ADD COLUMN `x` int NULL"; got != want {
		t.Fatalf("追加在末尾不该带位置:\ngot  %s\nwant %s", got, want)
	}
}

// 删列之后剩下的列顺序没变，不应误判成挪动。
func TestMySQLDeleteDoesNotMove(t *testing.T) {
	orig := mysqlCols("a", "b", "c")
	working := mysqlCols("a", "b", "c")
	working[1].Status = "deleted"
	if got, want := mysqlAlter(t, working, orig), "ALTER TABLE `t` DROP COLUMN `b`"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestParseOnUpdate(t *testing.T) {
	for in, want := range map[string]string{
		"DEFAULT_GENERATED on update CURRENT_TIMESTAMP(3)": "CURRENT_TIMESTAMP(3)",
		"on update current_timestamp":                      "CURRENT_TIMESTAMP",
		"auto_increment":                                   "",
	} {
		if got := ParseOnUpdate(in); got != want {
			t.Errorf("ParseOnUpdate(%q) = %q, want %q", in, got, want)
		}
	}
}
