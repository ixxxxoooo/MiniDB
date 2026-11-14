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
			cellBox(c, g.rowTint(c, row), g.colWidth(0), func() {
				if !showRowNumbers {
					return
				}
				ui.Text(c, fmt.Sprint(g.offset+row+1)).Font(mono).FontSize(font2XS).FixedLineHeight(14).TextColor(fgMuted(c)).SingleLine()
			})
			return
		}
		if g.cell != nil && g.cell(c, row, col, selected) {
			return
		}
		text, null := cellDisplay(g.col(col), g.rows[row][g.col(col).Name])
		pal := baseTokens(c)
		color := valueColor(pal, g.col(col).Type)
		if null {
			color, text = pal.fgMuted, locales.TableNull()
		}
		cb := cellBox(c, ui.Color{}, g.colWidth(col), func() {
			t := ui.Text(c, oneLine(text)).Font(mono).FontSize(font2XS).FixedLineHeight(14).TextColor(color).SingleLine()
			if null {
				t.Italic()
			}
		})
		if !null && text != "" {
			bigTip(c, cb, clipText(text, 500, 14)) // 悬浮看完整内容，太长就截断
		}
	}).Grow(1).MinWidth(0).Label(label)
	tbl = listBars(c, tbl, &g.list)
	paintGridBg(tbl, len(g.rows), baseTokens(c).stripe, c.Theme().Space(8)+1, showZebra, nil) // 表头高度取表格自己的主题
	if g.menuOpened {                                                                         // 右键之后表格也要拿到键盘，菜单里标的快捷键才不是摆设
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
	t := c.Theme()
	v, h := t.Space(1.5), t.Space(2.5)
	row := ui.Row(c).AlignItems(ui.Center).Height(gridRowH).Margin(-v, -h).Padding(0, h)
	if w <= 0 { // 填满剩余宽度的那一列
		row = row.FillWidth()
	} else {
		row = row.Width(w)
	}
	if tint != (ui.Color{}) {
		row = row.Background(tint)
	}
	return row.Children(content)
}

func fgMuted(c *ui.Context) ui.Color { return baseTokens(c).fgMuted }

func (g *resultGrid) rowTint(c *ui.Context, row int) ui.Color {
	if g.rowTint0 == nil {
		return ui.Color{}
	}
	return g.rowTint0(c, row)
}

// paintGridBg 给表格铺底：表头的底色，以及延伸到表格底部的斑马纹——
// 行数不够时下面也是一张表的样子，和老版一致。head 是表头（含分隔线）的高度。
// extra 不为 nil 时，在铺完表头底色后再调用一次，给调用方多画一点（比如延伸行底色）。
func paintGridBg(tbl ui.Element, n int, stripe ui.Color, head float32, zebra bool, extra func(p *ui.Painter, r ui.Rect)) {
	tbl.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: head - 1}, stripe, 0) // 表头的底色，同老版的 --row-stripe
		if extra != nil {
			extra(p, r)
		}
		if !zebra {
			return
		}
		bottom := r.Y + r.H
		for i := n; ; i++ {
			y := r.Y + head + float32(i)*gridRowH
			if y >= bottom {
				return
			}
			if i%2 == 1 {
				p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: min(gridRowH, bottom-y)}, stripe, 0)
			}
		}
	})
}
