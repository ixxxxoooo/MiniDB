package ui

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// 结果网格的选区与复制。

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
