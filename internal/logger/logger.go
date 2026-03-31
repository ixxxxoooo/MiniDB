// Package logger 提供统一的日志工具，所有日志写入文件并输出到终端，
// 方便运行时和事后排查问题。日志文件保存在 ~/.tableplus-ai/logs/ 目录下，
// 按日期自动分割。
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger

	logFile *os.File
)

// Init 初始化日志系统，需在应用启动时最先调用。
// 日志同时写入 ~/.tableplus-ai/logs/<date>.log 和 stderr。
func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	logDir := filepath.Join(homeDir, ".tableplus-ai", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	logPath := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stderr, logFile)

	flags := log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
	infoLogger = log.New(multiWriter, "[INFO]  ", flags)
	warnLogger = log.New(multiWriter, "[WARN]  ", flags)
	errorLogger = log.New(multiWriter, "[ERROR] ", flags)
	debugLogger = log.New(multiWriter, "[DEBUG] ", flags)

	infoLogger.Println("========== 日志系统初始化完成 ==========")
	infoLogger.Printf("日志文件路径: %s", logPath)
	return nil
}

// Close 关闭日志文件，应在应用退出时调用
func Close() {
	if logFile != nil {
		infoLogger.Println("========== 应用关闭，日志结束 ==========")
		logFile.Close()
	}
}

// Info 记录信息级别日志
func Info(format string, v ...interface{}) {
	if infoLogger != nil {
		infoLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Warn 记录警告级别日志
func Warn(format string, v ...interface{}) {
	if warnLogger != nil {
		warnLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Error 记录错误级别日志
func Error(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Debug 记录调试级别日志
func Debug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Output(2, fmt.Sprintf(format, v...))
	}
}
