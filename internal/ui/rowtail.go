// @author ixxxxoooo
package ui

import "github.com/egoist/mygo/ui"

// 横向滚到底时，Table 的滚动区右侧留着一条约 14px 的轨道，行的内容铺不进去，
// 靠单元格自己铺的行底色（斑马纹、新增 / 待删除）就会比表格的右边缘短一截。
// 补满列的单元格画的时候记下自己的位置和底色，下一帧由表格这一层把底色
// 从单元格右缘接着铺到表格边缘，整行就从左到右连成一片。

// rowTail 是一行底色要往右延伸的起点：补满列单元格的右缘 x、行的 y 和高 h、底色。
type rowTail struct {
	x, y, h float32
	color   ui.Color
}

// rowTails 按网格记着上一帧各行的 rowTail。界面只在一个线程里画，不加锁。
var rowTails = map[*ui.ListState][]rowTail{}

// recordRowTail 让补满列的单元格 cell 在画完时登记自己的位置，供表格下一帧延伸底色。
//
// 参数：ls 网格的状态（当作 key）；cell 补满列的单元格元素；tint 这一行的底色，零值不登记。
// 无返回值。
func recordRowTail(ls *ui.ListState, cell ui.Element, tint ui.Color) {
	if tint == (ui.Color{}) {
		return
	}
	cell.DrawOver(func(_ *ui.Painter, r ui.Rect) {
		rowTails[ls] = append(rowTails[ls], rowTail{x: r.X + r.W, y: r.Y, h: r.H, color: tint})
	})
}

// paintRowTails 在表格自己的绘制里（在单元格之下）把上一帧登记的行底色延伸到表格右缘 right，
// 随后清空，等本帧的单元格重新登记。
//
// 参数：pt 画笔；ls 网格的状态；right 表格右缘的 x。无返回值。
func paintRowTails(pt *ui.Painter, ls *ui.ListState, right float32) {
	for _, t := range rowTails[ls] {
		if t.x < right {
			pt.Fill(ui.Rect{X: t.x, Y: t.y, W: right - t.x, H: t.h}, t.color, 0)
		}
	}
	rowTails[ls] = rowTails[ls][:0]
}
