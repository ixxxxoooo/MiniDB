package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/internal/logger"
	"minidb/locales"
)

func (g *resultGrid) tableUI(c *ui.Context, th *ui.Theme, label string, toast func(string)) bool {
	defer logger.PerfUI("ui.build.grid")() // 数据网格是最重的一段：行数多、列多时先怀疑它
	tbl := ui.Table(c, &g.list, g.uiCols, len(g.rows), func(row, col int) {
		selected := g.list.CellSelected(row, col)
		if col > 0 && g.reveal != nil {
			g.markReveal(c, row, col)
		}
		if col == 0 {
			cellBoxAt(c, g.rowTint(c, row), g.colWidth(0), true, func() {
				if !showRowNumbers {
					return
				}
				ui.Text(c, fmt.Sprint(g.offset+row+1)).FontSize(fontCaption).FixedLineHeight(14).TextColor(baseTokens(c).fgFaint).SingleLine()
			})
			return
		}
		if g.cell != nil && g.cell(c, row, col, selected) {
			return
		}
		text, null := cellDisplay(g.col(col), g.rows[row][g.col(col).Name])
		pal := baseTokens(c)
		if null {
			text = locales.TableNull()
		}
		cb := cellBoxAt(c, ui.Color{}, g.colWidth(col), g.numEnd(col), func() {
			valueText(c, pal, g.kindOf(g.col(col), g.pkOf(g.col(col).Name)), text, null)
		})
		if !null && text != "" {
			bigTip(c, cb, clipText(text, 500, 14)) // 悬浮看完整内容，太长就截断
		}
	}).Grow(1).MinWidth(0).Label(label).Padding(1, 0, 0, 0) // 库里的表头比设计稿的 28 少 1px，顶上补一格
	tbl = listBars(c, tbl, &g.list)
	paintGridBg(tbl, len(g.rows), baseTokens(c), gridHeadH, !g.plain, nil) // 表头高度取表格自己的主题
	if g.menuOpened {                                                      // 右键之后表格也要拿到键盘，菜单里标的快捷键才不是摆设
		g.menuOpened = false
		tbl.Focus(ui.FocusOptions{PreventScroll: true})
	}
	// scoped to the grid, so typing in a field or an editor is left alone
	if g.onKeys != nil {
		g.onKeys(c, tbl)
	}
	return tbl.Changed()
}

// colWidth is the width of column i of the grid (0 is the row numbers), as
// the user has dragged it or as it started.
func (g *resultGrid) colWidth(i int) float32 {
	if i < 0 || i >= len(g.uiCols) {
		return 0
	}
	col := g.uiCols[i]
	if w, ok := g.list.Columns.Widths[col.ID]; ok && !col.Fixed {
		return w
	}
	return col.Width
}

// cellBox is the content row of a grid cell, w wide in all (its column's
// width). It always spreads over the whole cell, padding included: a tint then
// colours the row without gaps between cells, and a click or double click
// anywhere in the cell, edges too, lands on it. (Without a tint it used to be
// only as big as its content, so a double click on the padding selected the
// row but never started an edit.)
func cellBox(c *ui.Context, tint ui.Color, w float32, content func()) ui.Element {
	return cellBoxAt(c, tint, w, false, content)
}

// cellBoxAt 同 cellBox；end 为 true 时内容靠右（数值列，设计稿的数字右对齐）。
func cellBoxAt(c *ui.Context, tint ui.Color, w float32, end bool, content func()) ui.Element {
	t := c.Theme()
	v, h := t.Space(1.5), t.Space(2.5)
	row := ui.Row(c).AlignItems(ui.Center).Height(gridRowH).Margin(-v, -h).Padding(0, gridPadX)
	if w <= 0 { // 填满剩余宽度的那一列
		row = row.FillWidth()
	} else {
		row = row.Width(w)
	}
	if tint != (ui.Color{}) {
		row = row.Background(tint).Transition(cellFlash)
	}
	if end {
		row = row.Justify(ui.End)
	}
	if !showZebra { // 设计稿的细行线：每个单元格在自己底边画 1px，整行连成一条
		line := baseTokens(c).borderSubtle
		row = row.DrawOver(func(pt *ui.Painter, r ui.Rect) { pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, line, 0) })
	}
	return row.Children(content)
}

func (g *resultGrid) rowTint(c *ui.Context, row int) ui.Color {
	if g.rowTint0 == nil {
		return ui.Color{}
	}
	return g.rowTint0(c, row)
}

// paintGridBg 给表格铺底：表头的底色，以及延伸到表格底部的行线或斑马纹——
// 行数不够时下面也是一张表的样子。head 是表头（含分隔线）的高度。
// 行样式照设计稿：默认每行底下一条 1px 细线（borderSubtle），设置里开了斑马纹就换成隔行底色。
// fill 为假时不铺表头以外的空行线；extra 不为 nil 时，在铺完表头底色后再调用一次，给调用方多画一点（比如延伸行底色）。
func paintGridBg(tbl ui.Element, n int, p tokens, head float32, fill bool, extra func(pt *ui.Painter, r ui.Rect)) {
	zebra := showZebra
	tbl.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: head - 1}, p.surfaceSecondary, 0) // 表头的底色
		if extra != nil {
			extra(pt, r)
		}
		if !fill { // 结构、概览这些表（设计稿）：行数不够时下面留白，不铺空行
			return
		}
		bottom := r.Y + r.H
		// 数据行会随表格滚动：它们的行线由单元格自己画（cellBox），斑马纹由表格铺；这里只补数据行下面的空行
		for i := n; ; i++ {
			y := r.Y + head + float32(i)*gridRowH
			if y >= bottom {
				return
			}
			switch {
			case zebra && i%2 == 1:
				pt.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: min(gridRowH, bottom-y)}, p.stripe, 0)
			case !zebra && y+gridRowH <= bottom:
				pt.Fill(ui.Rect{X: r.X, Y: y + gridRowH - 1, W: r.W, H: 1}, p.borderSubtle, 0)
			}
		}
	})
}
