package ui

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 数据网格的「补满」：各列加起来比表格窄时，让最后一列把多出来的宽度吃掉，
// 右边不再留一片空白（设计稿 A 的数据页就是这样）。列宽仍按内容估算，只是在富余时才撑开；
// 内容比表格宽时一切照旧，靠横向滚动条看。用户拖过最后一列的宽度后，就尊重他的选择，不再撑。

// gridScrollTrack 是表格右侧纵向滚动条占的轨道宽度：补满时要把它让出来，否则会凭空多出横向滚动条。
const gridScrollTrack = 14

// fillCols 返回这一帧交给 ui.Table 的列。富余时最后一个可伸缩的列宽度置 0（吃掉剩余空间），
// 同时记下它的位置和实际宽度，单元格内容按 colWidth 取到的就是这个宽度。
// 返回的是副本：g.uiCols 里保存的始终是按内容估出来的自然宽度，翻页、重排都以它为准。
func (g *resultGrid) fillCols() []ui.TableColumn {
	g.fillAt = 0
	if g.plain { // 查询结果：列宽按比例撑满整行，和 HTML 表格的自动布局一样（设计稿）
		g.stretchCols()
		return g.uiCols
	}
	last := g.lastFlexCol()
	if last < 0 || g.tableW <= 0 {
		return g.uiCols
	}
	if _, dragged := g.list.Columns.Widths[g.uiCols[last].ID]; dragged {
		return g.uiCols
	}
	var others float32
	for i := range g.uiCols {
		if i != last {
			others += g.colWidth(i)
		}
	}
	avail := g.tableW - gridScrollTrack
	if others+g.uiCols[last].Width >= avail {
		return g.uiCols
	}
	cols := slices.Clone(g.uiCols)
	cols[last].Width = 0
	g.fillAt, g.fillW = last+1, avail-others
	return cols
}

// lastFlexCol 是现在排在最右边、可以伸缩的那一列在 uiCols 里的下标（固定的列、行号列不算）。
// 用户拖过列序时，以显示顺序为准而不是 uiCols 的顺序。没有这样的列返回 -1。
func (g *resultGrid) lastFlexCol() int {
	order := g.displayOrder()
	for k := len(order) - 1; k >= 0; k-- {
		name := order[k]
		if g.hidden[name] || g.pinned[name] {
			continue
		}
		for i, col := range g.uiCols {
			if col.ID == name && !col.Fixed && !col.RowHeader {
				return i
			}
		}
	}
	return -1
}

// trackTableWidth 在绘制时记下表格这一帧的实际宽度；变了就请求再画一帧，让补满马上生效。
func (g *resultGrid) trackTableWidth(c *ui.Context, w float32) {
	if w == g.tableW {
		return
	}
	g.tableW = w
	c.Invalidate()
}

// stretchCols 在列宽加起来不满一行时，把多出来的宽度按各列现有宽度的比例分给所有可伸缩的列；
// 用户拖过任何一列的宽度就不再动；已经撑满或超出时不缩小（超出的部分横向滚动）。
func (g *resultGrid) stretchCols() {
	if g.tableW <= 0 || len(g.list.Columns.Widths) > 0 {
		return
	}
	var fixed, flex float32
	for i, col := range g.uiCols {
		if col.Fixed {
			fixed += g.colWidth(i)
		} else {
			flex += g.colWidth(i)
		}
	}
	avail := g.tableW - gridScrollTrack
	if flex <= 0 || fixed+flex >= avail-0.5 {
		return
	}
	scale := (avail - fixed) / flex
	for i := range g.uiCols {
		if !g.uiCols[i].Fixed {
			g.uiCols[i].Width *= scale
		}
	}
}
