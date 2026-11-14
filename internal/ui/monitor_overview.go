package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// 监控概览：顶部是各指标的当前值，下面是几张实时折线。
// 折线复用查询结果图表的画法（paintChart），数据来自 app.MonitorSession 的环形缓冲。

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
	case app.MetricBytesIn, app.MetricBytesOut:
		return humanBytes(int64(v)) + "/s"
	case app.MetricCacheHit:
		return fmt.Sprintf("%.1f%%", v)
	}
	return formatNumber(v)
}

// monitorPanels 是概览里的折线分组，每组画在一张图里。
var monitorPanels = []struct {
	id   string
	keys []string
}{
	{"sessions", []string{app.MetricConnections, app.MetricRunning}},
	{"queries", []string{app.MetricQueries, app.MetricSlow}},
	{"traffic", []string{app.MetricBytesIn, app.MetricBytesOut}},
	{"cache", []string{app.MetricCacheHit}},
}

// monitorPanelTitle 是一张折线图的标题。
func monitorPanelTitle(t model.DBType, id string) string {
	switch id {
	case "sessions":
		return locales.MonitorSessions()
	case "queries":
		return monitorMetricName(t, app.MetricQueries)
	case "traffic":
		return locales.MonitorTraffic()
	}
	return locales.MonitorCacheHit()
}

func (s *Shell) monitorOverview(c *ui.Context, p tokens) {
	m := &s.proc.mon
	t := s.proc.w.cfg.Type
	if m.sess == nil {
		return
	}
	v := m.sess.View()
	if v.Err != "" {
		ui.Text(c, v.Err).TextColor(p.danger).FontSize(font2XS).Selectable().FillWidth().Shrink(0)
	}
	if len(v.Points) == 0 {
		if v.Err == "" {
			ui.Text(c, locales.MonitorWaiting()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		}
		return
	}
	last := v.Points[len(v.Points)-1]

	// 指标卡：这一刻上报了哪些就显示哪些
	ui.Row(c).Gap(8).FillWidth().Shrink(0).Children(func() {
		for _, def := range app.MetricDefs {
			val, ok := last.Values[def.Key]
			if !ok {
				continue
			}
			def, val := def, val
			dlgBox(c, p).Grow(1).MinWidth(0).Padding(8, 12).Gap(2).Children(func() {
				ui.Text(c.Key("l-"+def.Key), monitorMetricName(t, def.Key)).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
				ui.Text(c.Key("v-"+def.Key), monitorMetricValue(def.Key, val)).FontSize(18).FontWeight(600).SingleLine()
			})
		}
	})

	// 折线：数据只在有新点时重算
	if m.charts == nil || !m.chartAt.Equal(last.At) {
		m.charts = map[string]chartData{}
		for _, pn := range monitorPanels {
			if d := monitorChartData(v.Points, pn.keys, func(k string) string { return monitorMetricName(t, k) }); len(d.Series) > 0 {
				m.charts[pn.id] = d
			}
		}
		m.chartAt = last.At
	}
	th := chartTheme{grid: p.borderSubtle, axis: p.border, text: p.fgSecondary, bg: p.surface}
	var shown []string
	for _, pn := range monitorPanels {
		if _, ok := m.charts[pn.id]; ok {
			shown = append(shown, pn.id)
		}
	}
	span := last.At.Sub(v.Points[0].At)
	for i := 0; i < len(shown); i += 2 {
		ui.Row(c.Key(fmt.Sprint("row", i))).Gap(8).Grow(1).FillWidth().MinHeight(110).AlignItems(ui.Stretch).Children(func() {
			for _, id := range shown[i:min(i+2, len(shown))] {
				id, d := id, m.charts[id]
				dlgBox(c.Key("box-"+id), p).Grow(1).MinWidth(0).FillHeight().Children(func() {
					ui.Row(c).Gap(8).Padding(6, 10, 0, 10).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
						ui.Text(c, monitorPanelTitle(t, id)).FontSize(font2XS).FontWeight(600).SingleLine()
						ui.Spacer(c)
						ui.Text(c, locales.MonitorWindow(max(1, int(span.Minutes()+0.5)))).FontSize(font2XS - 1).TextColor(p.fgMuted)
					})
					ui.Box(c.Key("chart-" + id)).Grow(1).FillWidth().MinHeight(70).Draw(func(pt *ui.Painter, rect ui.Rect) {
						paintChart(pt, rect, chartLine, d, th, "")
					})
				})
			}
		})
	}
	if len(last.Values) > 0 && !hasRates(last) {
		ui.Text(c, locales.MonitorWaitingRate()).FontSize(font2XS).TextColor(p.fgMuted)
	}
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
