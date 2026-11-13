package app

import (
	"path/filepath"
	"testing"

	"minidb/internal/database"
)

func newSQLiteDatabaseService(t *testing.T) *DatabaseService {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "test", Type: "sqlite", Database: filepath.Join(t.TempDir(), "o.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	return NewDatabaseService(manager, nil)
}

func TestCloneTableAndSearch(t *testing.T) {
	svc := newSQLiteDatabaseService(t)
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO users(name) VALUES ('alice'), ('bob'), ('alicia')`,
		`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)`,
		`INSERT INTO notes(body) VALUES ('nothing here')`,
	} {
		if err := svc.ExecuteRawSQL("c1", "", s); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.CloneTable("c1", "", "users", "users_bak", true); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.SearchDatabase("c1", "", "ali")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Count != 2 || hits[1].Count != 2 {
		t.Fatalf("应命中 users 与 users_bak 各 2 行: %+v", hits)
	}
	if err := svc.CloneTable("c1", "", "users", "users_bak", false); err == nil {
		t.Fatal("目标表已存在应报错")
	}
	if _, err := svc.SearchDatabase("c1", "", "  "); err == nil {
		t.Fatal("空关键字应报错")
	}
}

func TestDatabaseLevelStatementsRefusedOnSQLite(t *testing.T) {
	svc := newSQLiteDatabaseService(t)
	if err := svc.CreateDatabase("c1", "x", ""); err == nil {
		t.Fatal("SQLite 不支持新建库")
	}
	if err := svc.DropDatabase("c1", "x"); err == nil {
		t.Fatal("SQLite 不支持删库")
	}
}

func TestObjectsOnReadOnlyConnection(t *testing.T) {
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Type: "sqlite", Database: filepath.Join(t.TempDir(), "r.db"), ReadOnly: true}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	svc := NewDatabaseService(manager, nil)
	if err := svc.CloneTable("c1", "", "a", "b", false); err == nil {
		t.Fatal("只读连接不应允许克隆")
	}
}
