package sqltext

import (
	"reflect"
	"testing"
)

func eq(t *testing.T, sql string, want []string) {
	t.Helper()
	got := Texts(sql)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split(%q)\n got %q\nwant %q", sql, got, want)
	}
}

func TestSplit(t *testing.T) {
	eq(t, "SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"})
	eq(t, "SELECT *\nFROM users\nWHERE id = 1", []string{"SELECT *\nFROM users\nWHERE id = 1"})
	for _, s := range []string{"INSERT INTO a (x)\nSELECT x FROM b", "CREATE TABLE t AS\nSELECT 1", "EXPLAIN\nSELECT 1"} {
		eq(t, s, []string{s})
	}
	eq(t, "SELECT 'a;b'; SELECT \"c;d\"", []string{"SELECT 'a;b'", `SELECT "c;d"`})
	eq(t, "SELECT `a;b` FROM t; SELECT 2", []string{"SELECT `a;b` FROM t", "SELECT 2"})
	eq(t, "SELECT 'it''s;ok'; SELECT 2", []string{"SELECT 'it''s;ok'", "SELECT 2"})
	eq(t, `SELECT 'a\';b'; SELECT 2`, []string{`SELECT 'a\';b'`, "SELECT 2"})
	eq(t, "SELECT 1 -- a;b\n; SELECT 2", []string{"SELECT 1 -- a;b", "SELECT 2"})
	eq(t, "/* x; y */ SELECT 1", []string{"/* x; y */ SELECT 1"})
	eq(t, "-- just a comment\n;", nil)
	eq(t, "# mysql comment; still comment\nSELECT 1", []string{"# mysql comment; still comment\nSELECT 1"})
	fn := "CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql"
	eq(t, fn+"; SELECT 1", []string{fn, "SELECT 1"})
	eq(t, "SELECT $tag$a;b$tag$", []string{"SELECT $tag$a;b$tag$"})
	eq(t, "", nil)
	eq(t, "  \n ; ; ", nil)
	eq(t, "SELECT '中文;分号'; SELECT 2", []string{"SELECT '中文;分号'", "SELECT 2"})
}

func TestSplitRanges(t *testing.T) {
	src := "  SELECT 1;\n\nSELECT 2  ;"
	for _, s := range Split(src) {
		if src[s.Start:s.End] != s.Text {
			t.Errorf("range %d..%d = %q, text %q", s.Start, s.End, src[s.Start:s.End], s.Text)
		}
	}
}

func TestAt(t *testing.T) {
	src := "SELECT 1;\nSELECT 2;\n\nSELECT 3"
	cases := map[int]string{0: "SELECT 1", 9: "SELECT 1", 10: "SELECT 2", 19: "SELECT 2", 20: "SELECT 2", 21: "SELECT 3", 22: "SELECT 3", len(src): "SELECT 3"}
	for caret, want := range cases {
		got, ok := At(src, caret)
		if !ok || got.Text != want {
			t.Errorf("At(%d) = %q, want %q", caret, got.Text, want)
		}
	}
	if _, ok := At("  ", 0); ok {
		t.Error("blank script has no statement")
	}
}
