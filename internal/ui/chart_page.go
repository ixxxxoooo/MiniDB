package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 结果图表整页（设计稿 A「图表」）：点结果区的「图表」后，查询标签的整个页面换成它——
// 顶部条（← 返回查询、柱状 / 折线 / 饼图），左边是大图（标题、副标题、右侧图例、图表），
// 右边一条配置栏（X 轴、Y 轴、显示数值、显示网格）。

const chartMaxH = 440 // 图表主体（标题 + 图）的最大高度，窗口再高也不继续拉伸，垂直居中

// chartTarget 返回要画图表的那一页结果；当前不在图表页或结果不能画图时返回 nil。
func (q *queryTab) chartTarget() *queryResult {
	if q.page != pageChart || q.shown < 0 || q.shown >= len(q.results) {
		return nil
	}
	r := q.results[q.shown]
	if r.res.Error != "" || len(r.res.Columns) == 0 {
		return nil
	}
	return r
}

// chartPage 画图表整页。
func (q *queryTab) chartPage(c *ui.Context, p tokens, r *queryResult) {
	r.ensureChart()
	st := &r.chart
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(spaceSM).Padding(0, spaceSM+2).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Children(func() {
			if ghostBtn(c.Key("chart-back"), icBack, locales.ChartBack()).Clicked() {
				q.page = pageData
			}
			vline(c, p.border, 14)
			kind := chartKindIndex(st.spec.Kind)
			if segmented(c.Key("ckind"), &kind, chartKindLabels()...) {
				st.spec.Kind = chartKinds[kind]
			}
		})
	})
	hairline(c, p.border)
	sig := fmt.Sprint(st.spec.X, st.spec.Ys)
	if st.sig != sig {
		st.sig, st.data = sig, buildChart(r.res.Rows, st.spec)
	}
	ui.Row(c).Grow(1).FillWidth().Children(func() {
		q.chartCanvas(c, p, r)
		q.chartPane(c, p, r)
	})
}

// chartCanvas 是左边的大图：标题行（标题 + 副标题，图例靠右）和图表本身。
func (q *queryTab) chartCanvas(c *ui.Context, p tokens, r *queryResult) {
	st := &r.chart
	th := newChartTheme(p)
	th.hideValues, th.hideGrid, th.noLegend = st.hideVals, st.hideGrid, true
	ui.Column(c.Key("chart-canvas")).Grow(1).MinWidth(0).FillHeight().Padding(spaceXL, spaceXL+4).Center().Children(func() {
		if len(st.spec.Ys) == 0 || len(st.data.Labels) == 0 {
			ui.Text(c, locales.ChartNoData()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
			return
		}
		ui.Column(c.Key("chart-body")).FillWidth().Grow(1).MaxHeight(chartMaxH).Gap(spaceXS).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Start).Shrink(0).Gap(spaceLG).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, strings.Join(st.spec.Ys, " · ")).FontSize(fontTitle).FontWeight(600).TextColor(p.fg).SingleLine()
					sub := locales.ChartRowsSub(len(r.res.Rows))
					if st.data.Thin {
						sub += " · " + locales.ChartThinned(maxChartPoints)
					}
					ui.Text(c, sub).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
				})
				ui.Spacer(c)
				q.chartLegend(c, p, th, st)
			})
			kind, data, other := st.spec.Kind, st.data, locales.ChartOther()
			ui.Box(c.Key("chart")).Grow(1).FillWidth().MinHeight(160).Draw(func(pt *ui.Painter, rect ui.Rect) {
				paintChart(pt, rect, kind, data, th, other)
			})
		})
	})
}

// chartLegend 画图例：单系列的柱状图按类别（和柱子颜色对应），多系列按系列名；饼图自己带图例，这里不画。
func (q *queryTab) chartLegend(c *ui.Context, p tokens, th chartTheme, st *chartState) {
	if st.spec.Kind == chartPie {
		return
	}
	names := st.data.Labels
	if len(st.data.Series) > 1 || st.spec.Kind == chartLine {
		names = nil
		for _, s := range st.data.Series {
			names = append(names, s.Name)
		}
	}
	ui.Row(c.Key("chart-legend")).Gap(spaceLG).AlignItems(ui.Center).Shrink(1).Children(func() {
		for i, n := range names {
			if i >= 6 {
				break
			}
			ui.Row(c.Key("lg" + strconv.Itoa(i))).Gap(spaceXS + 1).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(sizeDot).Height(sizeDot).Background(th.color(i)).Radius(1)
				ui.Text(c, clipLabel(n, 14)).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine()
			})
		}
	})
}

// chartPane 是右边的配置栏。
func (q *queryTab) chartPane(c *ui.Context, p tokens, r *queryResult) {
	st := &r.chart
	var names []string
	for _, col := range r.res.Columns {
		names = append(names, col.Name)
	}
	nums := numericColumns(r.res.Columns, r.res.Rows)
	label := func(s string) {
		ui.Text(c, s).FontSize(fontCaption).TextColor(p.fgMuted).Margin(spaceSM, 0, 0, 0)
	}
	ui.Column(c.Key("chart-pane")).Width(sizeSidePane).Shrink(0).FillHeight().Padding(spaceLG).Gap(spaceXS).
		Background(p.surfaceSecondary).Children(func() {
		label(locales.ChartX())
		ui.Select(c.Key("cx"), &st.spec.X, names).FillWidth().Height(sizeBtn)
		label(locales.ChartY())
		ui.TokenField(c.Key("cy"), &st.spec.Ys, nums).FillWidth()
		chartSwitch(c, p, locales.ChartShowValues(), &st.hideVals)
		chartSwitch(c, p, locales.ChartShowGrid(), &st.hideGrid)
	})
}

// chartSwitch 是配置栏里「文字 + 开关」一行；hide 取反存放，开关打开就是不隐藏。
func chartSwitch(c *ui.Context, p tokens, text string, hide *bool) {
	on := !*hide
	ui.Row(c.Key("sw-"+text)).FillWidth().AlignItems(ui.Center).Margin(spaceSM, 0, 0, 0).Children(func() {
		ui.Text(c, text).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine()
		ui.Spacer(c)
		if ui.Switch(c, &on).Changed() {
			*hide = !on
		}
	})
}
