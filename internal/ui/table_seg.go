package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 视图切换（设计稿 .vseg）：灰色圆角轨道，选中的一段是抬起来的白色（带细阴影），字比普通分段开关更大。
// 用在表标签顶部的 数据 / 结构 / DDL / 文档。

// viewSeg 画视图切换，返回被点中的段（没有则 -1）。段高用 actBtn 那一档（22），加上轨道上下各 2，整条 26，和同一行的过滤条、按钮等高。
func viewSeg(c *ui.Context, p tokens, key string, labels []string, cur int) (clicked int) {
	clicked = -1
	ui.Row(c.Key(key)).Gap(2).Padding(2).Margin(0, spaceSM, 0, 0).Radius(radiusView).Background(p.segTrack).AlignItems(ui.Center).Shrink(0).Children(func() {
		for i, l := range labels {
			i, l := i, l
			b := ui.Button(c.Key(fmt.Sprint(key, i)), "").Label(l).Padding(0, spaceLG-3).Radius(radiusViewIt).Height(sizeBtnSm)
			if i == cur {
				b.Background(p.segThumb).Border(1, p.borderSubtle).Shadow(0, 1, 2, 0, p.shadow)
			} else {
				b.Background(ui.Color{}).Border(1, ui.Transparent) // 未选中也留着透明边，切换时不抖
			}
			b.Children(func() {
				col, w := p.fgSecondary, 500
				if i == cur {
					col, w = p.fg, 600
				}
				ui.Text(c, l).FontSize(fontCtl).FontWeight(w).TextColor(col).SingleLine()
			})
			if b.Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}
