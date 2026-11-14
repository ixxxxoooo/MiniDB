package database

import (
	"context"
	"fmt"
)

// SQL Server 的会话列表与终止会话。

func (sqlserverDialect) ProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	var out []ProcessInfo
	err := queryEach(ctx, db, `SELECT CAST(s.session_id AS varchar(20)), COALESCE(s.login_name, ''), COALESCE(s.host_name, ''),
			COALESCE(DB_NAME(s.database_id), ''), COALESCE(r.command, ''),
			COALESCE(DATEDIFF(SECOND, COALESCE(r.start_time, s.last_request_start_time), GETDATE()), 0),
			COALESCE(r.status, s.status, ''), COALESCE(t.text, '')
		FROM sys.dm_exec_sessions s
		LEFT JOIN sys.dm_exec_requests r ON r.session_id = s.session_id
		LEFT JOIN sys.dm_exec_connections c ON c.session_id = s.session_id
		OUTER APPLY sys.dm_exec_sql_text(COALESCE(r.sql_handle, c.most_recent_sql_handle)) t
		WHERE s.is_user_process = 1 AND s.session_id <> @@SPID
		ORDER BY s.session_id`, nil, func(rows rowScanner) error {
		var p ProcessInfo
		if err := rows.Scan(&p.ID, &p.User, &p.Host, &p.Database, &p.Command, &p.TimeSec, &p.State, &p.Info); err != nil {
			return err
		}
		p.Info = truncateProcessInfo(p.Info)
		out = append(out, p)
		return nil
	})
	return out, err
}

// KillProcess 终止会话。SQL Server 没有只取消语句的 KILL，queryOnly 时同样终止整个会话。
// KILL 不接受参数，ID 已由 ParseProcessID 校验成纯数字后拼进语句。
func (sqlserverDialect) KillProcess(ctx context.Context, db Executor, id uint64, _ bool) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("KILL %d", id))
	return err
}
