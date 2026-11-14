package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// 结果网格的数据装载、列宽估算与列布局重建。

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
