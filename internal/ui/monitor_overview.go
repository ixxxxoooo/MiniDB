package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// 监控概览：一组指标卡，每张卡带一条实时走势；数据来自 app.MonitorSession 的环形缓冲。

// monitorChartData 把采样点整理成折线数据：横轴是时间，每个键一条线；没有任何数据的键不出线。
func monitorChartData(points []app.MonitorPoint, keys []string, name func(key string) string) chartData {
	d := chartData{Labels: make([]string, len(points))}
	for i, pt := range points {
		d.Labels[i] = pt.At.Format("15:04:05")
	}
	for _, k := range keys {
		vals := make([]float64, len(points))
		first := -1
		for i, pt := range points {
			if v, ok := pt.Values[k]; ok {
				if first < 0 {
					first = i
				}
				vals[i] = v
			} else if first >= 0 {
				vals[i] = vals[i-1] // 中间缺的点（计数器被清零）沿用上一个值
			}
		}
		if first < 0 {
			continue
		}
		for i := 0; i < first; i++ {
			vals[i] = vals[first] // 开头缺的点（第一次采样还没有速率）与第一个有值的点齐平，不画成 0
		}
		d.Series = append(d.Series, chartSeries{Name: name(k), Values: vals})
	}
	return d
}

// monitorMetricName 是指标在界面上的名字。PostgreSQL 的「查询」其实是事务数。
func monitorMetricName(t model.DBType, key string) string {
	switch key {
	case app.MetricConnections:
		return locales.MonitorConnections()
	case app.MetricRunning:
		return locales.MonitorRunning()
	case app.MetricQueries:
		if app.CapsOf(t).QueriesAreTransactions {
			return locales.MonitorTransactions()
		}
		return locales.MonitorQueries()
	case app.MetricSlow:
		return locales.MonitorSlow()
	case monitorNet:
		return locales.MonitorNetwork()
	case app.MetricBytesIn:
		return locales.MonitorBytesIn()
	case app.MetricBytesOut:
		return locales.MonitorBytesOut()
	}
	return locales.MonitorCacheHit()
}

// monitorMetricValue 把指标值写成带单位的短文本。
func monitorMetricValue(key string, v float64) string {
	switch key {
	case app.MetricBytesIn, app.MetricBytesOut, monitorNet:
		return humanBytes(int64(v)) + "/s"
	case app.MetricCacheHit:
		return fmt.Sprintf("%.1f%%", v)
	}
	return formatNumber(v)
}

// monitorMetricParts 把指标值拆成「数值」和「单位」，卡片里数值大、单位小。
func monitorMetricParts(t model.DBType, key string, v float64) (num, unit string) {
	switch key {
	case app.MetricBytesIn, app.MetricBytesOut, monitorNet:
		s := humanBytes(int64(v)) + "/s"
		if i := strings.LastIndex(s, " "); i > 0 {
			return s[:i], s[i+1:]
		}
		return s, ""
	case app.MetricCacheHit:
		return fmt.Sprintf("%.1f", v), "%"
	case app.MetricConnections, app.MetricRunning:
		return fmt.Sprintf("%.0f", v), ""
	case app.MetricQueries:
		if !app.CapsOf(t).QueriesAreTransactions {
			return formatNumber(v), "qps"
		}
	}
	return formatNumber(v), ""
}

// monitorDelta 是最后一点相对前一点的涨跌百分比；前一点是 0 或只有一个点时 ok 为 false。
func monitorDelta(vals []float64) (pct float64, ok bool) {
	if len(vals) < 2 || vals[len(vals)-2] == 0 {
		return 0, false
	}
	prev := vals[len(vals)-2]
	return (vals[len(vals)-1] - prev) / prev * 100, true
}

// monitorOverview 画概览：三列指标卡，每张卡是名称、涨跌、当前值，下面一条实时走势。
func (s *Shell) monitorOverview(c *ui.Context, p tokens) {
	m := &s.proc.mon
	t := s.proc.w.cfg.Type
	if m.sess == nil {
		return
	}
	v := m.sess.View()
	if v.Err != "" {
		ui.Text(c, v.Err).TextColor(p.danger).FontSize(fontCtl).Selectable().FillWidth().Shrink(0)
	}
	if len(v.Points) == 0 {
		if v.Err == "" {
			ui.Text(c, locales.MonitorWaiting()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		}
		return
	}
	points := withNetwork(v.Points)
	last := points[len(points)-1]
	var keys []string
	for _, def := range app.MetricDefs {
		if def.Key == app.MetricBytesOut {
			continue // 收到和发出合并成一张「网络流量」卡（设计稿 A）
		}
		key := def.Key
		if key == app.MetricBytesIn {
			key = monitorNet
		}
		if _, ok := last.Values[key]; ok {
			keys = append(keys, key)
		}
	}
	for i := 0; i < len(keys); i += monitorCols {
		ui.Row(c.Key(fmt.Sprint("row", i))).Gap(spaceMD).FillWidth().Shrink(0).Children(func() {
			for j := 0; j < monitorCols; j++ {
				if i+j >= len(keys) {
					ui.Box(c).Grow(1).Basis(0).MinWidth(0)
					continue
				}
				s.monitorCard(c, p, t, keys[i+j], points, last.Values[keys[i+j]])
			}
		})
	}
	if len(last.Values) > 0 && !hasRates(last) {
		ui.Text(c, locales.MonitorWaitingRate()).FontSize(fontCtl).TextColor(p.fgMuted)
	}
}

// monitorNet 是合并后的「网络流量」指标键：每秒收到 + 每秒发出。
const monitorNet = "net"

// withNetwork 返回加上合并指标 monitorNet 的采样点：收到、发出至少有一个时，值是两者之和。
// 不改动传入的点（它们属于监控会话的环形缓冲），需要加值的点复制一份。
func withNetwork(points []app.MonitorPoint) []app.MonitorPoint {
	out := make([]app.MonitorPoint, len(points))
	for i, pt := range points {
		in, okIn := pt.Values[app.MetricBytesIn]
		o, okOut := pt.Values[app.MetricBytesOut]
		if okIn || okOut {
			vals := make(map[string]float64, len(pt.Values)+1)
			for k, v := range pt.Values {
				vals[k] = v
			}
			vals[monitorNet] = in + o
			pt.Values = vals
		}
		out[i] = pt
	}
	return out
}

// monitorCols 是概览每行放几张指标卡。
const monitorCols = 3

// monitorCard 画一张指标卡：左上名称、右上涨跌芯片，下面是当前值（数值大、单位小）和走势。
func (s *Shell) monitorCard(c *ui.Context, p tokens, t model.DBType, key string, points []app.MonitorPoint, cur float64) {
	d := monitorChartData(points, []string{key}, func(string) string { return "" })
	var vals []float64
	if len(d.Series) > 0 {
		vals = d.Series[0].Values
	}
	num, unit := monitorMetricParts(t, key, cur)
	dlgBox(c.Key("card-"+key), p).Grow(1).Basis(0).MinWidth(0).Background(p.surface).Border(1, p.border).Radius(radiusPanel).Padding(spaceMD, spaceMD+2).Gap(2).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, monitorMetricName(t, key)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			ui.Spacer(c)
			if pct, ok := monitorDelta(vals); ok {
				arrow := "▲"
				if pct < 0 {
					arrow, pct = "▼", -pct
				}
				chip(c, chipOK, fmt.Sprintf("%s %.1f%%", arrow, pct), false)
			}
		})
		ui.Row(c).Gap(spaceXS).AlignItems(ui.End).Shrink(0).Children(func() {
			ui.Text(c, num).FontSize(fontHeading).FontWeight(600).SingleLine()
			if unit != "" {
				ui.Text(c, unit).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Margin(0, 0, 2, 0)
			}
		})
		ui.Box(c.Key("spark-" + key)).FillWidth().Height(sizeSpark).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			paintSpark(pt, r, vals, p.accent, p.accentSoft)
		})
	})
}

// hasRates 报告这个点是否已经带有计数器换算出的速率（第一次采样没有）。
func hasRates(pt app.MonitorPoint) bool {
	for _, def := range app.MetricDefs {
		if _, ok := pt.Values[def.Key]; ok && def.Counter {
			return true
		}
	}
	return false
}
