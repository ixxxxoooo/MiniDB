package ui

import (
	"sort"

	"minidb/internal/model"
)

// ER 图布局的后半段：同层内减少边交叉、推开重叠、求整体范围。

// erReduceCrossings 用重心法调整层内次序：每层按相邻层里邻居的平均位置排，上下各扫几遍。
func erReduceCrossings(g *model.ERGraph, cols [][]string) {
	pos := map[string]int{}
	index := func() {
		for _, c := range cols {
			for i, n := range c {
				pos[n] = i
			}
		}
	}
	index()
	nbr := map[string][]string{}
	for _, e := range g.Edges {
		if e.From != e.To {
			nbr[e.From] = append(nbr[e.From], e.To)
			nbr[e.To] = append(nbr[e.To], e.From)
		}
	}
	layerOf := map[string]int{}
	for i, c := range cols {
		for _, n := range c {
			layerOf[n] = i
		}
	}
	sweep := func(i, from int) {
		type kv struct {
			name string
			bary float64
			idx  int
		}
		items := make([]kv, len(cols[i]))
		for k, n := range cols[i] {
			sum, cnt := 0.0, 0
			for _, m := range nbr[n] {
				if layerOf[m] == from {
					sum += float64(pos[m])
					cnt++
				}
			}
			b := float64(k)
			if cnt > 0 {
				b = sum / float64(cnt)
			}
			items[k] = kv{n, b, k}
		}
		sort.SliceStable(items, func(a, b int) bool { return items[a].bary < items[b].bary })
		for k, it := range items {
			cols[i][k] = it.name
		}
		index()
	}
	for round := 0; round < 4; round++ {
		for i := 1; i < len(cols); i++ {
			sweep(i, i-1)
		}
		for i := len(cols) - 2; i >= 0; i-- {
			sweep(i, i+1)
		}
	}
}

// erResolve 把重叠的节点沿重叠较少的方向推开（按表名次序逐对处理，结果确定）。分层布局本身不重叠，
// 它兜底的是孤立表和用户拖到一起的情形。
func erResolve(g *model.ERGraph, rects map[string]erRect) {
	names := make([]string, 0, len(rects))
	for _, n := range g.Nodes {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	const margin = 12
	for iter := 0; iter < 40; iter++ {
		moved := false
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				a, b := rects[names[i]], rects[names[j]]
				if !a.overlaps(b, margin) {
					continue
				}
				dx := min(a.X+a.W, b.X+b.W) - max(a.X, b.X) + margin
				dy := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y) + margin
				if dx < dy {
					if a.X+a.W/2 <= b.X+b.W/2 {
						b.X += dx
					} else {
						b.X -= dx
					}
				} else {
					if a.Y+a.H/2 <= b.Y+b.H/2 {
						b.Y += dy
					} else {
						b.Y -= dy
					}
				}
				rects[names[j]] = b
				moved = true
			}
		}
		if !moved {
			return
		}
	}
}

// erBounds 是所有节点的外接矩形。
func erBounds(rects map[string]erRect) (b erRect, ok bool) {
	first := true
	var x1, y1 float32
	for _, r := range rects {
		if first {
			b.X, b.Y, x1, y1, first = r.X, r.Y, r.X+r.W, r.Y+r.H, false
			continue
		}
		b.X, b.Y = min(b.X, r.X), min(b.Y, r.Y)
		x1, y1 = max(x1, r.X+r.W), max(y1, r.Y+r.H)
	}
	b.W, b.H = x1-b.X, y1-b.Y
	return b, !first
}
