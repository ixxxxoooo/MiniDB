package database

import (
	"context"
	"math"
	"testing"
	"time"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMonitorPointOf(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prev := &MonitorSample{At: t0, Values: map[string]float64{
		MetricConnections: 5, MetricQueries: 1000, MetricBytesIn: 2000, MetricSlow: 7,
	}}
	cur := MonitorSample{At: t0.Add(5 * time.Second), Values: map[string]float64{
		MetricConnections: 8, MetricQueries: 1500, MetricBytesIn: 12000, MetricSlow: 3, // 慢查询计数变小：服务重启
	}}
	pt := MonitorPointOf(prev, cur)
	if pt.Values[MetricConnections] != 8 {
		t.Errorf("仪表应取当前值: %v", pt.Values)
	}
	if !near(pt.Values[MetricQueries], 100) || !near(pt.Values[MetricBytesIn], 2000) {
		t.Errorf("速率不对: %v", pt.Values)
	}
	if _, ok := pt.Values[MetricSlow]; ok {
		t.Errorf("计数器变小时不该出点: %v", pt.Values)
	}
	if _, ok := pt.Values[MetricBytesOut]; ok {
		t.Errorf("没上报的指标不该出现: %v", pt.Values)
	}

	first := MonitorPointOf(nil, cur)
	if _, ok := first.Values[MetricQueries]; ok || first.Values[MetricConnections] != 8 {
		t.Errorf("第一次采样只有仪表: %v", first.Values)
	}
	same := MonitorPointOf(prev, MonitorSample{At: t0, Values: cur.Values})
	if _, ok := same.Values[MetricQueries]; ok {
		t.Errorf("时间没有前进不能除以零: %v", same.Values)
	}
}

func TestMySQLSampleOf(t *testing.T) {
	v := mysqlSampleOf(map[string]string{
		"Threads_connected": "12", "Threads_running": "3", "Questions": "9001", "Slow_queries": "x", "Bytes_sent": "1e3",
	})
	if v[MetricConnections] != 12 || v[MetricRunning] != 2 || v[MetricQueries] != 9001 || v[MetricBytesOut] != 1000 {
		t.Errorf("sample = %v", v)
	}
	if _, ok := v[MetricSlow]; ok {
		t.Errorf("无法解析的值应跳过: %v", v)
	}
	if v := mysqlSampleOf(map[string]string{"Threads_running": "0"}); v[MetricRunning] != 0 {
		t.Errorf("运行数不能变成负数: %v", v)
	}
}

func TestMonitorCaps(t *testing.T) {
	want := map[DBType]bool{MySQL: true, MariaDB: true, Postgres: true, SQLServer: true, ClickHouse: true, TiDB: false, StarRocks: false, SQLite: false}
	for tp, ok := range want {
		if SupportsMonitor(tp) != ok {
			t.Errorf("%s: Monitor = %v，应为 %v", tp, SupportsMonitor(tp), ok)
		}
		if !ok {
			if _, err := GetMonitorSample(context.Background(), nil, tp); err == nil {
				t.Errorf("%s 不支持监控，应返回错误", tp)
			}
		}
	}
	// 声明了 Monitor 的方言必须真的实现接口
	for _, tp := range AllDBTypes() {
		if SupportsMonitor(tp) {
			if _, err := monitorOf(tp); err != nil {
				t.Errorf("%s: %v", tp, err)
			}
		}
	}
}
