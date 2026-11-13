package database

import "testing"

func TestParseSQLiteNumericVersion(t *testing.T) {
	if got := ParseSQLiteNumericVersion("SQLite 3.40.0"); got != 3_040_000 {
		t.Fatalf("got %d want 3040000", got)
	}
	if ParseSQLiteNumericVersion("3.25.1") != 3_025_001 {
		t.Fatal("bare semver")
	}
}

func TestSQLiteSupportsCapabilities(t *testing.T) {
	if !SQLiteSupportsRenameColumn("3.25.0") {
		t.Fatal("3.25 should support rename")
	}
	if SQLiteSupportsRenameColumn("3.24.0") {
		t.Fatal("3.24 should not")
	}
	if !SQLiteSupportsDropColumn("3.35.0") {
		t.Fatal("3.35 should support drop column")
	}
	if SQLiteSupportsDropColumn("3.34.0") {
		t.Fatal("3.34 should not")
	}
}

func TestIsLikelyFullStatementForDialect(t *testing.T) {
	if !IsLikelyFullStatementForDialect("sqlite", "pragma table_info(x)") {
		t.Fatal("sqlite PRAGMA")
	}
	if !IsLikelyFullStatementForDialect("mysql", "use foo") {
		t.Fatal("mysql USE")
	}
	if !IsLikelyFullStatementForDialect("postgres", "vacuum analyze") {
		t.Fatal("postgres VACUUM")
	}
	// 未知类型：不包含方言特有前缀
	if IsLikelyFullStatementForDialect("", "use foo") {
		t.Fatal("empty dbType 不应把 USE 当整句")
	}
	if !IsLikelyFullStatementForDialect("", "select 1") {
		t.Fatal("通用 SELECT 仍应识别")
	}
}

func TestValidateStructureAlterSupported(t *testing.T) {
	if err := ValidateStructureAlterSupported("mysql", ""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStructureAlterSupported("sqlite", ""); err != nil {
		t.Fatal("未知 SQLite 版本应放行校验，由生成阶段再判断能力")
	}
	if err := ValidateStructureAlterSupported("sqlite", "3.20.0"); err == nil {
		t.Fatal("过低版本应拒绝")
	}
	if err := ValidateStructureAlterSupported("oracle", ""); err == nil {
		t.Fatal("不支持类型应报错")
	}
}
