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

func TestBuildAlterTableFromStructureDiff_PostgresMultiError(t *testing.T) {
	working := []StructureColumnEdit{
		{UID: "n1", Status: "new", Name: "c", Type: "int", Nullable: true, Comment: "x"},
	}
	_, err := BuildAlterTableFromStructureDiff("postgres", "t", working, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "多条 DDL") {
		t.Fatalf("want multi-ddl error, got %v", err)
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
