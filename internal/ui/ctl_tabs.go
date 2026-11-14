package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 下划线标签：同一个页面里的并列分区（结构页的 列 / 索引 / 外键 / 触发器，监控的分页等）。
// 设计稿 .ul：整条高 28，底部一条分隔线；选中项文字用主文字色，下面压一条 2px 的强调色线；
// 标签后可以跟灰色的计数。与分段开关（segmented）的分工：分段开关切换「同一份数据的视图」，
// 下划线标签切换「同级的不同内容」。

// ulTabs 画一条下划线标签：labels 是各标签文字，counts 是各标签后的计数（长度不够或为 nil 的标签不显示计数，
// 负数也不显示），cur 是当前标签。返回被点中的标签下标，没有点击返回 -1。
func ulTabs(c *ui.Context, p tokens, key string, labels []string, counts []int, cur int) (clicked int) {
	clicked = -1
	ui.Row(c.Key(key)).FillWidth().Height(sizeUnder).Shrink(0).Gap(2).AlignItems(ui.End).Children(func() {
		for i, l := range labels {
			i, l := i, l
			b := ui.Button(c.Key(fmt.Sprint(key, i)), "").Label(l).Height(sizeUnder).Padding(0, 10).Gap(5).
				AlignItems(ui.Center).Radius(radiusSm).Background(ui.Transparent).BorderWidth(0)
			if b.Hovered() && i != cur {
				b.Background(p.hover)
			}
			fg := p.fgSecondary
			if i == cur {
				fg = p.fg
				b.DrawOver(func(pt *ui.Painter, r ui.Rect) {
					pt.Fill(ui.Rect{X: r.X + 8, Y: r.Y + r.H - 2, W: r.W - 16, H: 2}, p.accent, 1)
				})
			}
			b.Children(func() {
				ui.Text(c, l).FontSize(fontCtl).FontWeight(500).TextColor(fg).SingleLine()
				if i < len(counts) && counts[i] >= 0 {
					ui.Text(c, fmt.Sprint(counts[i])).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
				}
			})
			if b.Clicked() {
				clicked = i
			}
		}
	})
	hairline(c, p.border)
	return clicked
}
