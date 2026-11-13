package sqltext

import (
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"select a,b from t where x=1 and y=2 order by a desc limit 10",
			"SELECT a,\n  b\nFROM t\nWHERE x = 1\n  AND y = 2\nORDER BY a DESC\nLIMIT 10"},
		{"select u.id from users u left join orders o on o.uid=u.id",
			"SELECT u.id\nFROM users u\nLEFT JOIN orders o ON o.uid = u.id"},
		{"select 'a,b' as \"x,y\" -- c, d\nfrom t",
			"SELECT 'a,b' AS \"x,y\" -- c, d\nFROM t"},
		{"select * from t where a between 1 and 5 and b in (1,2,3)",
			"SELECT *\nFROM t\nWHERE a BETWEEN 1 AND 5\n  AND b IN (1, 2, 3)"},
		{"select 1; select 2", "SELECT 1;\nSELECT 2"},
	}
	for _, c := range cases {
		if got := Format(c.in); got != c.want {
			t.Errorf("Format(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

// Formatting must not change what the statement means: the tokens other than
// whitespace and keyword case are identical.
func TestFormatKeepsTokens(t *testing.T) {
	in := "select `Name`, 'x;y' , 1.5 from  \"T\"  where a<>b -- tail"
	out := Format(in)
	strip := func(s string) []string {
		var r []string
		for _, tk := range tokenize(s) {
			if tk.kind != tkSpace {
				r = append(r, upperIfKeyword(tk))
			}
		}
		return r
	}
	a, b := strip(in), strip(out)
	if len(a) != len(b) {
		t.Fatalf("token count %d vs %d: %q", len(a), len(b), out)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("token %d: %q vs %q", i, a[i], b[i])
		}
	}
}

func upperIfKeyword(tk token) string {
	if tk.kind == tkWord {
		return toUpper(tk.text)
	}
	return tk.text
}

func toUpper(s string) string { return strings.ToUpper(s) }
