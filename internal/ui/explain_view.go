package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// planNum 把行数、代价格式成简短的数字；0 表示数据库没给，留空。
func planNum(f float64) string {
	switch {
	case f <= 0:
		return ""
	case f >= 1e9:
		return strconv.FormatFloat(f/1e9, 'f', 1, 64) + "G"
	case f >= 1e6:
		return strconv.FormatFloat(f/1e6, 'f', 1, 64) + "M"
	case f >= 1e4:
		return strconv.FormatFloat(f/1e3, 'f', 1, 64) + "K"
	case f == float64(int64(f)):
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// planMs 把实际耗时（毫秒）格式成 "1.23 ms"；0 留空。
func planMs(ms float64) string {
	if ms <= 0 {
		return ""
	}
	if ms >= 1000 {
		return strconv.FormatFloat(ms/1000, 'f', 2, 64) + " s"
	}
	return strconv.FormatFloat(ms, 'f', 2, 64) + " ms"
}

// pageTabs 画结果区底栏的页签：数据、图表、消息，有执行计划时再多一个「执行计划」。
func (q *queryTab) pageTabs(c *ui.Context, p tokens) {
	labels := []string{locales.QueryPageData(), locales.QueryPageChart(), locales.QueryMessages()}
	if q.plan != nil {
		labels = append(labels, locales.QueryPagePlan())
	}
	if i := pillTabs(c, p, "pages", labels, q.page, false); i >= 0 {
		q.page = i
	}
}

// planView 是结果区的「执行计划」页：顶部是语句和 ANALYZE 开关，中间是计划树或原文，底部是状态栏。
func (q *queryTab) planView(c *ui.Context, p tokens) {
	ps := q.plan
	ui.Column(c.Key("plan")).Fill().Children(func() {
		ui.Row(c).Gap(8).Padding(5, 12).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
			ui.Text(c, firstLines(ps.stmt, 1)).Font(mono).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine().Grow(1).MinWidth(0)
			if app.CanExplainAnalyze(q.w.cfg.Type) {
				ui.Checkbox(c.Key("plan-analyze"), &q.planAnalyze, locales.ExplainAnalyze())
			}
			if actBtn(c.Key("plan-rerun"), icRefresh, locales.ExplainRerun()).Disabled(ps.loading).Clicked() {
				q.explainStmt(ps.stmt, q.planAnalyze)
			}
		})
		hairline(c, p.borderSubtle)
		switch {
		case ps.loading:
			ui.Row(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, locales.ExplainRunning()).FontSize(font2XS).TextColor(p.fgMuted)
			})
		case ps.err != "":
			q.banner(c, p, kindError, ps.err, nil)
		default:
			q.planBody(c, p, ps)
		}
		hairline(c, p.border)
		q.planStatusBar(c, p, ps)
	})
}

// planBody 是计划树 / 原文两个页签。
func (q *queryTab) planBody(c *ui.Context, p tokens, ps *planState) {
	res := ps.res
	if res.Note != "" {
		ui.Row(c).Gap(6).Padding(4, 12).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
			ui.Icon(c, icInfo).FontSize(12).TextColor(p.warning)
			ui.Text(c, res.Note).FontSize(font2XS).TextColor(p.fgSecondary)
		})
	}
	if len(res.Roots) > 0 {
		ui.Row(c).Padding(2, 12).FillWidth().Shrink(0).Children(func() {
			ui.Segmented(c.Key("plan-seg"), &ps.tab, locales.ExplainTree(), locales.ExplainRaw())
		})
	}
	if ps.tab == 1 || len(res.Roots) == 0 {
		scrollbars(c, ui.ScrollBoth(c.Key("plan-raw")).Grow(1).FillWidth()).Children(func() {
			ui.Text(c, res.Raw).Font(mono).FontSize(font2XS).Selectable().Padding(8, 12)
		})
		return
	}
	q.planTree(c, p, ps)
}

// planColumn 是树表的一列：表头、宽度（0 为撑满）、取值。
type planColumn struct {
	title string
	width float32
	cell  func(n *model.PlanNode) string
}

func (q *queryTab) planTree(c *ui.Context, p tokens, ps *planState) {
	res := ps.res
	cols := []planColumn{
		{locales.ExplainColObject(), 160, func(n *model.PlanNode) string { return n.Object }},
		{locales.ExplainColRows(), 70, func(n *model.PlanNode) string { return planNum(n.Rows) }},
		{locales.ExplainColCost(), 70, func(n *model.PlanNode) string { return planNum(n.Cost) }},
	}
	if res.Analyzed {
		cols = append(cols, planColumn{locales.ExplainColTime(), 80, func(n *model.PlanNode) string { return planMs(n.ActualMs) }})
	}
	cols = append(cols, planColumn{locales.ExplainColExtra(), 0, func(n *model.PlanNode) string { return n.Extra }})
	const opW = 260
	scrollbars(c, ui.ScrollBoth(c.Key("plan-tree")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).MinWidth(0).Children(func() {
			ui.Row(c.Key("ph")).Gap(8).Padding(3, 12).Background(p.surfaceSecondary).Children(func() {
				ui.Text(c, locales.ExplainColOp()).Width(opW).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
				for i, col := range cols {
					t := ui.Text(c.Key(fmt.Sprint("h", i)), col.title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
					planCellSize(t, col.width)
				}
			})
			for i, pr := range res.Flat() {
				hot := pr.Node == ps.hot
				row := ui.Row(c.Key(fmt.Sprint("pr", i))).Gap(8).Padding(3, 12).AlignItems(ui.Center)
				if hot {
					row.Background(p.danger.Alpha(0.12))
				} else if row.Hovered() {
					row.Background(p.hover)
				}
				row.Children(func() {
					ui.Row(c.Key("op")).Width(opW).Shrink(0).AlignItems(ui.Center).Children(func() {
						ui.Box(c.Key("ind")).Width(float32(pr.Depth) * 14).Height(1).Shrink(0)
						col := p.fg
						if hot {
							col = p.danger
						}
						ui.Text(c, pr.Node.Op).FontSize(font2XS).FontWeight(500).TextColor(col).SingleLine()
					})
					for j, col := range cols {
						t := ui.Text(c.Key(fmt.Sprint("c", j)), col.cell(pr.Node)).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine()
						if col.title == locales.ExplainColExtra() || col.title == locales.ExplainColObject() {
							t.Font(mono)
						}
						planCellSize(t, col.width)
					}
				})
			}
		})
	})
}

// planCellSize 给表格单元定宽；宽度为 0 的最后一列撑满剩余空间。
func planCellSize(t ui.Element, w float32) {
	if w > 0 {
		t.Width(w).Shrink(0)
		return
	}
	t.Grow(1).MinWidth(200)
}

// planStatusBar 是底栏：节点数、耗时、热点说明，右边是页签和关闭按钮。
func (q *queryTab) planStatusBar(c *ui.Context, p tokens, ps *planState) {
	ui.Row(c).Gap(12).Padding(4, 12).AlignItems(ui.Center).FillWidth().Background(p.surfaceSecondary).Shrink(0).Children(func() {
		if ps.res != nil {
			ui.Text(c, locales.ExplainNodes(len(ps.res.Flat()))+" · "+fmtDur(ps.dur)).FontSize(12).TextColor(p.fgMuted)
			if ps.res.Analyzed {
				ui.Text(c, "ANALYZE").FontSize(font2XS - 1).FontWeight(600).TextColor(p.warning)
			}
			if ps.hot != nil {
				ui.Text(c, "■ "+strings.TrimSpace(locales.ExplainHot())).FontSize(font2XS).TextColor(p.danger)
			}
		}
		ui.Spacer(c)
		if len(q.results) > 0 {
			q.pageTabs(c, p)
		}
		if tipBtn(c.Key("plan-close"), icX, locales.ExplainClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			q.closePlan()
		}
	})
}
