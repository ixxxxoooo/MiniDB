package ui

import (
	"strings"
	"testing"
)

func TestCopyFormats(t *testing.T) {
	b := block{
		cols: []string{"id", "name", "note"},
		rows: []map[string]any{
			{"id": int64(1), "name": "O'Brien", "note": nil},
			{"id": int64(2), "name": "a|b\tc", "note": "x\ny"},
		},
	}
	cases := []struct {
		kind int
		want string
	}{
		{fmtTSV, "id\tname\tnote\n1\tO'Brien\t\n2\ta|b c\tx y\n"},
		{fmtCSV, "id,name,note\n1,O'Brien,\n2,a|b\tc,\"x\ny\"\n"},
		{fmtInsert, "INSERT INTO `t` (`id`, `name`, `note`) VALUES (1, 'O''Brien', NULL);\nINSERT INTO `t` (`id`, `name`, `note`) VALUES (2, 'a|b\tc', 'x\ny');\n"},
		{fmtMarkdown, "| id | name | note |\n| --- | --- | --- |\n| 1 | O'Brien |  |\n| 2 | a\\|b\tc | x y |\n"},
	}
	for _, c := range cases {
		if got := b.format(c.kind, "mysql", "t", true); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", copyFormatNames[c.kind], got, c.want)
		}
	}
	js := b.format(fmtJSON, "mysql", "t", true)
	if !strings.Contains(js, `"note": null`) || !strings.Contains(js, `"name": "O'Brien"`) {
		t.Errorf("json: %s", js)
	}
	if got := b.format(fmtTSV, "mysql", "t", false); strings.HasPrefix(got, "id") {
		t.Error("no header requested")
	}
	if got := b.format(fmtInsert, "postgres", "t", true); !strings.HasPrefix(got, `INSERT INTO "t" ("id"`) {
		t.Errorf("postgres quoting: %s", got)
	}
}

// 单元格的几种复制写法：JSON 值、SQL 字面量、WHERE 条件、列名。
func TestCellCopyText(t *testing.T) {
	cases := []struct {
		kind int
		v    any
		want string
	}{
		{0, "a\"b", `"a\"b"`},
		{0, int64(7), `7`},
		{0, nil, `null`},
		{1, "it's", `'it''s'`},
		{1, nil, `NULL`},
		{2, "admin", "`role` = 'admin'"},
		{2, int64(3), "`role` = 3"},
		{2, nil, "`role` IS NULL"},
		{3, "x", "role"},
	}
	for _, c := range cases {
		if got := cellCopyText(c.kind, "mysql", "role", c.v); got != c.want {
			t.Errorf("kind %d %v: got %q want %q", c.kind, c.v, got, c.want)
		}
	}
}
