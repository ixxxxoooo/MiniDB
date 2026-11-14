package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ProcessInfo 是一条服务端会话/进程信息（MySQL PROCESSLIST 或 PostgreSQL pg_stat_activity）。
type ProcessInfo struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Host     string `json:"host"`
	Database string `json:"database"`
	Command  string `json:"command"`
	TimeSec  int64  `json:"timeSec"`
	State    string `json:"state"`
	Info     string `json:"info"` // 正在执行的 SQL（已截断）
}

const maxProcessInfoLen = 4000

// SupportsProcessList 该方言是否支持查看/终止进程。
func SupportsProcessList(dbType DBType) bool { return CapsOf(dbType).ProcessList }

func processAdmin(dbType DBType) (ProcessAdmin, error) {
	d, err := DialectOf(dbType)
	if err == nil && d.Caps().ProcessList {
		if pa, ok := d.(ProcessAdmin); ok {
			return pa, nil
		}
	}
	return nil, fmt.Errorf("%s 不支持查看进程列表", dbType)
}

// GetProcessList 返回当前所有会话（排除本连接自身）。
func GetProcessList(ctx context.Context, db Executor, dbType DBType) ([]ProcessInfo, error) {
	pa, err := processAdmin(dbType)
	if err != nil {
		return nil, err
	}
	return pa.ProcessList(ctx, db)
}

func truncateProcessInfo(s string) string {
	if r := []rune(s); len(r) > maxProcessInfoLen {
		return string(r[:maxProcessInfoLen]) + "…"
	}
	return s
}

// ParseProcessID 校验进程 ID 为纯数字。KILL 语句无法使用占位符，ID 会被拼进 SQL，必须严格校验。
func ParseProcessID(id string) (uint64, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的进程 ID: %q", id)
	}
	return n, nil
}

// KillProcess 终止会话。queryOnly=true 只中断当前正在执行的语句并保留连接；否则断开整个连接。
func KillProcess(ctx context.Context, db Executor, dbType DBType, id string, queryOnly bool) error {
	n, err := ParseProcessID(id)
	if err != nil {
		return err
	}
	pa, err := processAdmin(dbType)
	if err != nil {
		return fmt.Errorf("%s 不支持终止进程", dbType)
	}
	return pa.KillProcess(ctx, db, n, queryOnly)
}
