package database

import (
	"context"
	"fmt"
)

// SQL Server 的监控：指标来自 sys.dm_os_performance_counters 与 sys.dm_exec_requests，
// 慢查询来自 sys.dm_exec_query_stats。这些视图需要 VIEW SERVER STATE 权限，没有权限时错误原样显示。

func (sqlserverDialect) Sample(ctx context.Context, db Executor) (MonitorSample, error) {
	counters := map[string]float64{}
	err := queryEach(ctx, db, `SELECT RTRIM(counter_name), CAST(cntr_value AS float) FROM sys.dm_os_performance_counters
		WHERE counter_name IN ('User Connections', 'Batch Requests/sec', 'Buffer cache hit ratio', 'Buffer cache hit ratio base')`, nil,
		func(rows rowScanner) error {
			var name string
			var v float64
			if err := rows.Scan(&name, &v); err != nil {
				return err
			}
			counters[name] = v
			return nil
		})
	if err != nil {
		return MonitorSample{}, fmt.Errorf("读取性能计数器失败（需要 VIEW SERVER STATE 权限）: %w", err)
	}
	vals := map[string]float64{}
	if v, ok := counters["User Connections"]; ok {
		vals[MetricConnections] = v
	}
	// Batch Requests/sec 在 DMV 里是累计值
	if v, ok := counters["Batch Requests/sec"]; ok {
		vals[MetricQueries] = v
	}
	if base := counters["Buffer cache hit ratio base"]; base > 0 {
		vals[MetricCacheHit] = counters["Buffer cache hit ratio"] / base * 100
	}
	var running float64
	if err := db.QueryRowContext(ctx, `SELECT CAST(COUNT(*) AS float) FROM sys.dm_exec_requests r
		JOIN sys.dm_exec_sessions s ON s.session_id = r.session_id WHERE s.is_user_process = 1 AND r.session_id <> @@SPID`).Scan(&running); err == nil {
		vals[MetricRunning] = running
	}
	return MonitorSample{Values: vals}, nil
}

func (sqlserverDialect) SlowQueries(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var out []SlowQuery
	err := queryEach(ctx, db, `SELECT TOP (@p1) st.text, COALESCE(DB_NAME(st.dbid), ''), qs.execution_count,
			CAST(qs.total_elapsed_time AS float) / 1000, CAST(qs.total_elapsed_time AS float) / 1000 / qs.execution_count,
			CAST(qs.max_elapsed_time AS float) / 1000, qs.total_rows, CONVERT(varchar(19), qs.last_execution_time, 120)
		FROM sys.dm_exec_query_stats qs CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
		WHERE st.text IS NOT NULL AND qs.execution_count > 0
		ORDER BY CAST(qs.total_elapsed_time AS float) / qs.execution_count DESC`, []any{limit}, func(rows rowScanner) error {
		q := SlowQuery{Source: "sys.dm_exec_query_stats"}
		if err := rows.Scan(&q.SQL, &q.Database, &q.Calls, &q.TotalMs, &q.AvgMs, &q.MaxMs, &q.Rows, &q.LastSeen); err != nil {
			return err
		}
		out = append(out, q)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("读取查询统计失败（需要 VIEW SERVER STATE 权限）: %w", err)
	}
	return out, nil
}
