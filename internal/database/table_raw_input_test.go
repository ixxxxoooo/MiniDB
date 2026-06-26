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

func TestNormalizeSqlQuotes(t *testing.T) {
	cases := map[string]string{
		"city_name='aaa'":             "city_name='aaa'",
		"city_name=\u2018aaa\u2019":    "city_name='aaa'",  // ‘ ’ 智能单引号
		`name="bob"`:                  `name="bob"`,
		"name=\u201Cbob\u201D":        `name="bob"`,        // “ ” 智能双引号
		"col=\uFF07x\uFF07":           "col='x'",           // ＇全角单引号
		"col=\uFF02x\uFF02":           `col="x"`,           // ＂全角双引号
		"no quotes here":              "no quotes here",
	}
	for in, want := range cases {
		if got := normalizeSqlQuotes(in); got != want {
			t.Errorf("normalizeSqlQuotes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildTableDataQuerySQL_NormalizesSmartQuotes(t *testing.T) {
	// macOS 智能引号：'aaa' -> ‘aaa’，拼接 WHERE 后必须是直引号才能执行
	in := "city_name=\u2018aaa\u2019"
	got, err := BuildTableDataQuerySQL("mysql", "db1", "users", in, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "city_name='aaa'") {
		t.Fatalf("expected smart quotes normalized to ASCII, got %q", got)
	}
}
