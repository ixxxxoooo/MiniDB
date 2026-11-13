package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RootDir returns the application data root: the user's config directory
// (~/Library/Application Support/MiniDB on macOS). Tests redirect it by
// setting HOME.
func RootDir() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "MiniDB")
	}
	return "MiniDB"
}

// LegacyRootDir is where the Wails-era app kept its data (~/.minidb).
func LegacyRootDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".minidb"
	}
	return filepath.Join(home, ".minidb")
}

// EnsureRootDir creates the application data root.
func EnsureRootDir() error {
	if err := os.MkdirAll(RootDir(), 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	return nil
}

// DataFilePath returns the SQLite file holding all app state.
func DataFilePath() string { return filepath.Join(RootDir(), "minidb.sqlite") }

// LegacyDataFilePath is the BoltDB file of the Wails-era app.
func LegacyDataFilePath() string { return filepath.Join(LegacyRootDir(), "data.db") }

// KnownHostsPath returns the file that records trusted SSH host keys (trust on first use).
func KnownHostsPath() string { return filepath.Join(RootDir(), "known_hosts") }

// LogsRootPath returns the directory that stores application logs.
func LogsRootPath() string { return filepath.Join(RootDir(), "logs") }

// EnsureLogsRootDir creates the application logs root if it does not exist.
func EnsureLogsRootDir() error { return os.MkdirAll(LogsRootPath(), 0o755) }

// LogFilePath returns today's log file path.
func LogFilePath() string {
	return filepath.Join(LogsRootPath(), time.Now().Format("2006-01-02")+".log")
}

// SecretKeyPath is the fallback file for the local encryption key, used only
// where no system credential store is available.
func SecretKeyPath() string { return filepath.Join(RootDir(), "secret.key") }

// LegacySecretKeyPath is the key file of the Wails-era app.
func LegacySecretKeyPath() string { return filepath.Join(LegacyRootDir(), "secret.key") }

// MigrateLegacyFiles copies small legacy files that have no new counterpart
// yet (currently the SSH known_hosts). Legacy files are never removed.
func MigrateLegacyFiles() error {
	if err := EnsureRootDir(); err != nil {
		return err
	}
	src, dst := filepath.Join(LegacyRootDir(), "known_hosts"), KnownHostsPath()
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return nil // nothing to migrate
	}
	return os.WriteFile(dst, data, 0o600)
}
