package database

import (
	"context"
	"sort"
	"time"

	"minidb/internal/model"
)

// 监控：服务端指标采样与慢查询。方言通过可选接口 Monitor 提供（Caps().Monitor 为 true）。
// 指标分两类：仪表（连接数、运行中的语句数，直接取值）和计数器（累计的查询数、流量，要用相邻两次采样的差算速率）。

// 指标的键。方言只上报自己有的那些，界面按有无决定画哪些卡片和折线。
const (
	MetricConnections = "connections" // 仪表：当前连接数
	MetricRunning     = "running"     // 仪表：正在执行语句的会话数
	MetricCacheHit    = "cacheHit"    // 仪表：缓存命中率，0–100
	MetricQueries     = "queries"     // 计数器：累计语句数（PostgreSQL 是累计事务数）
	MetricSlow        = "slow"        // 计数器：累计慢查询数
	MetricBytesIn     = "bytesIn"     // 计数器：累计收到的字节
	MetricBytesOut    = "bytesOut"    // 计数器：累计发出的字节
)

// MetricDef 描述一个指标怎么算、怎么显示。
type MetricDef struct {
	Key     string
	Counter bool   // 为 true 时取相邻采样的差再除以时间，得到每秒速率
	Unit    string // count、bytes、percent
}

// MetricDefs 是全部指标的定义，顺序即界面上卡片的顺序。
var MetricDefs = []MetricDef{
	{MetricConnections, false, "count"},
	{MetricRunning, false, "count"},
	{MetricQueries, true, "count"},
	{MetricSlow, true, "count"},
	{MetricBytesIn, true, "bytes"},
	{MetricBytesOut, true, "bytes"},
	{MetricCacheHit, false, "percent"},
}

// MonitorSample 是某一刻的一次采样：仪表直接是当前值，计数器是累计值。
type MonitorSample struct {
	At     time.Time          `json:"at"`
	Values map[string]float64 `json:"values"`
}

// SlowQuery 是一条慢查询（或一类语句的汇总）。时间单位是毫秒。
type SlowQuery struct {
	SQL      string  `json:"sql"`
	Database string  `json:"database"`
	Calls    int64   `json:"calls"`
	TotalMs  float64 `json:"totalMs"`
	AvgMs    float64 `json:"avgMs"`
	MaxMs    float64 `json:"maxMs"`
	Rows     int64   `json:"rows"`     // 扫描或返回的行数，取不到是 -1
	LastSeen string  `json:"lastSeen"` // 最近一次执行的时间，取不到是空串
	Source   string  `json:"source"`   // 数据来源，如 performance_schema
}

// Monitor 由能提供监控数据的方言额外实现（Caps().Monitor 为 true）。全部是只读查询。
type Monitor interface {
	// Sample 取一次指标采样。
	Sample(ctx context.Context, db Executor) (MonitorSample, error)
	// SlowQueries 返回最慢的至多 limit 条，按平均耗时从高到低。
	// 数据源不可用时返回带开启办法的 CodeMonitorUnavailable 错误。
	SlowQueries(ctx context.Context, db Executor, limit int) ([]SlowQuery, error)
}

func monitorOf(t DBType) (Monitor, error) {
	if d, ok := dialects[t]; ok && d.Caps().Monitor {
		if m, ok := d.(Monitor); ok {
			return m, nil
		}
	}
	return nil, model.Errf(model.CodeMonitorUnsupported, "%s 不支持监控", t)
}

// SupportsMonitor 报告类型是否有监控概览与慢查询。
func SupportsMonitor(t DBType) bool { return CapsOf(t).Monitor }

// GetMonitorSample 取一次指标采样。
func GetMonitorSample(ctx context.Context, db Executor, t DBType) (MonitorSample, error) {
	m, err := monitorOf(t)
	if err != nil {
		return MonitorSample{}, err
	}
	s, err := m.Sample(ctx, db)
	if s.At.IsZero() {
		s.At = time.Now()
	}
	return s, err
}

// GetSlowQueries 返回最慢的至多 limit 条语句。
func GetSlowQueries(ctx context.Context, db Executor, t DBType, limit int) ([]SlowQuery, error) {
	m, err := monitorOf(t)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	out, err := m.SlowQueries(ctx, db, limit)
	sort.SliceStable(out, func(i, j int) bool { return out[i].AvgMs > out[j].AvgMs })
	return out, err
}

// MonitorPoint 是画图用的一个点：仪表是当前值，计数器已经换算成每秒速率。
type MonitorPoint struct {
	At     time.Time          `json:"at"`
	Values map[string]float64 `json:"values"`
}

// MonitorPointOf 由相邻两次采样算出一个点；prev 为空（第一次采样）时只有仪表。
// 计数器变小（服务重启或统计被清零）时这一项跳过，不画负数。
func MonitorPointOf(prev *MonitorSample, cur MonitorSample) MonitorPoint {
	pt := MonitorPoint{At: cur.At, Values: map[string]float64{}}
	dt := 0.0
	if prev != nil {
		dt = cur.At.Sub(prev.At).Seconds()
	}
	for _, def := range MetricDefs {
		v, ok := cur.Values[def.Key]
		if !ok {
			continue
		}
		if !def.Counter {
			pt.Values[def.Key] = v
			continue
		}
		if prev == nil || dt <= 0 {
			continue
		}
		if p, ok := prev.Values[def.Key]; ok && v >= p {
			pt.Values[def.Key] = (v - p) / dt
		}
	}
	return pt
}
