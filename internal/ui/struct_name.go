package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 已有的表在结构页：顶部条里「表名」后面跟一个输入框（设计稿 A 把表名放在页签同一行，不另占一行）。

// structNameInline 画条栏里的表名输入框；视图和只读连接里它是只读的。
func (t *tableTab) structNameInline(c *ui.Context, p tokens) {
	s := &t.st
	ro := t.w.cfg.ReadOnly || t.w.isViewName(t.table)
	ui.Text(c, locales.StructTableName()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Margin(0, 0, 0, spaceXS)
	ui.TextInput(c.Key("tname"), &s.name).Width(130).Height(sizeBtnSm).Padding(0, spaceSM).FontSize(fontCtl).ReadOnly(ro)
}
