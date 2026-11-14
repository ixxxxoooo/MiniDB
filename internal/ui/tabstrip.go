package ui

import (
	"github.com/egoist/mygo/ui"
)

// 不能关闭的下划线标签栏：结构页下半区的 索引 / 外键 / 触发器、查询结果的 结果 N。外观同设计稿的 .ul。

// tabStrip 画一条不能关闭的标签栏（设计稿：34px 的条栏，下划线标签 + 灰色计数，右端放操作按钮），
// 返回被点中的标签下标（没有则 -1）。counts 是各标签后的计数（可为 nil、负数不显示），trailing 非空时画在最右端。
func tabStrip(c *ui.Context, p tokens, key string, labels []string, counts []int, cur int, trailing func()) (clicked int) {
	ui.Row(c.Key(key+"-bar")).FillWidth().Height(sizeBar).Shrink(0).Padding(0, spaceSM+2).AlignItems(ui.Center).Background(p.surface).Children(func() {
		clicked = ulStrip(c, p, key, labels, counts, cur, sizeBar, false)
		ui.Spacer(c)
		if trailing != nil {
			ui.Row(c.Key(key + "-trailing")).AlignItems(ui.Center).Gap(spaceXS).Shrink(0).Children(trailing)
		}
	})
	hairline(c, p.border)
	return clicked
}
