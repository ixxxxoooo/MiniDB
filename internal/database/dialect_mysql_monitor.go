package database

import (
	"context"
	"strconv"

	"minidb/internal/model"
)

// MySQL 与 MariaDB 的监控：指标来自 SHOW GLOBAL STATUS；
// 慢查询先看 performance_schema 的语句摘要，再看 mysql.slow_log 表（要求 log_output 含 TABLE）。

// mysqlStatusKeys 是要读的状态变量与指标键的对应。
var mysqlStatusKeys = map[string]string{
	"Threads_connected": MetricConnections,
	"Threads_running":   MetricRunning,
	"Questions":         MetricQueries,
	"Slow_queries":      MetricSlow,
	"Bytes_received":    MetricBytesIn,
	"Bytes_sent":        MetricBytesOut,
}

// mysqlSampleOf 把状态变量（名字到文本值）整理成指标。Threads_running 包含监控自己这一条，扣掉。
func mysqlSampleOf(status map[string]string) map[string]float64 {
	vals := map[string]float64{}
	for name, key := range mysqlStatusKeys {
		raw, ok := status[name]
		if !ok {
			continue
		}
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			vals[key] = v
		}
	}
	if v, ok := vals[MetricRunning]; ok && v > 0 {
		vals[MetricRunning] = v - 1
	}
	return vals
}

func (mysqlDialect) Sample(ctx context.Context, db Executor) (MonitorSample, error) {
	status := map[string]string{}
	err := queryEach(ctx, db, `SHOW GLOBAL STATUS WHERE Variable_name IN
		('Threads_connected', 'Threads_running', 'Questions', 'Slow_queries', 'Bytes_received', 'Bytes_sent')`, nil,
		func(rows rowScanner) error {
			var name, value string
			if err := rows.Scan(&name, &value); err != nil {
				return err
			}
			status[name] = value
			return nil
		})
	if err != nil {
		return MonitorSample{}, err
	}
	return MonitorSample{Values: mysqlSampleOf(status)}, nil
}

const mysqlSlowHint = "没有可用的慢查询数据源。可以任选其一开启：" +
	"1) 在配置里设置 performance_schema=ON 后重启（语句摘要，推荐）；" +
	"2) 执行 SET GLOBAL slow_query_log = 'ON'; SET GLOBAL log_output = 'TABLE'; 把慢查询记录到 mysql.slow_log 表"

func (d mysqlDialect) SlowQueries(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var psOn int
	_ = db.QueryRowContext(ctx, `SELECT @@performance_schema`).Scan(&psOn)
	digestOK := false
	if psOn == 1 {
		out, err := mysqlDigestSlow(ctx, db, limit)
		if err == nil {
			digestOK = true
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	out, err := mysqlSlowLog(ctx, db, limit)
	if err == nil && len(out) > 0 {
		return out, nil
	}
	if digestOK || err == nil {
		return nil, nil // 数据源可用，只是还没有记录
	}
	return nil, model.Errf(model.CodeMonitorUnavailable, "%s", mysqlSlowHint)
}

func mysqlDigestSlow(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var out []SlowQuery
	err := queryEach(ctx, db, `SELECT DIGEST_TEXT, COALESCE(SCHEMA_NAME, ''), COUNT_STAR,
			SUM_TIMER_WAIT / 1000000000, AVG_TIMER_WAIT / 1000000000, MAX_TIMER_WAIT / 1000000000,
			SUM_ROWS_EXAMINED, COALESCE(DATE_FORMAT(LAST_SEEN, '%Y-%m-%d %H:%i:%s'), '')
		FROM performance_schema.events_statements_summary_by_digest
		WHERE DIGEST_TEXT IS NOT NULL AND DIGEST_TEXT NOT LIKE 'SHOW GLOBAL STATUS%'
		ORDER BY AVG_TIMER_WAIT DESC LIMIT ?`, []any{limit}, func(rows rowScanner) error {
		q := SlowQuery{Source: "performance_schema"}
		if err := rows.Scan(&q.SQL, &q.Database, &q.Calls, &q.TotalMs, &q.AvgMs, &q.MaxMs, &q.Rows, &q.LastSeen); err != nil {
			return err
		}
		out = append(out, q)
		return nil
	})
	return out, err
}

func mysqlSlowLog(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var out []SlowQuery
	err := queryEach(ctx, db, `SELECT sql_text, db,
			TIME_TO_SEC(query_time) * 1000 + MICROSECOND(query_time) / 1000, rows_examined,
			DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s')
		FROM mysql.slow_log ORDER BY start_time DESC LIMIT ?`, []any{limit}, func(rows rowScanner) error {
		q := SlowQuery{Calls: 1, Source: "mysql.slow_log"}
		if err := rows.Scan(&q.SQL, &q.Database, &q.TotalMs, &q.Rows, &q.LastSeen); err != nil {
			return err
		}
		q.AvgMs, q.MaxMs = q.TotalMs, q.TotalMs
		out = append(out, q)
		return nil
	})
	return out, err
}
