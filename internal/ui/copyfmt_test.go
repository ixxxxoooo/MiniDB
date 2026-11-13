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
