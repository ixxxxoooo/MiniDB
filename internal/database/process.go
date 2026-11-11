package database

import (
	"context"
	"database/sql"
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
func SupportsProcessList(dbType string) bool {
	return IsMySQLCompatible(dbType) || dbType == "postgres"
}

// GetProcessList 返回当前所有会话（排除本连接自身）。
func GetProcessList(ctx context.Context, db Executor, dbType string) ([]ProcessInfo, error) {
	switch {
	case IsMySQLCompatible(dbType):
		return getMySQLProcessList(ctx, db)
	case dbType == "postgres":
		return getPostgresProcessList(ctx, db)
	default:
		return nil, fmt.Errorf("%s 不支持查看进程列表", dbType)
	}
}

func getMySQLProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// 不同发行版（MySQL / TiDB / StarRocks）的列数和顺序不同，按列名（忽略大小写）取值
	idx := map[string]int{}
	for i, c := range cols {
		idx[strings.ToLower(c)] = i
	}
	var out []ProcessInfo
	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		get := func(name string) string {
			if i, ok := idx[name]; ok {
				return raw[i].String
			}
			return ""
		}
		sec, _ := strconv.ParseInt(get("time"), 10, 64)
		out = append(out, ProcessInfo{
			ID: get("id"), User: get("user"), Host: get("host"), Database: get("db"),
			Command: get("command"), TimeSec: sec, State: get("state"), Info: truncateProcessInfo(get("info")),
		})
	}
	return out, rows.Err()
}

func getPostgresProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT pid::text, COALESCE(usename, ''), COALESCE(client_addr::text, ''), COALESCE(datname, ''),
		       COALESCE(backend_type, ''), COALESCE(EXTRACT(EPOCH FROM (now() - query_start))::bigint, 0),
		       COALESCE(state, ''), COALESCE(query, '')
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		ORDER BY query_start NULLS LAST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProcessInfo
	for rows.Next() {
		var p ProcessInfo
		if err := rows.Scan(&p.ID, &p.User, &p.Host, &p.Database, &p.Command, &p.TimeSec, &p.State, &p.Info); err != nil {
			return nil, err
		}
		p.Info = truncateProcessInfo(p.Info)
		out = append(out, p)
	}
	return out, rows.Err()
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
func KillProcess(ctx context.Context, db Executor, dbType, id string, queryOnly bool) error {
	n, err := ParseProcessID(id)
	if err != nil {
		return err
	}
	switch {
	case IsMySQLCompatible(dbType):
		kind := "CONNECTION"
		if queryOnly {
			kind = "QUERY"
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("KILL %s %d", kind, n))
	case dbType == "postgres":
		fn := "pg_terminate_backend"
		if queryOnly {
			fn = "pg_cancel_backend"
		}
		var ok bool
		if err = db.QueryRowContext(ctx, "SELECT "+fn+"($1)", int64(n)).Scan(&ok); err == nil && !ok {
			err = fmt.Errorf("未能终止进程 %d（可能已结束，或没有权限）", n)
		}
	default:
		return fmt.Errorf("%s 不支持终止进程", dbType)
	}
	return err
}
