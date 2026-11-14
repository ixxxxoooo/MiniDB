package database

import (
	"context"

	"minidb/internal/model"
)

// PostgreSQL 的监控：指标来自 pg_stat_database 与 pg_stat_activity（全部库合计）；
// 慢查询来自 pg_stat_statements 扩展。PostgreSQL 没有累计语句数，「查询」指标用累计事务数代替。

func (postgresDialect) Sample(ctx context.Context, db Executor) (MonitorSample, error) {
	var conns, txns, hit, read float64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(numbackends), 0)::float8, COALESCE(SUM(xact_commit + xact_rollback), 0)::float8,
			COALESCE(SUM(blks_hit), 0)::float8, COALESCE(SUM(blks_read), 0)::float8 FROM pg_stat_database`).Scan(&conns, &txns, &hit, &read); err != nil {
		return MonitorSample{}, err
	}
	vals := map[string]float64{MetricConnections: conns, MetricQueries: txns}
	if hit+read > 0 {
		vals[MetricCacheHit] = hit / (hit + read) * 100
	}
	var running float64
	if err := db.QueryRowContext(ctx, `SELECT count(*)::float8 FROM pg_stat_activity WHERE state = 'active' AND pid <> pg_backend_pid()`).Scan(&running); err == nil {
		vals[MetricRunning] = running
	}
	return MonitorSample{Values: vals}, nil
}

const postgresSlowHint = "没有启用 pg_stat_statements。需要在 postgresql.conf 里设置 shared_preload_libraries = 'pg_stat_statements' 并重启，" +
	"然后在要监控的库里执行 CREATE EXTENSION pg_stat_statements;"

func (postgresDialect) SlowQueries(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var has bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')`).Scan(&has); err != nil {
		return nil, err
	}
	if !has {
		return nil, model.Errf(model.CodeMonitorUnavailable, "%s", postgresSlowHint)
	}
	// PostgreSQL 13 起列名改为 *_exec_time，旧版本是 total_time / mean_time / max_time
	cols := [][3]string{{"total_exec_time", "mean_exec_time", "max_exec_time"}, {"total_time", "mean_time", "max_time"}}
	var lastErr error
	for _, c := range cols {
		var out []SlowQuery
		err := queryEach(ctx, db, `SELECT s.query, COALESCE(d.datname, ''), s.calls, s.`+c[0]+`, s.`+c[1]+`, s.`+c[2]+`, s.rows
			FROM pg_stat_statements s LEFT JOIN pg_database d ON d.oid = s.dbid
			ORDER BY s.`+c[1]+` DESC LIMIT $1`, []any{limit}, func(rows rowScanner) error {
			q := SlowQuery{Source: "pg_stat_statements"}
			if err := rows.Scan(&q.SQL, &q.Database, &q.Calls, &q.TotalMs, &q.AvgMs, &q.MaxMs, &q.Rows); err != nil {
				return err
			}
			out = append(out, q)
			return nil
		})
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	// 扩展已创建但没有预加载时，查询会报错：把办法一并给出
	return nil, model.Errf(model.CodeMonitorUnavailable, "%s\n（%v）", postgresSlowHint, lastErr)
}
