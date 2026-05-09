package appdata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appversion "minidb/internal/version"
)

const legacyDataDirName = ".tableplus-ai"

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
	root := RootDir()
	if _, err := os.Stat(root); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查数据目录失败: %w", err)
	}

	if err := migrateLegacyRootDir(root); err != nil {
		return err
	}
	return os.MkdirAll(root, 0755)
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

func legacyRootDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return legacyDataDirName
	}
	return filepath.Join(homeDir, legacyDataDirName)
}

func migrateLegacyRootDir(root string) error {
	if appversion.DataDirName == legacyDataDirName {
		return nil
	}
	legacy := legacyRootDir()
	if legacy == root {
		return nil
	}
	info, err := os.Stat(legacy)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查旧数据目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	if err := copyDir(legacy, root); err != nil {
		return fmt.Errorf("迁移旧数据目录失败: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
