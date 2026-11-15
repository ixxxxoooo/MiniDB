package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 安全模式的下拉：展开后每一级都写着它的简短说明，类别（提醒 / 安全）有各自的小标题，
// 鼠标停在某一级上还会浮出完整的说明（safeModeTip）。连接表单和设置页都用它，
// 不再是只有标签的普通下拉、选中之后才在旁边看到说明。

// safeSelect 画安全模式下拉。参数 c 为界面上下文（同一页有多个时传 c.Key），p 为主题色板，
// label 绑定当前级别的显示名，width 是触发器的宽度；级别被改动时返回 true。
func safeSelect(c *ui.Context, p tokens, label *string, width float32) bool {
	cur := safeModeFromLabel(*label)
	sel := ui.SelectBase(c, label)
	tipOn(c, sel.Trigger, safeModeTip(cur)).Width(width).Height(sizeBtn).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(radiusBtn).Background(p.surface).Border(1, p.border).Children(func() {
		ui.Icon(c, safeModeIcon(cur)).FontSize(14).TextColor(safeModeColor(p, cur))
		ui.Text(c, *label).FontSize(fontBase).Grow(1).SingleLine()
		ui.Icon(c, icChevronD).FontSize(12).TextColor(p.fgMuted)
	})
	safePopup(c, p, sel, cur)
	return sel.Trigger.Changed()
}
