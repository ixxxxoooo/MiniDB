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
	q, err := BuildTableDataQuerySQL("mysql", "db1", "users", "status = 1", 2, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if q == "" || !strings.Contains(q, "WHERE status = 1") || !strings.Contains(q, "LIMIT 10 OFFSET 10") {
		t.Fatalf("unexpected SQL: %q", q)
	}
}

func TestBuildTableDataQuerySQL_FullPassthrough(t *testing.T) {
	in := "SELECT id FROM users"
	got, err := BuildTableDataQuerySQL("mysql", "db1", "users", in, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestBuildTableDataQuerySQL_RejectsRiskyFullStatement(t *testing.T) {
	if _, err := BuildTableDataQuerySQL("mysql", "db1", "users", "DROP TABLE users", 1, 10, ""); err == nil {
		t.Fatal("expected risky SQL to be rejected")
	}
}

func TestBuildTableDataQuerySQL_RejectsMultipleStatements(t *testing.T) {
	if _, err := BuildTableDataQuerySQL("mysql", "db1", "users", "SELECT * FROM users; DROP TABLE users", 1, 10, ""); err == nil {
		t.Fatal("expected multiple statements to be rejected")
	}
}
