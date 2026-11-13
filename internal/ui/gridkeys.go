package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// The grid's keyboard, after TablePlus's: handled only while the grid has the
// focus, so typing in a field or an editor is left alone.
//
//	Return / Tab      edit the cell          ⌥Return   Set Value menu
//	⌘C  copy rows     ⌘⇧C copy cells         ⌘V        paste rows
//	⌘⇧V paste into the chosen cells           ⌘D        duplicate rows
//	⌘Return           Quick Look             ⌘↑ / ⌘↓   first / last row
//	⌘← / ⌘→           previous / next page   Esc       deselect
//	⌫ / Delete        delete rows

// cursorCell is the cell the keys act on: the first chosen row, and the
// column the cursor is in (the first column shown when there is none).
func (t *tableTab) cursorCell() (row int, col string, ok bool) {
	row = t.previewRow()
	if row < 0 || len(t.g.vis) == 0 {
		return 0, "", false
	}
	col = t.g.list.Cells.Cursor
	if col == "" || col == "\x00#" || t.g.colIndex(col) < 0 {
		col = t.g.cols[t.g.vis[0]].Name
	}
	return row, col, true
}

// gridKeys runs the shortcuts of the grid; tbl is the Table element.
func (t *tableTab) gridKeys(c *ui.Context, tbl ui.Element) {
	g := t.g
	switch {
	case tbl.Shortcut(0, ui.KeyBackspace), tbl.Shortcut(0, ui.KeyDelete):
		if t.es.editing == nil && !t.w.cfg.ReadOnly && g.chosen.Len() > 0 {
			t.deleteSelected()
		}
	case tbl.Shortcut(0, ui.KeyTab):
		if row, col, ok := t.cursorCell(); ok {
			t.beginEdit(row, col)
		}
	case tbl.Shortcut(ui.Alt, ui.KeyEnter):
		if row, col, ok := t.cursorCell(); ok {
			t.quick = &quickCell{row: row, col: col}
		}
	case tbl.Shortcut(ui.Cmd, ui.KeyEnter):
		if row, col, ok := t.cursorCell(); ok {
			t.openQuickLook(row, col)
		}
	case tbl.Shortcut(ui.Cmd|ui.Shift, ui.KeyC):
		if g.copyCells(c) {
			t.w.sh.toastOK(locales.StatusCopied())
		}
	case tbl.Shortcut(ui.Cmd, ui.KeyC):
		if g.copyRows(c) {
			t.w.sh.toastOK(locales.StatusCopied())
		}
	case tbl.Shortcut(ui.Cmd|ui.Shift, ui.KeyV):
		t.pasteCells(c.ReadClipboard())
	case tbl.Shortcut(ui.Cmd, ui.KeyV):
		t.pasteRows(c.ReadClipboard())
	case tbl.Shortcut(ui.Cmd, ui.KeyD):
		t.duplicateRows()
	case tbl.Shortcut(ui.Cmd, ui.KeyUp):
		t.chooseRow(0)
	case tbl.Shortcut(ui.Cmd, ui.KeyDown):
		t.chooseRow(len(g.rows) - 1)
	case tbl.Shortcut(ui.Cmd, ui.KeyLeft):
		t.turnPage(-1)
	case tbl.Shortcut(ui.Cmd, ui.KeyRight):
		t.turnPage(1)
	}
}

// chooseRow chooses one row and scrolls it into view.
func (t *tableTab) chooseRow(row int) {
	if row < 0 || row >= len(t.g.rows) {
		return
	}
	t.g.chosen.Clear()
	t.g.chosen.Add(row)
	t.g.list.ScrollIntoView(row)
}

// turnPage goes to the next or previous page.
func (t *tableTab) turnPage(d int) {
	size := t.size()
	pages := max(int((t.total+int64(size)-1)/int64(size)), 1)
	if t.total < 0 {
		pages = t.page + 1
	}
	next := t.page + d
	if next < 1 || next > pages {
		return
	}
	t.reload(func() { t.page = next })
}

// copyRows copies the chosen rows, every column shown, as tab-separated text.
func (g *resultGrid) copyRows(c *ui.Context) bool { return g.copyRowsAs(c, fmtTSV, false) }

// copyRowsAs 把选中的行（所有可见列，不受选中的列影响）按 kind 的格式复制，header 表示带不带表头。
func (g *resultGrid) copyRowsAs(c *ui.Context, kind int, header bool) bool {
	var rows []int
	for r := range g.chosen.All() {
		if r >= 0 && r < len(g.rows) {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return false
	}
	slices.Sort(rows)
	b := block{}
	for _, i := range g.vis {
		b.cols = append(b.cols, g.cols[i].Name)
	}
	for _, r := range rows {
		b.rows = append(b.rows, g.rows[r])
	}
	c.Services().WriteClipboard(b.format(kind, g.dbType, g.table, header))
	return true
}

// copyCells copies the chosen cells: the chosen rows by the chosen columns.
func (g *resultGrid) copyCells(c *ui.Context) bool { return g.copySelection(c, fmtTSV, false) }

// ---- paste, duplicate ----

// clipboardRows splits clipboard text into rows of cells (tab separated).
func clipboardRows(text string) [][]string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out [][]string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, strings.Split(line, "\t"))
	}
	return out
}

// clipboardValue turns a pasted cell into a value: NULL is NULL.
func clipboardValue(s string) any {
	if s == "NULL" {
		return nil
	}
	return s
}

// newRow adds a row to the grid, filled with the table's defaults and vals.
func (t *tableTab) newRow(vals map[string]any) {
	row := map[string]any{}
	for _, c := range t.g.cols {
		row[c.Name] = nil
	}
	for k, v := range t.es.defaults {
		row[k] = v
	}
	for k, v := range vals {
		row[k] = v
	}
	t.g.rows = append(t.g.rows, row)
	t.es.added++
}

// pasteRows adds the clipboard's rows to the table as new rows, their cells
// going to the columns shown, in order.
func (t *tableTab) pasteRows(text string) {
	rows := clipboardRows(text)
	if len(rows) == 0 {
		t.w.sh.toastWarn(locales.TableNothingToPaste())
		return
	}
	if !t.writable(len(t.es.orig)) {
		return
	}
	for _, cells := range rows {
		vals := map[string]any{}
		for i, v := range cells {
			if i >= len(t.g.vis) {
				break
			}
			vals[t.g.cols[t.g.vis[i]].Name] = clipboardValue(v)
		}
		t.newRow(vals)
	}
	t.g.chosen.Clear()
	t.g.list.ScrollToEnd()
	t.w.sh.toastOK(locales.TablePasted(len(rows)))
}

// pasteCells writes the clipboard into the cells from the cursor cell on:
// down the rows and across the columns shown.
func (t *tableTab) pasteCells(text string) {
	cells := clipboardRows(text)
	row, colName, ok := t.cursorCell()
	if len(cells) == 0 || !ok {
		t.w.sh.toastWarn(locales.TableNothingToPaste())
		return
	}
	startCol := slices.IndexFunc(t.g.vis, func(i int) bool { return t.g.cols[i].Name == colName })
	for dr, line := range cells {
		r := row + dr
		if r >= len(t.g.rows) || !t.writable(r) {
			break
		}
		for dc, v := range line {
			ci := startCol + dc
			if ci >= len(t.g.vis) {
				break
			}
			t.setCell(r, t.g.cols[t.g.vis[ci]].Name, clipboardValue(v))
		}
	}
}

// duplicateRows adds a copy of each chosen row as a new row; the primary key
// is left for the database to give.
func (t *tableTab) duplicateRows() {
	var rows []int
	for r := range t.g.chosen.All() {
		if r >= 0 && r < len(t.g.rows) {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		t.w.sh.toastWarn(locales.EditNothingSelected())
		return
	}
	if !t.writable(len(t.es.orig)) {
		return
	}
	slices.Sort(rows)
	t.g.chosen.Clear()
	for _, r := range rows {
		vals := map[string]any{}
		for k, v := range t.g.rows[r] {
			if slices.Contains(t.es.pk, k) {
				continue
			}
			vals[k] = v
		}
		t.newRow(vals)
		t.g.chosen.Add(len(t.g.rows) - 1)
	}
	t.g.list.ScrollToEnd()
}

// ---- Set Value ----

// quickCell is the cell the Set Value menu (⌥Return) is open for.
type quickCell struct {
	row int
	col string
}

// quickValues are the values the menu offers for a column of this type.
func quickValues(typ string) []string {
	out := []string{"NULL", "DEFAULT", "EMPTY"}
	t := strings.ToUpper(typ)
	if strings.Contains(t, "DATE") || strings.Contains(t, "TIME") {
		out = append(out, "NOW()")
	}
	return out
}

// setQuickValue sets a cell to NULL, its default, an empty string or now.
func (t *tableTab) setQuickValue(row int, col, what string) {
	if !t.writable(row) || row >= len(t.g.rows) {
		return
	}
	switch what {
	case "NULL":
		t.setCell(row, col, nil)
	case "EMPTY":
		t.setCell(row, col, "")
	case "DEFAULT":
		if v, ok := t.es.defaults[col]; ok {
			t.setCell(row, col, v)
		} else {
			t.setCell(row, col, nil)
		}
	case "NOW()":
		t.setCell(row, col, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// quickDialog is the Set Value menu: the values to put in the cell.
func (t *tableTab) quickDialog(c *ui.Context) {
	q := t.quick
	if q == nil {
		return
	}
	sh := t.w.sh
	p := sh.tokens(c)
	open := true
	typ := ""
	if i := t.g.colIndex(q.col); i >= 0 {
		typ = t.g.cols[i].Type
	}
	sh.dialog(c, &open, dlgOpts{title: locales.TableSetValue(), sub: q.col, width: 300}, func() {
		for _, v := range quickValues(typ) {
			v := v
			row := ui.Row(c.Key("qv-"+v)).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
			if row.Hovered() {
				row.Background(p.hover)
			}
			row.Children(func() { ui.Text(c, v).Font(mono) })
			if row.Clicked() {
				t.setQuickValue(q.row, q.col, v)
				t.quick = nil
			}
		}
	}, nil)
	if !open {
		t.quick = nil
	}
}

// setValueMenu adds the Set Value submenu to a cell's context menu.
func (t *tableTab) setValueMenu(m *ui.Menu, row int, col string) {
	typ := ""
	if i := t.g.colIndex(col); i >= 0 {
		typ = t.g.cols[i].Type
	}
	m.Submenu(locales.TableSetValue(), func(m *ui.Menu) {
		for _, v := range quickValues(typ) {
			if m.Item(v).Chosen() {
				t.setQuickValue(row, col, v)
			}
		}
	})
}

var _ = fmt.Sprint
