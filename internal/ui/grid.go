package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// resultGrid shows rows of a result in a virtualized table with cell block
// selection, sortable headers and copy in several formats. Table and query
// tabs share it.
type resultGrid struct {
	cols []model.ColumnMeta
	rows []map[string]any

	list    ui.ListState
	chosen  ui.Selection[int]
	uiCols  []ui.TableColumn
	libSort ui.SortOrder // 只用来接住 ui.Table 的表头点击，见 view
	sig     string
	sort    ui.SortOrder
	marks   map[string]sortMark // 多列排序时每列的方向与序号（第几个排序列）

	sortable bool
	local    bool             // 排序在内存里做（查询结果）；否则交给调用方重新查询（表数据）
	src      []map[string]any // local 时的原始顺序，取消排序时回到它
	offset   int              // number of the first row, minus one
	dbType   model.DBType     // for INSERT copies
	table    string

	// rowTint, when set, gives a row's state colour (deleted, new), which
	// also tints its row number.
	rowTint0 func(c *ui.Context, row int) ui.Color
	// menuOpened 记下右键菜单刚弹出过：右键只选中了格子，键盘焦点还在别处，
	// 要在这一帧画完表格后把焦点交给它，⌘D、⌘↵ 这些表格快捷键才会生效。
	menuOpened bool
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

// sortMark 是列头上的排序标记：方向，以及多列排序时它是第几个排序列（单列排序为 0，不显示序号）。
type sortMark struct {
	desc bool
	n    int
}

// gridCellH is the height of a cell's content: with the cell padding of the
// grid's theme every row is gridRowH, which the stripes below the rows rely on.
const (
	gridSpacing = 2.75                         // 表格主题的间距单位：表头 Space(8) = 22px，加分隔线共 23px
	gridRowH    = 24.5                         // 一行的总高度，表格下方的斑马纹按它铺
	gridCellH   = gridRowH - 2*1.5*gridSpacing // 内容高度：行高减去上下内边距 Space(1.5)
)

// showRowNumbers is the Settings choice for the row number column.
var showRowNumbers = true

// showZebra 是设置里"斑马纹"的开关（默认开）。
var showZebra = true

func newResultGrid(sortable bool) *resultGrid { return &resultGrid{sortable: sortable} }

// newLocalGrid 是查询结果用的网格：点列名就在内存里排序，三态与表数据一致。
func newLocalGrid() *resultGrid { return &resultGrid{sortable: true, local: true} }

func (g *resultGrid) set(cols []model.ColumnMeta, rows []map[string]any) {
	g.cols, g.src = cols, rows
	g.rows = g.arranged(rows)
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
		w := estimateTextWidth(col.Name, 7) + 36 // 表头：文字 + 排序标记 + 内边距
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

// mark 是列头上的排序标记：多列排序时带序号，只按一列排时没有序号。
func (g *resultGrid) mark(name string) (sortMark, bool) {
	if m, ok := g.marks[name]; ok {
		return m, true
	}
	if g.sort.Column == name && name != "" {
		return sortMark{desc: g.sort.Descending}, true
	}
	return sortMark{}, false
}

// headerMark 在列头标题后面画排序标记：一枚强调色的小 V 形（升序朝上、降序朝下），
// 多列排序时后面跟一个小序号。标题本身不再夹带 ↑/↓ 字符，换标记时列头的文字不会跟着重排。
func (g *resultGrid) headerMark(c *ui.Context, col int) {
	if col <= 0 || col > len(g.vis) {
		return
	}
	m, ok := g.mark(g.col(col).Name)
	if !ok {
		return
	}
	acc := c.Theme().Accent
	ic := icSortUp
	if m.desc {
		ic = icSortDown
	}
	ui.Icon(c, ic).FontSize(10).TextColor(acc).Margin(0, 0, 0, 4).Shrink(0)
	if m.n > 0 {
		ui.Text(c, itoa(m.n)).FontSize(9).FontWeight(600).TextColor(acc).Margin(0, 0, 0, 1).Shrink(0)
	}
}

// rebuild lays the table's columns out from the columns shown.
func (g *resultGrid) rebuild() {
	// 重排表头（排序标记变了、列显示/隐藏）时沿用各列已有的宽度：
	// 否则列宽会先缩回初始值、等数据回来再撑开，看起来在抖。
	prev := map[string]float32{}
	for _, col := range g.uiCols {
		prev[col.ID] = col.Width
	}
	g.vis = g.vis[:0]
	g.uiCols = []ui.TableColumn{{Title: "#", ID: "\x00#", RowHeader: true, Width: 56, Fixed: true}}
	for i, c := range g.cols {
		if g.hidden[c.Name] {
			continue
		}
		g.vis = append(g.vis, i)
		g.uiCols = append(g.uiCols, ui.TableColumn{Title: c.Name, ID: c.Name, Width: max(prev[c.Name], 72), Sortable: g.sortable, MinWidth: 60})
	}
}

// col is the column behind table column i (1-based: 0 is the row numbers).
func (g *resultGrid) col(i int) model.ColumnMeta { return g.cols[g.vis[i-1]] }

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

// inMultiBlock 报告 (row, col) 这一格是否落在一个多于一格的选中区块里。
func (g *resultGrid) inMultiBlock(row, col int) bool {
	if col <= 0 || col > len(g.vis) || !g.chosen.Has(row) {
		return false
	}
	b := g.selected()
	return slices.Contains(b.cols, g.col(col).Name) && len(b.rows)*len(b.cols) > 1
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
	g.list.CellsStyle = ui.CellsStyle{Cell: th.Accent.Alpha(0.30), CellText: th.Accent, Row: th.Accent.Alpha(0.14), Stripe: stripe}
	if g.sortable {
		// ui.Table 自己画的排序箭头是个小 V 形，而且只会在升/降序间切换；
		// 这里让它只记录"点了哪一列"（libSort 每帧清零，所以它不画箭头），
		// 三态切换和箭头（列名后的 ↑/↓）都由我们自己处理。
		g.list.Sort = &g.libSort
	}
	g.list.HeaderCell = func(_ ui.Element, col int) { g.headerMark(c, col) }
	g.list.CellMenu = func(m *ui.Menu, row, col int) {
		if row < 0 || row >= len(g.rows) {
			return
		}
		g.menuOpened = true
		if g.extraMenu != nil {
			defer g.extraMenu(m, row, col)
		}
		// 右键点在多格选区里：先给出"复制单元格"，把整块选中的值按 TSV 复制（可直接粘进表格软件）；
		// "复制值"仍只复制被点的这一格
		if g.inMultiBlock(row, col) && m.Item(locales.TableCopyCells()).Chosen() {
			if g.copyCells(c) {
				toast(locales.StatusCopied())
			}
		}
		if col > 0 && m.Item(locales.TableCopyValue()).Chosen() {
			v, _ := cellText(g.rows[row][g.col(col).Name])
			c.Services().WriteClipboard(v)
			toast(locales.StatusCopied())
		}
		if col > 0 { // 针对被点的这一格
			m.Submenu(locales.TableCopyValueAs(), func(m *ui.Menu) {
				for k, name := range copyCellFormats() {
					if m.Item(name).Chosen() {
						c.Services().WriteClipboard(cellCopyText(k, g.dbType, g.col(col).Name, g.rows[row][g.col(col).Name]))
						toast(locales.StatusCopied())
					}
				}
			})
		}
		m.Submenu(locales.TableCopyRowsAs(), func(m *ui.Menu) { // 针对整行：选中的行，所有可见列
			for k, name := range copyFormatNames {
				if m.Item(name).Chosen() {
					if g.chosen.Len() == 0 {
						g.chosen.Add(row)
					}
					if g.copyRowsAs(c, k, true) {
						toast(locales.StatusCopied())
					}
				}
			}
		})
	}
	// The original's grid is dense: ~20px rows, headers in the text color.
	gt := *th
	// 表头沿用老版 .data-grid-header：等宽半粗体（11px，排序箭头才看得清）、正文色，上下各 4px 内边距，
	// 连同 1px 分隔线共 23px 高；单元格的字号都是单独设的，所以这里的字号只管表头。
	gt.Spacing, gt.FontSize, gt.Font = gridSpacing, 11, mono
	gt.TextMuted = th.Text
	var changed bool
	ui.Themed(c, &gt).Background(ui.Transparent).FixedLineHeight(14).Grow(1).FillWidth().MinWidth(0).Children(func() {
		changed = g.tableUI(c, th, label, toast)
	})
	// Table 的 Changed 连"选中了别的格子"也算，所以只认 libSort：它有值才是点了表头。
	clicked := g.libSort.Column
	g.libSort = ui.SortOrder{}
	if !changed || !g.sortable || clicked == "" {
		return false
	}
	// 三态循环，和老版一样：不排序 → 升序 → 降序 → 回到不排序。
	switch {
	case g.sort.Column != clicked:
		g.sort = ui.SortOrder{Column: clicked}
	case !g.sort.Descending:
		g.sort = ui.SortOrder{Column: clicked, Descending: true}
	default:
		g.sort = ui.SortOrder{}
	}
	if g.local {
		g.rebuild() // 表头的箭头跟着变
		g.rows = g.arranged(g.src)
		g.chosen.Clear()
		return false
	}
	return true
}
