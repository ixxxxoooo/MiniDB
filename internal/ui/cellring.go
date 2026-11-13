package ui

import "github.com/egoist/mygo/ui"

// paintEditRing 在 r 的边缘画编辑态的外框：外圈 1.5px 强调色实线，
// 内圈再叠一层淡淡的光晕，像浮起来的输入框。数据网格和结构视图的单元格编辑共用。
//
// @author ixxxxoooo
//
// 参数：pt 画笔；r 单元格的外框；accent 强调色。无返回值。
func paintEditRing(pt *ui.Painter, r ui.Rect, accent ui.Color) {
	ring := func(in, w float32, col ui.Color) {
		x, y, ww, hh := r.X+in, r.Y+in, r.W-2*in, r.H-2*in
		for _, e := range []ui.Rect{{X: x, Y: y, W: ww, H: w}, {X: x, Y: y + hh - w, W: ww, H: w}, {X: x, Y: y, W: w, H: hh}, {X: x + ww - w, Y: y, W: w, H: hh}} {
			pt.Fill(e, col, 0)
		}
	}
	ring(0, 1.5, accent)
	ring(1.5, 2, accent.Alpha(0.16))
}

// ringCellBox 是带编辑外框的单元格：盒子铺满整个单元格（含表格自己的内边距，
// 和选中态的底色一样大），content 里的输入框拿到焦点时 *focused 为真，
// 盒子外圈就描上编辑框、底色换成 surface（和数据网格的编辑态一样）。
// tint 为零值时不铺底色；w <= 0 表示填满剩余宽度。
//
// 结构视图的网格套了一份「淡强调色」主题，所以强调色必须取外层的 p.accent，
// 不能在这里用 baseTokens(c)，否则外框会发灰。
//
// 参数：c 上下文；p 外层（原主题）的色板；tint 行底色；w 列宽；focused 由 content 回写输入框是否有焦点；content 单元格内容。
// 返回值：单元格的盒子元素。
func ringCellBox(c *ui.Context, p tokens, tint ui.Color, w float32, focused *bool, content func()) ui.Element {
	th := c.Theme()
	v, h := th.Space(1.5), th.Space(2.5)
	row := ui.Row(c).AlignItems(ui.Center).Height(gridRowH).Margin(-v, -h).Padding(0, h)
	if w > 0 {
		row = row.Width(w)
	} else {
		row = row.FillWidth()
	}
	if tint != (ui.Color{}) {
		row = row.Background(tint)
	}
	row = row.Children(content)
	if *focused {
		row = row.Background(p.surface)
		row.DrawOver(func(pt *ui.Painter, r ui.Rect) { paintEditRing(pt, r, p.accent) })
	}
	return row
}

// editInput 让网格里的输入框用原强调色画光标和选区（网格主题里的强调色被调淡了，光标会看不清）。
//
// 参数：in 输入框元素；accent 原主题的强调色。
// 返回值：同一个元素，便于链式调用。
func editInput(in ui.Element, accent ui.Color) ui.Element {
	return in.CaretColor(accent).SelectionColor(accent.Alpha(0.3))
}
