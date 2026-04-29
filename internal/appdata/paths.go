package appdata

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	appversion "tableplus-ai/internal/version"
)

// RootDir returns the application data root under the current user's home.
func RootDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return appversion.DataDirName
	}
	return filepath.Join(homeDir, appversion.DataDirName)
}

// EnsureRootDir creates the application data root if it does not exist.
func EnsureRootDir() error {
	return os.MkdirAll(RootDir(), 0755)
}

// DataFilePath returns the BoltDB file used by the application.
func DataFilePath() string {
	return filepath.Join(RootDir(), "data.db")
}

// LogsRootPath returns the directory that stores application logs.
func LogsRootPath() string {
	return filepath.Join(RootDir(), "logs")
}

// EnsureLogsRootDir creates the application logs root if it does not exist.
func EnsureLogsRootDir() error {
	return os.MkdirAll(LogsRootPath(), 0755)
}

// LogFilePath returns today's log file path.
func LogFilePath() string {
	return filepath.Join(LogsRootPath(), time.Now().Format("2006-01-02")+".log")
}

// SecretKeyPath returns the local key used to encrypt persisted secrets.
func SecretKeyPath() string {
	return filepath.Join(RootDir(), "secret.key")
}
