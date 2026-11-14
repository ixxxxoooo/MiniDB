package app

import (
	"os"
	"path/filepath"
	"testing"
)

// 覆盖写已存在的 0644 文件时，权限也要收紧到 0600。
func TestCreatePrivateFileTightensExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.sql")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("权限应为 0600, 实际 %v", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(path); string(data) != "new" {
		t.Fatalf("内容应被覆盖: %q", data)
	}
}
