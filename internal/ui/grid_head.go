package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 数据网格的列头（设计稿「列头」）：列名前面一枚类型徽标——整数 123（蓝）、小数 1.0（蓝）、
// 文字 Aa（绿）、时间用时钟图标（灰）；列名半粗；主键在名字后面跟一把橙色钥匙。
// 查询结果的网格（plain）和表数据一样画列头；驱动没报出类型的列（表达式、常量）不画徽标，只有半粗的列名。

// icClock 是时间类列的徽标图标。
var icClock = svg(`<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>`)

// typeKind 把数据库的列类型归成徽标的四类。
type typeKind int

const (
	tyText typeKind = iota
	tyInt
	tyFloat
	tyTime
)

// typeKindOf 按类型名判断徽标种类；认不出的按文字。
func typeKindOf(typ string) typeKind {
	t := strings.ToLower(typ)
	switch {
	case strings.Contains(t, "date") || strings.Contains(t, "time"):
		return tyTime
	case strings.HasPrefix(t, "enum") || strings.Contains(t, "char") || strings.Contains(t, "text"):
		return tyText
	case strings.Contains(t, "dec") || strings.Contains(t, "numeric") || strings.Contains(t, "float") || strings.Contains(t, "double") || strings.Contains(t, "real") || strings.Contains(t, "money"):
		return tyFloat
	case strings.Contains(t, "int") || strings.Contains(t, "serial"):
		return tyInt
	}
	return tyText
}

// typeBadge 在列名前画类型徽标。
func typeBadge(c *ui.Context, p tokens, typ string) {
	k := typeKindOf(typ)
	bg, fg := p.infoBg, p.info
	switch k {
	case tyText:
		bg, fg = p.successBg, p.success
	case tyTime:
		bg, fg = p.surfaceTertiary, p.fgSecondary
	}
	b := ui.Row(c.Key("tyb")).Height(sizeTypeBadge).MinWidth(sizeTypeBadge+spaceSM).Padding(0, spaceXS+1).Margin(0, spaceSM-1, 0, 0).Shrink(0).Center().Radius(radiusSm).Background(bg)
	b.Children(func() {
		switch k {
		case tyInt:
			ui.Text(c, "123").Font(mono).FontSize(fontMicro).FontWeight(600).TextColor(fg)
		case tyFloat:
			ui.Text(c, "1.0").Font(mono).FontSize(fontMicro).FontWeight(600).TextColor(fg)
		case tyTime:
			ui.Icon(c, icClock).FontSize(fontMicro + 1).TextColor(fg)
		default:
			ui.Text(c, "Aa").FontSize(fontMicro).FontWeight(600).TextColor(fg)
		}
	})
}

// hasBadge 报告这一列的列头要不要画类型徽标：表数据的列都画；查询结果只画驱动报出了类型的列。
func (g *resultGrid) hasBadge(m model.ColumnMeta) bool { return !g.plain || m.Type != "" }

// headerExtra 是列头在列名之外占的宽度：徽标、主键钥匙，用来估初始列宽。参数：m 是这一列。
func (g *resultGrid) headerExtra(m model.ColumnMeta) float32 {
	var w float32
	if g.hasBadge(m) {
		w = float32(sizeTypeBadge + spaceSM + spaceSM)
	}
	if g.pkOf(m.Name) {
		w += 16
	}
	return w
}

// headerCell 画一列的表头内容：先是徽标（有类型时）和列名（列的 Title 是空的，由这里自己画），再是主键、固定、排序标记。
func (g *resultGrid) headerCell(c *ui.Context, col int) {
	if col <= 0 || col > len(g.vis) {
		return
	}
	m := g.col(col)
	p := baseTokens(c)
	if g.hasBadge(m) {
		typeBadge(c, p, m.Type)
	}
	ui.Text(c, m.Name).FontSize(fontCaption + 0.5).FontWeight(600).TextColor(p.fg).SingleLine().MinWidth(0).Shrink(1)
	if g.pkOf(m.Name) {
		ui.Icon(c, icKey).FontSize(fontCaption).TextColor(p.warning).Margin(0, 0, 0, spaceXS+1).Shrink(0)
	}
	g.headerMark(c, col)
}
