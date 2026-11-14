package ui

import (
	"context"
	"math"
	"testing"
	"time"

	"minidb/internal/app"
	"minidb/internal/model"
)

func TestMonitorChartData(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	pts := []app.MonitorPoint{
		{At: t0, Values: map[string]float64{app.MetricConnections: 3}},
		{At: t0.Add(5 * time.Second), Values: map[string]float64{app.MetricConnections: 4, app.MetricRunning: 1}},
	}
	d := monitorChartData(pts, []string{app.MetricConnections, app.MetricRunning, app.MetricSlow}, func(k string) string { return "n-" + k })
	if len(d.Labels) != 2 || d.Labels[1] != "10:00:05" {
		t.Fatalf("横轴 = %v", d.Labels)
	}
	if len(d.Series) != 2 {
		t.Fatalf("没有数据的指标不该出线: %+v", d.Series)
	}
	if d.Series[0].Name != "n-connections" || d.Series[0].Values[1] != 4 {
		t.Errorf("连接数 = %+v", d.Series[0])
	}
	if d.Series[1].Values[0] != 1 || d.Series[1].Values[1] != 1 { // 开头缺的点与第一个有值的点齐平
		t.Errorf("运行数 = %+v", d.Series[1])
	}
	if lo, hi := d.bounds(); lo != 1 || hi != 4 {
		t.Errorf("bounds = %v %v", lo, hi)
	}
}

func TestMonitorMetricText(t *testing.T) {
	if monitorMetricName(model.Postgres, app.MetricQueries) == monitorMetricName(model.MySQL, app.MetricQueries) {
		t.Error("PostgreSQL 的「查询」应显示为事务")
	}
	if got := monitorMetricValue(app.MetricCacheHit, 99.04); got != "99.0%" {
		t.Errorf("命中率 = %s", got)
	}
	if got := monitorMetricValue(app.MetricBytesIn, 2048); got != "2.0 KB/s" && got != "2 KB/s" && got != "2.00 KB/s" {
		t.Errorf("流量 = %s", got)
	}
	if got := slowMs(250); got != "250 毫秒" {
		t.Errorf("slowMs(250) = %q", got)
	}
	if got := slowMs(1500); got != "1.50 s" {
		t.Errorf("slowMs(1500) = %q", got)
	}
}

func TestSameStatement(t *testing.T) {
	long := "SELECT  *  FROM orders WHERE customer_id = 42 AND status = 'open'"
	cases := []struct {
		info, sql string
		want      bool
	}{
		{long, "select * from orders where customer_id = 42 and status = 'open'", true}, // 大小写、空白不同
		{"SELECT * FROM orders WHERE customer_id = 42 AND st", long, true},              // 进程里的语句被截断
		{"SELECT * FROM orders WHERE customer_id = 7", long, false},                     // 不同的语句
		{"", long, false},              // 空闲连接
		{"SELECT 1", "SELECT 1", true}, // 很短的语句只认完全相同
		{"SELECT 1", "SELECT 1 FROM dual WHERE x = 1 AND y = 2 AND z = 3 AND w = 4", false}, // 很短的不按前缀匹配
	}
	for _, c := range cases {
		if got := sameStatement(c.info, c.sql); got != c.want {
			t.Errorf("sameStatement(%q, %q) = %v，应为 %v", c.info, c.sql, got, c.want)
		}
	}
}

func TestMonitorTabs(t *testing.T) {
	w := func(tp model.DBType) *Workspace { return &Workspace{cfg: model.ConnectionConfig{Type: tp}} }
	cases := map[model.DBType][]int{
		model.MySQL:      {monOverview, monProcess, monSlow},
		model.Postgres:   {monOverview, monProcess, monSlow},
		model.ClickHouse: {monOverview, monProcess, monSlow},
		model.TiDB:       {monProcess},
		model.StarRocks:  {monProcess},
		model.SQLite:     nil,
	}
	for tp, want := range cases {
		got := monitorTabs(w(tp))
		if len(got) != len(want) {
			t.Errorf("%s: tabs = %v，应为 %v", tp, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: tabs = %v，应为 %v", tp, got, want)
			}
		}
	}
}

// fakeMonitorSession 产生一组有起伏的样本，用来检查概览页能画出来。
func fakeMonitorSession(n int) *app.MonitorSession {
	i := 0
	t0 := time.Now().Add(-time.Duration(n) * 5 * time.Second)
	return app.NewMonitorSession(func(ctx context.Context) (app.MonitorSample, error) {
		i++
		f := float64(i)
		return app.MonitorSample{At: t0.Add(time.Duration(i) * 5 * time.Second), Values: map[string]float64{
			app.MetricConnections: 20 + 8*math.Sin(f/3), app.MetricRunning: 2 + math.Abs(3*math.Sin(f/2)),
			app.MetricQueries: f * 900, app.MetricSlow: f * 2,
			app.MetricBytesIn: f * 80000, app.MetricBytesOut: f * 400000, app.MetricCacheHit: 97 + math.Sin(f),
		}}, nil
	}, time.Millisecond, nil)
}

func TestMonitorOverviewRenders(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	sess := fakeMonitorSession(40)
	defer sess.Stop()
	for i := 0; i < 300 && len(sess.View().Points) < 40; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	sess.SetInterval(time.Hour) // 样本够了，不再采样（否则假采样函数到点会超时报错）
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monOverview, sess: sess}}
	e.tt.Frame()
	e.has(monitorMetricName(model.MySQL, app.MetricConnections))
	e.has(monitorPanelTitle(model.MySQL, "traffic"))
	e.shot("60-monitor-overview")
}

func TestMonitorSlowRenders(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monSlow, slow: slowState{loaded: true, sel: 0, items: []app.SlowQuery{
		{SQL: "SELECT o.id, c.name FROM orders o JOIN customers c ON c.id = o.customer_id WHERE o.status = ? ORDER BY o.created_at DESC", Database: "shop", Calls: 1520, TotalMs: 912000, AvgMs: 600, MaxMs: 5400, Rows: 480000, LastSeen: "2026-11-14 19:20:11", Source: "performance_schema"},
		{SQL: "UPDATE stock SET qty = qty - ? WHERE sku = ?", Database: "shop", Calls: 90000, TotalMs: 45000, AvgMs: 0.5, MaxMs: 120, Rows: -1, LastSeen: "2026-11-14 19:21:00", Source: "performance_schema"},
	}}}}
	e.tt.Frame()
	e.has("performance_schema")
	e.shot("61-monitor-slow")
}

func TestMonitorSlowEmptyAndError(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monSlow, slow: slowState{loaded: true, sel: -1}}}
	e.has("没有记录到慢查询")
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monSlow, slow: slowState{loaded: true, sel: -1, err: "请开启 performance_schema"}}}
	e.has("慢查询数据源不可用")
	e.has("请开启 performance_schema")
}

// 中间缺的点沿用上一个值（计数器被清零时那一点没有速率）。
func TestMonitorChartDataGaps(t *testing.T) {
	t0 := time.Now()
	pts := []app.MonitorPoint{
		{At: t0, Values: map[string]float64{}},
		{At: t0.Add(time.Second), Values: map[string]float64{app.MetricQueries: 5}},
		{At: t0.Add(2 * time.Second), Values: map[string]float64{}},
		{At: t0.Add(3 * time.Second), Values: map[string]float64{app.MetricQueries: 9}},
	}
	d := monitorChartData(pts, []string{app.MetricQueries}, func(string) string { return "q" })
	want := []float64{5, 5, 5, 9}
	for i, v := range want {
		if d.Series[0].Values[i] != v {
			t.Fatalf("values = %v，应为 %v", d.Series[0].Values, want)
		}
	}
}
