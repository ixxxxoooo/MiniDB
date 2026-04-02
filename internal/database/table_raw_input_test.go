package database

import (
	"strings"
	"testing"
)

func TestIsLikelyFullStatementInput(t *testing.T) {
	if !IsLikelyFullStatementInput("SELECT * FROM a") {
		t.Fatal("expected SELECT to be full statement")
	}
	if !IsLikelyFullStatementInput("  explain select 1") {
		t.Fatal("expected EXPLAIN prefix")
	}
	if IsLikelyFullStatementInput("id = 1") {
		t.Fatal("condition fragment should not be full statement")
	}
}

func TestBuildTableDataQuerySQL_WrapWhere(t *testing.T) {
	q := BuildTableDataQuerySQL("mysql", "db1", "users", "status = 1", 2, 10, "")
	if q == "" || !strings.Contains(q, "WHERE status = 1") || !strings.Contains(q, "LIMIT 10 OFFSET 10") {
		t.Fatalf("unexpected SQL: %q", q)
	}
}

func TestBuildTableDataQuerySQL_FullPassthrough(t *testing.T) {
	in := "SELECT id FROM users"
	if got := BuildTableDataQuerySQL("mysql", "db1", "users", in, 1, 10, ""); got != in {
		t.Fatalf("got %q want %q", got, in)
	}
}

// TestBuildTableDataQuerySQL_SQLitePragmaDialect 验证按方言识别整句：SQLite 下 PRAGMA 应直通，不得包成 WHERE
func TestBuildTableDataQuerySQL_SQLitePragmaDialect(t *testing.T) {
	in := "PRAGMA table_info(users)"
	got := BuildTableDataQuerySQL("sqlite", "main", "users", in, 0, 10, "3.40.0")
	if got != in {
		t.Fatalf("sqlite PRAGMA 应原样返回，got %q want %q", got, in)
	}
}

// TestBuildTableDataQuerySQL_MySQLUseDialect MySQL 系 USE 应识别为整句
func TestBuildTableDataQuerySQL_MySQLUseDialect(t *testing.T) {
	in := "USE otherdb"
	got := BuildTableDataQuerySQL("mysql", "db1", "users", in, 0, 10, "8.0.33")
	if got != in {
		t.Fatalf("USE 应原样返回，got %q want %q", got, in)
	}
}
