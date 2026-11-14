package ui

import (
	"slices"
	"strings"

	"minidb/locales"
)

// 数据网格的粘贴、复制行与新增行。

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
	if s == sqlNull {
		return nil
	}
	return s
}
