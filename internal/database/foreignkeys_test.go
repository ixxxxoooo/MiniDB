package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteColumnsCarryForeignKeys(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id), note TEXT)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	cols, err := getSQLiteColumns(db, "orders")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range cols {
		got[c.Name] = c.ForeignKey
	}
	if got["user_id"] != "users.id" || got["id"] != "" || got["note"] != "" {
		t.Fatalf("外键 = %v", got)
	}
}

func TestSplitForeignKey(t *testing.T) {
	for in, want := range map[string][2]string{"users.id": {"users", "id"}, "users": {"users", ""}, "a.b.c": {"a.b", "c"}} {
		if tb, col := SplitForeignKey(in); tb != want[0] || col != want[1] {
			t.Errorf("SplitForeignKey(%q) = %q, %q", in, tb, col)
		}
	}
}
