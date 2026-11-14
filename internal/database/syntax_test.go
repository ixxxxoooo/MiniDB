package database

import "testing"

func TestStdSyntax(t *testing.T) {
	s := stdSyntax{}
	if s.Placeholder(3) != "?" || (stdSyntax{numbered: true}).Placeholder(3) != "$3" {
		t.Fatal("占位符")
	}
	if got := s.AppendPage("SELECT 1", 10, 20); got != "SELECT 1 LIMIT 10 OFFSET 20" {
		t.Fatal(got)
	}
	if got := s.AppendPage("SELECT 1", 10, -1); got != "SELECT 1 LIMIT 10" {
		t.Fatal(got)
	}
	if got := (stdSyntax{textType: "TEXT"}).CastText(`"a"`); got != `CAST("a" AS TEXT)` {
		t.Fatal(got)
	}
	if got := s.ZeroRows("`t`"); got != "SELECT * FROM `t` LIMIT 0" {
		t.Fatal(got)
	}
}

func TestSyntaxOfBuiltins(t *testing.T) {
	if Placeholder(Postgres, 2) != "$2" || Placeholder(MySQL, 2) != "?" || Placeholder(SQLite, 2) != "?" {
		t.Fatal("各方言的占位符")
	}
	if got := SyntaxOf(Postgres).CastText("x"); got != "CAST(x AS TEXT)" {
		t.Fatal(got)
	}
	if got := SyntaxOf(MySQL).CastText("x"); got != "CAST(x AS CHAR)" {
		t.Fatal(got)
	}
	if got := SyntaxOf("nope").AppendPage("S", 1, 0); got != "S LIMIT 1 OFFSET 0" {
		t.Fatal(got)
	}
}

func TestTopLevelIndex(t *testing.T) {
	cases := []struct {
		sql, kw string
		last    bool
		want    int
	}{
		{"SELECT a FROM t ORDER BY a", "ORDER BY", false, 16},
		{"SELECT a FROM t order   by a", "ORDER BY", false, 16},
		{"SELECT a FROM (SELECT b FROM u ORDER BY b) x", "ORDER BY", false, -1},
		{"SELECT 'order by' FROM t", "ORDER BY", false, -1},
		{"SELECT [order by] FROM t", "ORDER BY", false, -1},
		{"SELECT a /* order by */ FROM t", "ORDER BY", false, -1},
		{"SELECT a FROM t -- order by\n", "ORDER BY", false, -1},
		{"SELECT a, border FROM t", "ORDER", false, -1},
		{"SELECT a FROM t ORDER BY a, (SELECT 1) ORDER BY b", "ORDER BY", true, 39},
		{"SELECT TOP 5 a FROM t", "TOP", false, 7},
	}
	for _, c := range cases {
		if got := topLevelIndex(c.sql, c.kw, c.last); got != c.want {
			t.Errorf("%q %q last=%v: %d, want %d", c.sql, c.kw, c.last, got, c.want)
		}
	}
}
