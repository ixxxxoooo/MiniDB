package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootDirIsUnderConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := RootDir(); filepath.Base(got) != "MiniDB" || filepath.Dir(got) == "." {
		t.Fatalf("RootDir() = %q", got)
	}
	if LegacyRootDir() != filepath.Join(home, ".minidb") {
		t.Fatalf("LegacyRootDir() = %q", LegacyRootDir())
	}
}

func TestMigrateLegacyFilesCopiesKnownHostsOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(LegacyRootDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(LegacyRootDir(), "known_hosts")
	if err := os.WriteFile(legacy, []byte("host-a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyFiles(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(KnownHostsPath())
	if string(got) != "host-a" {
		t.Fatalf("known_hosts = %q", got)
	}
	// A later legacy change must not overwrite the new file.
	os.WriteFile(legacy, []byte("host-b"), 0o600)
	if err := MigrateLegacyFiles(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(KnownHostsPath())
	if string(got) != "host-a" {
		t.Fatalf("known_hosts overwritten: %q", got)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("legacy file must remain")
	}
}
