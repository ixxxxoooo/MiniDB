package sqltext

import "testing"

var schema = []Table{
	{"users", []Column{{"id", "int"}, {"name", "text"}, {"email", "text"}}},
	{"orders", []Column{{"id", "int"}, {"user_id", "int"}, {"total", "real"}}},
}

func labels(items []Item) []string {
	var out []string
	for _, i := range items {
		out = append(out, i.Label)
	}
	return out
}

func TestCompleteAliasColumns(t *testing.T) {
	src := "SELECT u.na FROM users u JOIN orders o ON o.user_id = u.id"
	off := len("SELECT u.na")
	got := labels(Complete(src, off, "na", schema))
	if len(got) != 1 || got[0] != "name" {
		t.Fatalf("u.na → %q", got)
	}
	src2 := "SELECT o. FROM users u JOIN orders o ON 1=1"
	got = labels(Complete(src2, len("SELECT o."), "", schema))
	if len(got) != 3 || got[0] != "id" || got[1] != "user_id" {
		t.Fatalf("o. → %q", got)
	}
}

func TestCompleteTablesAndColumnsOfStatement(t *testing.T) {
	src := "SELECT * FROM users WHERE em"
	got := Complete(src, len(src), "em", schema)
	if len(got) == 0 || got[0].Label != "email" || got[0].Kind != KindColumn {
		t.Fatalf("em → %+v", got)
	}
	// Columns of tables not in this statement are not offered.
	for _, it := range Complete("SELECT * FROM users WHERE to", len("SELECT * FROM users WHERE to"), "to", schema) {
		if it.Label == "total" {
			t.Fatal("total belongs to orders, which this statement does not use")
		}
	}
	got = Complete("SELECT * FROM us", len("SELECT * FROM us"), "us", schema)
	if len(got) == 0 || got[0].Label != "users" || got[0].Kind != KindTable {
		t.Fatalf("us → %+v", got)
	}
	kw := Complete("sel", 3, "sel", schema)
	if len(kw) == 0 || kw[0].Label != "SELECT" {
		t.Fatalf("sel → %+v", kw)
	}
}

func TestCompleteScopedToCurrentStatement(t *testing.T) {
	src := "SELECT * FROM orders;\nSELECT tot"
	for _, it := range Complete(src, len(src), "tot", schema) {
		if it.Kind == KindColumn {
			t.Fatalf("second statement uses no table, got column %q", it.Label)
		}
	}
}
