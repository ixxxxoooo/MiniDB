package database

import (
	"context"
	"fmt"
)

// ClickHouse 的查询列表与终止查询。
//
// system.processes 的 query_id 是字符串，而 ProcessAdmin 用数字 ID；
// 这里用 cityHash64(query_id) 作为数字 ID，终止时再用 KILL QUERY WHERE cityHash64(query_id) = ? 反查。

func (clickhouseDialect) ProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	var out []ProcessInfo
	err := queryEach(ctx, db, `SELECT toString(cityHash64(query_id)), user, toString(address), current_database,
			multiIf(is_cancelled = 1, 'Cancelling', 'Query'), toInt64(elapsed), 'Running', query
		FROM system.processes WHERE query_id != queryID() ORDER BY elapsed DESC`, nil,
		func(rows rowScanner) error {
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

// KillProcess 终止查询。ClickHouse 没有「连接」可杀，queryOnly 对它没有区别。
// ID 已由 ParseProcessID 校验成纯数字，直接拼进语句。
func (clickhouseDialect) KillProcess(ctx context.Context, db Executor, id uint64, _ bool) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("KILL QUERY WHERE cityHash64(query_id) = %d", id))
	return err
}
