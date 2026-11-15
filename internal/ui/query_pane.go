package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 查询页的三种整页形态：编辑器 + 结果、执行计划、图表。切换时新形态淡入（motion_pane.go）。

// body 画查询页的主体。三种形态各自一个元素，从结果切到图表、进出执行计划时内容淡入。
func (q *queryTab) body(c *ui.Context, p tokens) {
	kind := "edit"
	switch {
	case q.page == pagePlan && q.plan != nil:
		kind = "plan"
	case q.chartTarget() != nil:
		kind = "chart"
	}
	paneIn(ui.Column(c.Key("qbody-" + kind))).Fill().Children(func() { q.bodyView(c, p) })
}
