// Package logger 提供统一的日志工具，所有日志写入文件并输出到终端，
// 方便运行时和事后排查问题。日志文件保存在应用数据目录的 logs/ 下（macOS：~/Library/Application Support/MiniDB/logs/），
// 按日期自动分割（跨天自动切换文件），启动时清理超过保留期的旧日志。
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"minidb/internal/appdata"
)

// retentionDays 是日志保留天数，超出后启动时自动清理。
const retentionDays = 14

var (
	mu sync.Mutex

	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger

	logFile    *os.File
	logFileDay string // 当前文件对应的日期 2006-01-02
	logPath    string
)

// Init 初始化日志系统，需在应用启动时最先调用。
// 日志同时写入应用数据目录下的 logs/<date>.log 和 stderr。
func Init() error {
	if err := appdata.EnsureLogsRootDir(); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	mu.Lock()
	err := openFileLocked(time.Now())
	mu.Unlock()
	if err != nil {
		return err
	}

	Info("========== 日志系统初始化完成 ==========")
	Info("日志文件路径: %s", CurrentPath())
	go cleanupOldLogs(retentionDays)
	return nil
}

// CurrentPath 返回当前正在写入的日志文件路径（跨天后会变化）。
func CurrentPath() string {
	mu.Lock()
	defer mu.Unlock()
	if logPath != "" {
		return logPath
	}
	return appdata.LogFilePath()
}

// openFileLocked 打开 now 当天的日志文件并重建各级别 logger。调用方须持有 mu。
func openFileLocked(now time.Time) error {
	day := now.Format("2006-01-02")
	path := filepath.Join(appdata.LogsRootPath(), day+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	_ = appdata.TightenFile(path) // 旧版本留下的 0644 日志也一并收紧
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile, logFileDay, logPath = f, day, path
	// 程序因未捕获的 panic 或运行时致命错误崩溃时，Go 把堆栈写到 stderr，
	// 从 Finder 或 Dock 启动时 stderr 无处可看。这里让它同时落到日志文件里，方便事后排查。
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})

	w := io.MultiWriter(os.Stderr, f)
	flags := log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
	infoLogger = log.New(w, "[INFO]  ", flags)
	warnLogger = log.New(w, "[WARN]  ", flags)
	errorLogger = log.New(w, "[ERROR] ", flags)
	debugLogger = log.New(w, "[DEBUG] ", flags)
	return nil
}

// rotateIfNeededLocked 跨天时切换到新一天的文件，避免长时间运行的应用一直写旧文件。
func rotateIfNeededLocked() {
	if logFile == nil {
		return
	}
	now := time.Now()
	if now.Format("2006-01-02") == logFileDay {
		return
	}
	_ = openFileLocked(now)
}

// cleanupOldLogs 删除超过保留期的 <date>.log 文件（只处理符合日期命名的文件，不误删 update.log 等）。
func cleanupOldLogs(days int) {
	entries, err := os.ReadDir(appdata.LogsRootPath())
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".log") {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(name, ".log"), time.Local)
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			_ = os.Remove(filepath.Join(appdata.LogsRootPath(), name))
		}
	}
}

// Close 关闭日志文件，应在应用退出时调用
func Close() {
	Info("========== 应用关闭，日志结束 ==========")
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

func output(pick func() *log.Logger, format string, v ...interface{}) {
	mu.Lock()
	rotateIfNeededLocked()
	l := pick()
	mu.Unlock()
	if l != nil {
		_ = l.Output(3, fmt.Sprintf(format, v...))
	}
}

// Info 记录信息级别日志
func Info(format string, v ...interface{}) {
	output(func() *log.Logger { return infoLogger }, format, v...)
}

// Warn 记录警告级别日志
func Warn(format string, v ...interface{}) {
	output(func() *log.Logger { return warnLogger }, format, v...)
}

// Error 记录错误级别日志
func Error(format string, v ...interface{}) {
	output(func() *log.Logger { return errorLogger }, format, v...)
}

// Debug 记录调试级别日志
func Debug(format string, v ...interface{}) {
	output(func() *log.Logger { return debugLogger }, format, v...)
}
