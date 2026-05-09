package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// 创建临时 Store 用于测试
func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "minidb-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := newStoreWithPath(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("创建测试 Store 失败: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}
	return store, cleanup
}

// TestPutAndGet 测试存取操作
func TestPutAndGet(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := TestData{Name: "test", Value: 42}
	if err := store.Put("connections", "key1", input); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	var output TestData
	if err := store.Get("connections", "key1", &output); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if output.Name != input.Name || output.Value != input.Value {
		t.Errorf("数据不匹配: got=%+v want=%+v", output, input)
	}
}

// TestGetNotFound 测试获取不存在的 key
func TestGetNotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	var output struct{}
	err := store.Get("connections", "nonexistent", &output)
	if err == nil {
		t.Error("获取不存在的 key 应该返回错误")
	}
}

// TestDelete 测试删除操作
func TestDelete(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	if err := store.Put("connections", "key1", "value1"); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	if err := store.Delete("connections", "key1"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	var output string
	err := store.Get("connections", "key1", &output)
	if err == nil {
		t.Error("删除后仍能获取到数据")
	}
}

// TestList 测试列表操作
func TestList(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	store.Put("connections", "1", Item{ID: "1", Name: "first"})
	store.Put("connections", "2", Item{ID: "2", Name: "second"})

	items, err := store.List("connections", func() interface{} {
		return &Item{}
	})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("期望 2 个结果，得到 %d", len(items))
	}
}
