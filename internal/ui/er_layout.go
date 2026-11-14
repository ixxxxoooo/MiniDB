package ui

import (
	"sort"

	"minidb/internal/model"
)

// ER 图的布局：分层为主（被引用的表在左，引用它的表在右），层内用重心法减少交叉，
// 孤立的表排在下面，最后用一次推开处理重叠。同样的图每次得到同样的位置（所有排序都有确定的次序）。
// 这一文件不依赖界面库，可以单独测试。

// 节点的尺寸与间距（世界坐标）。
const (
	erHeadH   = 28  // 表头高度
	erRowH    = 19  // 每列一行
	erPadX    = 10  // 左右内边距
	erMinW    = 150 // 最窄
	erMaxW    = 320 // 最宽
	erGapX    = 110 // 层与层之间
	erGapY    = 34  // 同一层里节点之间
	erFont    = 12  // 列的字号
	erMaxRows = 40  // 一个节点最多画几列，多的折叠成「… 还有 N 列」
)

// erRect 是一个节点占的矩形。
type erRect struct{ X, Y, W, H float32 }

func (r erRect) overlaps(o erRect, margin float32) bool {
	return r.X < o.X+o.W+margin && o.X < r.X+r.W+margin && r.Y < o.Y+o.H+margin && o.Y < r.Y+r.H+margin
}

// erRows 是节点里实际画出来的列：太多时主键和外键优先，其余按原顺序补满。
func erRows(n model.ERNode) (rows []model.ERColumn, hidden int) {
	if len(n.Columns) <= erMaxRows {
		return n.Columns, 0
	}
	keep := make([]bool, len(n.Columns))
	left := erMaxRows
	for i, c := range n.Columns {
		if (c.PK || c.FK) && left > 0 {
			keep[i], left = true, left-1
		}
	}
	for i := range n.Columns {
		if !keep[i] && left > 0 {
			keep[i], left = true, left-1
		}
	}
	for i, c := range n.Columns {
		if keep[i] {
			rows = append(rows, c)
		}
	}
	return rows, len(n.Columns) - len(rows)
}

// erSize 估算节点的宽高：宽按表名和各列「名称  类型」里最长的算，夹在 [erMinW, erMaxW]。
func erSize(n model.ERNode) (w, h float32) {
	w = textWidth(n.Name, 13) + 2*erPadX + 8
	rows, hidden := erRows(n)
	for _, c := range rows {
		w = max(w, textWidth(c.Name, erFont)+textWidth(c.Type, erFont-1)+3*erPadX+22)
	}
	lines := len(rows)
	if hidden > 0 {
		lines++
	}
	return min(max(w, erMinW), erMaxW), erHeadH + float32(lines)*erRowH + 6
}

// erAnchorY 是节点里某一列的纵向中心（世界坐标，相对节点顶部）；列不在画出的行里就是表头中心。
func erAnchorY(n model.ERNode, col string) float32 {
	rows, _ := erRows(n)
	for i, c := range rows {
		if c.Name == col {
			return erHeadH + float32(i)*erRowH + erRowH/2
		}
	}
	return erHeadH / 2
}

// erLayers 给每张表定一层：被引用的在前（层号小）。环上的边（含自引用）忽略，保证有解。
func erLayers(g *model.ERGraph) map[string]int {
	parents := map[string][]string{} // 表 → 它引用的表
	for _, e := range g.Edges {
		if e.From != e.To {
			parents[e.From] = append(parents[e.From], e.To)
		}
	}
	names := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	for _, ps := range parents {
		sort.Strings(ps)
	}
	layer := map[string]int{}
	state := map[string]int{} // 0 未访问，1 访问中，2 完成
	var visit func(string) int
	visit = func(n string) int {
		if state[n] == 2 {
			return layer[n]
		}
		state[n] = 1
		l := 0
		for _, p := range parents[n] {
			if state[p] == 1 {
				continue // 回边：环，忽略
			}
			l = max(l, visit(p)+1)
		}
		state[n], layer[n] = 2, l
		return l
	}
	for _, n := range names {
		visit(n)
	}
	return layer
}

// erLayout 算出每张表左上角的位置。
func erLayout(g *model.ERGraph) map[string]erRect {
	sizes := map[string]erRect{}
	for _, n := range g.Nodes {
		w, h := erSize(n)
		sizes[n.Name] = erRect{W: w, H: h}
	}
	linked := map[string]bool{}
	for _, e := range g.Edges {
		if e.From != e.To {
			linked[e.From], linked[e.To] = true, true
		}
	}
	layer := erLayers(g)
	var cols [][]string
	for _, n := range g.Nodes {
		if !linked[n.Name] {
			continue
		}
		l := layer[n.Name]
		for len(cols) <= l {
			cols = append(cols, nil)
		}
		cols[l] = append(cols[l], n.Name)
	}
	for _, c := range cols {
		sort.Strings(c)
	}
	erReduceCrossings(g, cols)

	// 每层一列，层内自上而下；各层按最高的一层居中。
	colH := make([]float32, len(cols))
	colW := make([]float32, len(cols))
	for i, c := range cols {
		for k, n := range c {
			colH[i] += sizes[n].H
			if k > 0 {
				colH[i] += erGapY
			}
			colW[i] = max(colW[i], sizes[n].W)
		}
	}
	var tallest, x float32
	for _, h := range colH {
		tallest = max(tallest, h)
	}
	for i, c := range cols {
		y := (tallest - colH[i]) / 2
		for _, n := range c {
			r := sizes[n]
			r.X, r.Y = x, y
			sizes[n] = r
			y += r.H + erGapY
		}
		x += colW[i] + erGapX
	}
	// 孤立的表排在下面，一行排不下就换行，行宽不超过上面那一块（至少 1200）。
	rowW := max(x-erGapX, 1200)
	y0 := tallest + erGapY*2
	if len(cols) == 0 {
		y0 = 0
	}
	var cx, rowH float32
	for _, n := range g.Nodes {
		if linked[n.Name] {
			continue
		}
		r := sizes[n.Name]
		if cx > 0 && cx+r.W > rowW {
			cx, y0, rowH = 0, y0+rowH+erGapY, 0
		}
		r.X, r.Y = cx, y0
		sizes[n.Name] = r
		cx += r.W + erGapY
		rowH = max(rowH, r.H)
	}
	erResolve(g, sizes)
	return sizes
}
