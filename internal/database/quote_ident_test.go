package database

import "testing"

func TestQuoteIdent_SQLiteUsesDoubleQuotes(t *testing.T) {
	got := QuoteIdent("sqlite", `col"name`)
	want := `"col""name"`
	if got != want {
		t.Fatalf("QuoteIdent(sqlite)=%q want %q", got, want)
	}
}

func TestQuoteIdent_PostgresSameAsSQLiteStyle(t *testing.T) {
	if QuoteIdent("postgres", "id") != `"id"` {
		t.Fatal(QuoteIdent("postgres", "id"))
	}
}
