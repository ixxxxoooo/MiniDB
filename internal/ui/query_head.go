package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 结果区顶部的 34px 标题栏（设计稿 15）：左边是下划线标签「结果 1 / 结果 2 / 消息 / 执行计划」，
// 右边是「数据 / 图表」分段开关（仅在当前结果能出图时出现）。

// resultHead 画结果区标题栏并处理点击：点结果标签切到那个结果的数据页，点「消息」「执行计划」切到对应页。
func (q *queryTab) resultHead(c *ui.Context, p tokens) {
	n := len(q.results)
	labels := make([]string, 0, n+2)
	counts := make([]int, 0, n+2)
	for i, r := range q.results {
		labels = append(labels, r.label(i))
		if r.res.Error == "" && len(r.res.Columns) > 0 {
			counts = append(counts, len(r.res.Rows))
		} else {
			counts = append(counts, -1)
		}
	}
	msgAt, planAt := len(labels), -1
	labels, counts = append(labels, locales.QueryMessages()), append(counts, n)
	if q.plan != nil || app.CanExplain(q.w.cfg.Type) { // 设计稿：执行计划标签一直在，还没有计划时点它就去生成
		planAt = len(labels)
		labels, counts = append(labels, locales.QueryPagePlan()), append(counts, -1)
	}
	cur := q.shown
	switch q.page {
	case pageMessages:
		cur = msgAt
	case pagePlan:
		cur = planAt
	}
	charting := false
	if q.shown >= 0 && q.shown < n {
		r := q.results[q.shown]
		charting = r.res.Error == "" && len(r.res.Columns) > 0
	}
	ui.Row(c.Key("res-head")).FillWidth().Height(sizeBar).Shrink(0).Padding(0, spaceSM+2).AlignItems(ui.Center).Background(p.surface).Children(func() {
		switch i := ulStrip(c, p, "results", labels, counts, cur, sizeBar, false); {
		case i < 0:
		case i == msgAt:
			q.page = pageMessages
		case i == planAt:
			if q.plan == nil {
				q.explain()
			} else {
				q.page = pagePlan
			}
		default:
			q.shown = i
			if q.page == pageMessages || q.page == pagePlan {
				q.page = pageData
			}
		}
		ui.Spacer(c)
		if charting && (q.page == pageData || q.page == pageChart) {
			sel := 0
			if q.page == pageChart {
				sel = 1
			}
			if segmented(c, &sel, locales.QueryPageData(), locales.QueryPageChart()) {
				q.page = pageData + sel
			}
		}
	})
	hairline(c, p.border)
}
