package database

import (
	"context"

	"minidb/internal/model"
)

// ClickHouse 的监控：指标来自 system.metrics（当前值）与 system.events（累计值），
// 慢查询来自 system.query_log，按语句模板归并。

func (clickhouseDialect) Sample(ctx context.Context, db Executor) (MonitorSample, error) {
	vals := map[string]float64{}
	var conns, running float64
	err := queryEach(ctx, db, `SELECT metric, toFloat64(value) FROM system.metrics
		WHERE metric IN ('TCPConnection', 'HTTPConnection', 'MySQLConnection', 'PostgreSQLConnection', 'InterserverConnection', 'Query')`, nil,
		func(rows rowScanner) error {
			var name string
			var v float64
			if err := rows.Scan(&name, &v); err != nil {
				return err
			}
			if name == "Query" {
				running = v
			} else if name != "InterserverConnection" {
				conns += v
			}
			return nil
		})
	if err != nil {
		return MonitorSample{}, err
	}
	vals[MetricConnections] = conns
	if running > 0 {
		running-- // 扣掉监控自己这一条
	}
	vals[MetricRunning] = running
	err = queryEach(ctx, db, `SELECT event, toFloat64(value) FROM system.events
		WHERE event IN ('Query', 'NetworkReceiveBytes', 'NetworkSendBytes')`, nil,
		func(rows rowScanner) error {
			var name string
			var v float64
			if err := rows.Scan(&name, &v); err != nil {
				return err
			}
			switch name {
			case "Query":
				vals[MetricQueries] = v
			case "NetworkReceiveBytes":
				vals[MetricBytesIn] = v
			case "NetworkSendBytes":
				vals[MetricBytesOut] = v
			}
			return nil
		})
	return MonitorSample{Values: vals}, err
}

const clickhouseSlowHint = "system.query_log 不可用。ClickHouse 默认会开启它；如果是自建配置，请在服务端配置里加上 <query_log> 段并重启。"

func (clickhouseDialect) SlowQueries(ctx context.Context, db Executor, limit int) ([]SlowQuery, error) {
	var out []SlowQuery
	err := queryEach(ctx, db, `SELECT any(query), any(current_database), toInt64(count()),
			toFloat64(sum(query_duration_ms)), toFloat64(avg(query_duration_ms)), toFloat64(max(query_duration_ms)),
			toInt64(sum(read_rows)), toString(max(event_time))
		FROM system.query_log
		WHERE type = 'QueryFinish' AND is_initial_query AND event_date >= today() - 1
			AND query_kind IN ('Select', 'Insert') AND NOT has(databases, 'system')
		GROUP BY normalized_query_hash ORDER BY avg(query_duration_ms) DESC LIMIT ?`, []any{limit}, func(rows rowScanner) error {
		q := SlowQuery{Source: "system.query_log"}
		if err := rows.Scan(&q.SQL, &q.Database, &q.Calls, &q.TotalMs, &q.AvgMs, &q.MaxMs, &q.Rows, &q.LastSeen); err != nil {
			return err
		}
		out = append(out, q)
		return nil
	})
	if err != nil {
		return nil, model.Errf(model.CodeMonitorUnavailable, "%s\n（%v）", clickhouseSlowHint, err)
	}
	return out, nil
}
