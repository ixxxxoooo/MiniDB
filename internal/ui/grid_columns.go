package ui

import (
	"maps"
	"slices"
	"sort"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// 网格的列布局：宽度、顺序（表头拖动）、隐藏、固定。表数据页按「连接+库+表」记住它，下次打开还原。
//
// 固定的列移到最左并锁定位置和宽度（ui.TableColumn.Fixed）。mygo 的表格还不支持横向滚动时
// 把列钉在屏幕上，所以固定的列会随表格一起横向滚动。

// colLayout 是可保存的列布局。列用名字标识。
type colLayout struct {
	Order  []string           `json:"order,omitempty"`  // 用户拖出来的顺序
	Widths map[string]float32 `json:"widths,omitempty"` // 用户拖出来的宽度
	Hidden []string           `json:"hidden,omitempty"`
	Pinned []string           `json:"pinned,omitempty"`
}

// equal 报告两份布局是否一样。
func (l colLayout) equal(o colLayout) bool {
	return slices.Equal(l.Order, o.Order) && slices.Equal(l.Hidden, o.Hidden) && slices.Equal(l.Pinned, o.Pinned) && maps.Equal(l.Widths, o.Widths)
}

// layout 是网格现在的布局；只含还在结果里的列，没动过时各项都是空。
func (g *resultGrid) layout() colLayout {
	var l colLayout
	for _, id := range g.list.Columns.Order {
		if g.colIndex(id) >= 0 {
			l.Order = append(l.Order, id)
		}
	}
	for id, w := range g.list.Columns.Widths {
		if g.colIndex(id) >= 0 {
			if l.Widths == nil {
				l.Widths = map[string]float32{}
			}
			l.Widths[id] = w
		}
	}
	for name := range g.hidden {
		l.Hidden = append(l.Hidden, name)
	}
	for name := range g.pinned {
		l.Pinned = append(l.Pinned, name)
	}
	sort.Strings(l.Hidden)
	sort.Strings(l.Pinned)
	return l
}

// applyLayout 把保存的布局套到网格上，忽略已经不存在的列；至少留一列显示。
func (g *resultGrid) applyLayout(l colLayout) {
	g.hidden, g.pinned = nil, nil
	for _, name := range l.Hidden {
		if g.colIndex(name) >= 0 && len(g.hidden)+1 < len(g.cols) {
			if g.hidden == nil {
				g.hidden = map[string]bool{}
			}
			g.hidden[name] = true
		}
	}
	for _, name := range l.Pinned {
		if g.colIndex(name) >= 0 {
			if g.pinned == nil {
				g.pinned = map[string]bool{}
			}
			g.pinned[name] = true
		}
	}
	tl := ui.TableLayout{}
	for _, id := range l.Order {
		if g.colIndex(id) >= 0 {
			tl.Order = append(tl.Order, id)
		}
	}
	for id, w := range l.Widths {
		if g.colIndex(id) >= 0 && w > 0 {
			if tl.Widths == nil {
				tl.Widths = map[string]float32{}
			}
			tl.Widths[id] = w
		}
	}
	g.list.Columns = tl
}

// trackLayout 在布局变了时通知调用方去保存（拖动表头、固定、隐藏、重置之后调用）。
func (g *resultGrid) trackLayout() {
	l := g.layout()
	if l.equal(g.lastLayout) {
		return
	}
	g.lastLayout = l
	if g.onLayout != nil {
		g.onLayout(l)
	}
}

// displayOrder 是所有列现在的排列：固定的在最前，其余按用户拖出的顺序，没拖过的接在后面。
// 含隐藏的列，列管理对话框按它画。
func (g *resultGrid) displayOrder() []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, c := range g.cols {
		if g.pinned[c.Name] {
			add(c.Name)
		}
	}
	for _, id := range g.list.Columns.Order {
		if g.colIndex(id) >= 0 && !g.pinned[id] {
			add(id)
		}
	}
	for _, c := range g.cols {
		add(c.Name)
	}
	return out
}

// moveColumn 把一列在显示顺序里往前（d=-1）或往后（d=1）挪一格；固定的列不动，也不会被挪到固定列前面。
func (g *resultGrid) moveColumn(name string, d int) {
	if g.pinned[name] {
		return
	}
	order := g.displayOrder()
	i := slices.Index(order, name)
	j := i + d
	if i < 0 || j < len(g.pinned) || j >= len(order) {
		return
	}
	order[i], order[j] = order[j], order[i]
	g.list.Columns = ui.TableLayout{Order: order, Widths: g.list.Columns.Widths}
	g.rebuild()
	g.trackLayout()
}

// togglePin 固定或取消固定一列。
func (g *resultGrid) togglePin(name string) {
	if g.pinned == nil {
		g.pinned = map[string]bool{}
	}
	if g.pinned[name] {
		delete(g.pinned, name)
	} else {
		g.pinned[name] = true
	}
	g.rebuild()
	g.trackLayout()
}

// resetLayout 把宽度、顺序、隐藏和固定都还原成默认。
func (g *resultGrid) resetLayout() {
	g.hidden, g.pinned = nil, nil
	g.list.Columns = ui.TableLayout{}
	g.rebuild()
	for i := range g.uiCols[1:] {
		g.uiCols[i+1].Width = 72 // 从最小宽度重新按内容量
	}
	g.fitWidths()
	g.chosen.Clear()
	g.trackLayout()
}

// headerMenu 是列头的右键菜单：固定、隐藏、列管理。col 是 g.uiCols 里的下标（0 是行号，-1 是列头右边的空白）。
func (g *resultGrid) headerMenu(m *ui.Menu, col int) {
	if col > 0 && col <= len(g.vis) {
		name := g.col(col).Name
		label := locales.ColPin()
		if g.pinned[name] {
			label = locales.ColUnpin()
		}
		if m.Item(label).Chosen() {
			defer g.togglePin(name)
		}
		if m.Item(locales.ColHide()).Disabled(len(g.vis) <= 1).Chosen() {
			defer g.setHidden(name, true)
		}
		m.Separator()
	}
	if g.onManage != nil && m.Item(locales.ColManage()).Chosen() {
		defer g.onManage()
	}
}

// revealReq 是待办的「滚到这个格子」：行建出来之后，让它横向滚进视野。
type revealReq struct {
	row  int
	name string
}

// jumpTo 跳到某一列：选中当前行（没有选中就第一行）的这一格，并横向滚到它。
func (g *resultGrid) jumpTo(row int, name string) {
	if g.colIndex(name) < 0 || g.hidden[name] || len(g.rows) == 0 {
		return
	}
	row = min(max(row, 0), len(g.rows)-1)
	g.chosen.Clear()
	g.chosen.Add(row)
	g.list.Cells.Cursor, g.list.Cells.Anchor, g.list.Cells.All = name, name, false
	g.list.ScrollIntoView(row)
	g.reveal = &revealReq{row: row, name: name}
}

// markReveal 在要滚到的格子里放一个看不见的标记并让它滚进视野；由格子的建造函数调用。
func (g *resultGrid) markReveal(c *ui.Context, row, col int) {
	r := g.reveal
	if r == nil || r.row != row || g.colIndex(r.name) < 0 || g.col(col).Name != r.name {
		return
	}
	g.reveal = nil
	ui.Box(c.Key("reveal")).Absolute().Top(0).Left(0).Width(g.colWidth(col)).Height(gridRowH).ScrollIntoView()
}

// fuzzyColumns 按输入模糊匹配列名，最像的在前：前缀、连续、靠前的字符得分高，字符要按顺序出现。
// 输入为空时按原顺序返回全部。
func fuzzyColumns(names []string, q string) []string {
	if q == "" {
		return names
	}
	type hit struct {
		name  string
		score int
	}
	var hits []hit
	for _, n := range names {
		if s, ok := fuzzyScore(n, q); ok {
			hits = append(hits, hit{n, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.name
	}
	return out
}

// fuzzyScore 是 q 在 name 里按顺序出现的得分（不区分大小写）；没有全部出现返回 false。
func fuzzyScore(name, q string) (int, bool) {
	nr, qr := []rune(name), []rune(q)
	lower := func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}
	score, at, prev := 0, 0, -2
	for _, want := range qr {
		found := false
		for ; at < len(nr); at++ {
			if lower(nr[at]) != lower(want) {
				continue
			}
			found = true
			score += 10
			if at == prev+1 {
				score += 15 // 连续
			}
			if at == 0 {
				score += 20 // 开头
			} else if nr[at-1] == '_' || nr[at-1] == ' ' || (nr[at] >= 'A' && nr[at] <= 'Z' && nr[at-1] >= 'a' && nr[at-1] <= 'z') {
				score += 10 // 单词开头
			}
			prev = at
			at++
			break
		}
		if !found {
			return 0, false
		}
	}
	return score - len(nr) + len(qr), true
}
