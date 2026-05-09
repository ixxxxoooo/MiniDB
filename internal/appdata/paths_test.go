package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRootDirMigratesLegacyData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, legacyDataDirName)
	if err := os.MkdirAll(filepath.Join(legacy, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "data.db"), []byte("legacy-data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "logs", "app.log"), []byte("log"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureRootDir(); err != nil {
		t.Fatalf("EnsureRootDir() error = %v", err)
	}

	copied, err := os.ReadFile(filepath.Join(RootDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != "legacy-data" {
		t.Fatalf("copied data = %q", copied)
	}
	if _, err := os.Stat(filepath.Join(legacy, "data.db")); err != nil {
		t.Fatalf("legacy data should remain: %v", err)
	}
}

func TestEnsureRootDirDoesNotOverwriteExistingTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := RootDir()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.db"), []byte("new-data"), 0600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(home, legacyDataDirName)
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "data.db"), []byte("legacy-data"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureRootDir(); err != nil {
		t.Fatalf("EnsureRootDir() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-data" {
		t.Fatalf("target data was overwritten: %q", data)
	}
}
