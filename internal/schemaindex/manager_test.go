package schemaindex

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/storage"
	"testing"
	"time"
)

func newTestManager(t *testing.T, builder BuilderFunc) *Manager {
	t.Helper()

	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	resolver := func(connID string) (*database.ConnectionConfig, bool) {
		return &database.ConnectionConfig{
			ID:       connID,
			Type:     "mysql",
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "tester",
			Database: "db1",
		}, true
	}

	return NewManager(store, resolver, builder)
}

func testSchema(version string) *ai.SchemaContext {
	return &ai.SchemaContext{
		DatabaseType:    "mysql",
		DatabaseName:    "db1",
		DatabaseVersion: version,
		Tables: []ai.TableSchema{
			{
				Name:    "users",
				Comment: "user table",
				Columns: []ai.ColumnSchema{
					{Name: "id", Type: "bigint", IsPrimary: true},
					{Name: "name", Type: "varchar(255)", Nullable: false},
				},
			},
		},
	}
}

func TestManagerUsesPersistedRecordBeforeRebuild(t *testing.T) {
	var buildCount int32
	builder := func(connID, dbName string) (*ai.SchemaContext, error) {
		atomic.AddInt32(&buildCount, 1)
		return testSchema("v1"), nil
	}

	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	resolver := func(connID string) (*database.ConnectionConfig, bool) {
		return &database.ConnectionConfig{
			ID:       connID,
			Type:     "mysql",
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "tester",
			Database: "db1",
		}, true
	}

	managerA := NewManager(store, resolver, builder)
	if _, err := managerA.GetSchema(context.Background(), "conn-a", "db1"); err != nil {
		t.Fatalf("GetSchema first build failed: %v", err)
	}
	if got := atomic.LoadInt32(&buildCount); got != 1 {
		t.Fatalf("build count after first GetSchema = %d, want 1", got)
	}

	managerB := NewManager(store, resolver, builder)
	schema, err := managerB.GetSchema(context.Background(), "conn-b", "db1")
	if err != nil {
		t.Fatalf("GetSchema from persisted failed: %v", err)
	}
	if schema.DatabaseVersion != "v1" {
		t.Fatalf("persisted schema version = %s, want v1", schema.DatabaseVersion)
	}
	if got := atomic.LoadInt32(&buildCount); got != 1 {
		t.Fatalf("build count after persisted load = %d, want 1", got)
	}
}

func TestManagerDirtySchemaTriggersSynchronousRefresh(t *testing.T) {
	var currentVersion atomic.Value
	currentVersion.Store("v1")

	var buildCount int32
	manager := newTestManager(t, func(connID, dbName string) (*ai.SchemaContext, error) {
		atomic.AddInt32(&buildCount, 1)
		return testSchema(currentVersion.Load().(string)), nil
	})

	if _, err := manager.GetSchema(context.Background(), "conn-a", "db1"); err != nil {
		t.Fatalf("seed GetSchema failed: %v", err)
	}

	target, err := manager.resolveTarget("conn-a", "db1")
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}
	entry, ok, err := manager.loadEntry(target.schemaKey)
	if err != nil || !ok {
		t.Fatalf("loadEntry failed: ok=%v err=%v", ok, err)
	}
	record := entry.record
	record.Dirty = true
	record.RefreshReason = RefreshReasonDDL
	if err := manager.persistRecord(record); err != nil {
		t.Fatalf("persistRecord failed: %v", err)
	}
	manager.setEntry(record, SourceMemory, false)

	currentVersion.Store("v2")
	schema, err := manager.GetSchema(context.Background(), "conn-a", "db1")
	if err != nil {
		t.Fatalf("GetSchema dirty refresh failed: %v", err)
	}
	if schema.DatabaseVersion != "v2" {
		t.Fatalf("schema version after dirty refresh = %s, want v2", schema.DatabaseVersion)
	}
	if got := atomic.LoadInt32(&buildCount); got != 2 {
		t.Fatalf("build count after dirty refresh = %d, want 2", got)
	}
}

func TestManagerSingleflightRefresh(t *testing.T) {
	var buildCount int32
	manager := newTestManager(t, func(connID, dbName string) (*ai.SchemaContext, error) {
		atomic.AddInt32(&buildCount, 1)
		time.Sleep(100 * time.Millisecond)
		return testSchema("v1"), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if _, err := manager.GetSchema(context.Background(), "conn-a", "db1"); err != nil {
				t.Errorf("goroutine %d GetSchema failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&buildCount); got != 1 {
		t.Fatalf("build count after concurrent GetSchema = %d, want 1", got)
	}
}
