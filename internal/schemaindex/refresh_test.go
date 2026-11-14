package schemaindex

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"minidb/internal/ai"
	"minidb/internal/database"
)

func TestStableSchemaKeyNormalizesServerFields(t *testing.T) {
	a := &database.ConnectionConfig{Type: database.MySQL, Host: " DB.Example.com ", Port: 3306, User: " root "}
	b := &database.ConnectionConfig{Type: database.MySQL, Host: "db.example.com", Port: 3306, User: "root"}
	ka, err := stableSchemaKey(a, " shop ")
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := stableSchemaKey(b, "shop")
	if ka != kb {
		t.Fatalf("空白和主机名大小写不同的同一个库应得到同一个键: %q vs %q", ka, kb)
	}
	other, _ := stableSchemaKey(b, "other")
	if other == kb {
		t.Fatal("不同的库应得到不同的键")
	}
}

func TestStableSchemaKeyForSQLiteUsesAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.db")
	k1, err := stableSchemaKey(&database.ConnectionConfig{Type: database.SQLite, Database: p}, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	// 路径里多绕一下（dir/./x/../a.db）仍是同一个文件
	k2, _ := stableSchemaKey(&database.ConnectionConfig{Type: database.SQLite, Database: filepath.Join(dir, ".", "x", "..", "a.db")}, "other")
	if k1 != k2 || !strings.HasPrefix(k1, "sqlite|") {
		t.Fatalf("SQLite 按文件路径标识: %q vs %q", k1, k2)
	}
}

func TestResolveTargetErrors(t *testing.T) {
	m := newTestManager(t, func(context.Context, string, string) (*ai.SchemaContext, error) { return testSchema("v1"), nil })
	if _, err := m.resolveTarget("  ", "db1"); err == nil {
		t.Fatal("连接 ID 为空应报错")
	}
	m.resolveConfig = func(string) (*database.ConnectionConfig, bool) { return nil, false }
	if _, err := m.resolveTarget("c1", "db1"); err == nil {
		t.Fatal("找不到连接配置应报错")
	}
}

func TestResolveTargetFallsBackToConfigDatabase(t *testing.T) {
	m := newTestManager(t, func(context.Context, string, string) (*ai.SchemaContext, error) { return testSchema("v1"), nil })
	target, err := m.resolveTarget("c1", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.dbName != "db1" || target.connID != "c1" {
		t.Fatalf("库名为空时应取连接配置里的库: %+v", target)
	}
}

func TestResolveTargetNeedsADatabase(t *testing.T) {
	m := newTestManager(t, func(context.Context, string, string) (*ai.SchemaContext, error) { return testSchema("v1"), nil })
	m.resolveConfig = func(id string) (*database.ConnectionConfig, bool) {
		return &database.ConnectionConfig{ID: id, Type: database.MySQL, Host: "h", Port: 3306}, true
	}
	if _, err := m.resolveTarget("c1", ""); err == nil {
		t.Fatal("库名和配置里的库都为空应报错")
	}
}
