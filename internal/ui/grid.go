package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// resultGrid shows rows of a result in a virtualized table with cell block
// selection, sortable headers and copy in several formats. Table and query
// tabs share it.
type resultGrid struct {
	cols []database.ColumnMeta
	rows []map[string]any

	list   ui.ListState
	chosen ui.Selection[int]
	uiCols []ui.TableColumn
	sig    string
	sort   ui.SortOrder
	marks  map[string]string // header suffixes, e.g. " ↑2" for the second sort column

	sortable bool
	offset   int    // number of the first row, minus one
	dbType   string // for INSERT copies
	table    string

	copyKind int
	// rowTint, when set, gives a row's state colour (deleted, new), which
	// also tints its row number.
	rowTint0 func(c *ui.Context, row int) ui.Color
	// onKeys handles the grid's keyboard, while it has the focus.
	onKeys func(c *ui.Context, tbl ui.Element)
	// cell, when set, draws a cell itself and reports whether it did; the
	// table tab uses it for editing.
	// hidden columns are left out of the grid, its selections and its copies
	hidden map[string]bool
	vis    []int // indices into cols of the columns shown
	cell   func(c *ui.Context, row, col int, selected bool) bool
	// extraMenu adds items to the cell context menu.
	extraMenu func(m *ui.Menu, row, col int)
}

// gridCellH is the height of a cell's content: with the cell padding of the
// grid's theme every row is gridRowH, which the stripes below the rows rely on.
const (
	gridCellH = 17
	gridRowH  = gridCellH + 7.5
)

// showRowNumbers is the Settings choice for the row number column.
var showRowNumbers = true

// showZebra 是设置里"斑马纹"的开关（默认开）。
var showZebra = true

func newResultGrid(sortable bool) *resultGrid { return &resultGrid{sortable: sortable} }

func (g *resultGrid) set(cols []database.ColumnMeta, rows []map[string]any) {
	g.cols, g.rows = cols, rows
	g.chosen.Clear()
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	sig := strings.Join(names, "\x00")
	if sig == g.sig {
		g.fitWidths() // 同样的列、新的数据：按数据重新量一遍初始宽度（拖过的列宽不受影响）
		return
	}
	g.sig = sig
	for name := range g.hidden { // columns that are gone from the result
		if !slices.Contains(names, name) {
			delete(g.hidden, name)
		}
	}
	g.rebuild()
	g.fitWidths()
	g.list.Columns = ui.TableLayout{}
}

// fitWidths 按内容给各列定初始宽度：表头与前 100 行里最宽的那个值决定，
// 夹在 [72, 360] 之间——id、布尔、短状态列很窄，长文本/JSON 封顶，不再按类型一刀切。
func (g *resultGrid) fitWidths() {
	for k, i := range g.vis {
		if k+1 >= len(g.uiCols) {
			break
		}
		col := g.cols[i]
		w := estimateTextWidth(col.Name+g.marks[col.Name], 8) + 40 // 表头：文字 + 排序箭头 + 内边距
		for r, row := range g.rows {
			if r >= 100 {
				break
			}
			v, _ := cellText(row[col.Name])
			if j := strings.IndexByte(v, '\n'); j >= 0 {
				v = v[:j]
			}
			if rs := []rune(v); len(rs) > 80 {
				v = string(rs[:80])
			}
			w = max(w, estimateTextWidth(v, 7.4)+32)
		}
		// 只增不减：翻页时列宽不跟着数据来回跳；列变了（rebuild）才重新起算
		g.uiCols[k+1].Width = max(g.uiCols[k+1].Width, min(max(w, 72), 360))
	}
}

// estimateTextWidth 估算一段文字的宽度：半角按 perRune 算，全角（中日韩等）算两倍。
func estimateTextWidth(s string, perRune float32) float32 {
	var w float32
	for _, r := range s {
		if r > 0x2e80 {
			w += perRune * 1.8
		} else {
			w += perRune
		}
	}
	return w
}

// rebuild lays the table's columns out from the columns shown.
func (g *resultGrid) rebuild() {
	g.vis = g.vis[:0]
	g.uiCols = []ui.TableColumn{{Title: "#", ID: "\x00#", RowHeader: true, Width: 56, Fixed: true}}
	for i, c := range g.cols {
		if g.hidden[c.Name] {
			continue
		}
		g.vis = append(g.vis, i)
		g.uiCols = append(g.uiCols, ui.TableColumn{Title: c.Name + g.marks[c.Name], ID: c.Name, Width: 72, Sortable: g.sortable, MinWidth: 60})
	}
}

// col is the column behind table column i (1-based: 0 is the row numbers).
func (g *resultGrid) col(i int) database.ColumnMeta { return g.cols[g.vis[i-1]] }

// setHidden shows or hides a column.
func (g *resultGrid) setHidden(name string, hide bool) {
	if g.hidden == nil {
		g.hidden = map[string]bool{}
	}
	if hide {
		g.hidden[name] = true
	} else {
		delete(g.hidden, name)
	}
	g.rebuild()
	g.fitWidths()
	g.list.Columns = ui.TableLayout{}
	g.chosen.Clear()
}

// selected returns the chosen block: the chosen rows by the chosen columns,
// or every column when none are chosen by header.
func (g *resultGrid) selected() block {
	var rows []int
	for r := range g.chosen.All() {
		if r >= 0 && r < len(g.rows) {
			rows = append(rows, r)
		}
	}
	slices.Sort(rows)
	colNames := g.list.SelectedColumns()
	var cols []string
	for _, i := range g.vis {
		if c := g.cols[i]; len(colNames) == 0 || slices.Contains(colNames, c.Name) {
			cols = append(cols, c.Name)
		}
	}
	b := block{cols: cols}
	for _, r := range rows {
		b.rows = append(b.rows, g.rows[r])
	}
	return b
}

func (g *resultGrid) copySelection(c *ui.Context, kind int, header bool) bool {
	b := g.selected()
	if len(b.rows) == 0 {
		return false
	}
	c.Services().WriteClipboard(b.format(kind, g.dbType, g.table, header))
	return true
}

func (g *resultGrid) colIndex(name string) int {
	for i, c := range g.cols {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// view draws the grid and reports whether the user changed the sort.
func (g *resultGrid) view(c *ui.Context, label string, toast func(string)) (sortChanged bool) {
	th := c.Theme()
	if len(g.cols) == 0 {
		return false
	}
	g.list.Selection, g.list.CellSelect = &g.chosen, true
	if len(g.uiCols) > 0 { // hidden row numbers leave a slim gutter that still chooses the row
		g.uiCols[0].Width = map[bool]float32{true: 56, false: 14}[showRowNumbers]
	}
	// 选中的格子：淡淡的强调色底 + 一圈强调色边框（边框取自 CellText），
	// 所在整行只铺一层很浅的底，文字保持各自的类型颜色，不再整块发蓝。
	stripe := baseTokens(c).surfaceSecondary
	if !showZebra {
		stripe = ui.Transparent
	}
	g.list.CellsStyle = ui.CellsStyle{Cell: th.Accent.Alpha(0.13), CellText: th.Accent, Row: th.Accent.Alpha(0.055), Stripe: stripe}
	if g.sortable {
		g.list.Sort = &g.sort
	}
	g.list.CellMenu = func(m *ui.Menu, row, col int) {
		if row < 0 || row >= len(g.rows) {
			return
		}
		if g.extraMenu != nil {
			defer g.extraMenu(m, row, col)
		}
		if col > 0 && m.Item(locales.TableCopyValue()).Chosen() {
			v, _ := cellText(g.rows[row][g.col(col).Name])
			c.Services().WriteClipboard(v)
			toast(locales.StatusCopied())
		}
		m.Submenu(locales.TableCopyAs(), func(m *ui.Menu) {
			for k, name := range copyFormatNames {
				if m.Item(name).Chosen() {
					if g.chosen.Len() == 0 {
						g.chosen.Add(row)
					}
					if g.copySelection(c, k, true) {
						toast(locales.StatusCopied())
					}
				}
			}
		})
	}
	// The original's grid is dense: ~20px rows, headers in the text color.
	gt := *th
	gt.Spacing, gt.FontSize, gt.Font = 2.5, font2XS, mono // headers are mono too
	gt.TextMuted = th.Text
	var changed bool
	ui.Themed(c, &gt).Background(ui.Transparent).FixedLineHeight(14).Grow(1).FillWidth().MinWidth(0).Children(func() {
		changed = g.tableUI(c, th, label, toast)
	})
	return changed && g.sortable && g.sort.Column != ""
}

func (g *resultGrid) tableUI(c *ui.Context, th *ui.Theme, label string, toast func(string)) bool {
	tbl := ui.Table(c, &g.list, g.uiCols, len(g.rows), func(row, col int) {
		selected := g.list.CellSelected(row, col)
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
		text, null := cellText(g.rows[row][g.col(col).Name])
		pal := baseTokens(c)
		color := valueColor(pal, g.col(col).Type)
		if null {
			color, text = pal.fgMuted, locales.TableNull()
		}
		cellBox(c, ui.Color{}, g.colWidth(col), func() {
			t := ui.Text(c, oneLine(text)).Font(mono).FontSize(font2XS).FixedLineHeight(14).TextColor(color).SingleLine()
			if null {
				t.Italic()
			}
		})
	}).Grow(1).MinWidth(0).Label(label)
	tbl = listBars(c, tbl, &g.list)
	// Like the original, the stripes run on below the last row, down to the
	// bottom of the grid, so an empty or short table still looks like a table.
	n, stripe, head := len(g.rows), baseTokens(c).surfaceSecondary, c.Theme().Space(8)+1 // the header, in the grid's own theme
	tbl.Draw(func(p *ui.Painter, r ui.Rect) {
		if !showZebra {
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
// width). A tint spreads over the whole cell, padding included, so row states
// colour the row without gaps between cells.
func cellBox(c *ui.Context, tint ui.Color, w float32, content func()) ui.Element {
	row := ui.Row(c).AlignItems(ui.Center)
	if tint == (ui.Color{}) || w <= 0 {
		return row.FillWidth().Height(gridCellH).Children(content)
	}
	t := c.Theme()
	v, h := t.Space(1.5), t.Space(2.5)
	return row.Width(w).Height(gridRowH).Margin(-v, -h).Padding(0, h).Background(tint).Children(content)
}

func fgMuted(c *ui.Context) ui.Color { return baseTokens(c).fgMuted }

func (g *resultGrid) rowTint(c *ui.Context, row int) ui.Color {
	if g.rowTint0 == nil {
		return ui.Color{}
	}
	return g.rowTint0(c, row)
}
