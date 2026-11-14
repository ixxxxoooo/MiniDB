package model

import "sort"

// ER 图的数据：表是节点，外键是边。只描述关系，不含布局（布局在界面层）。

// ERColumn 是节点里的一列。
type ERColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
	PK   bool   `json:"pk"`
	FK   bool   `json:"fk"` // 这一列是某个外键的一部分
}

// ERNode 是一张表。
type ERNode struct {
	Name    string     `json:"name"`
	Comment string     `json:"comment,omitempty"`
	Columns []ERColumn `json:"columns"`
}

// EREdge 是一个外键：From 表的 FromCols 引用 To 表的 ToCols。
type EREdge struct {
	Name     string   `json:"name,omitempty"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	FromCols []string `json:"fromCols"`
	ToCols   []string `json:"toCols"`
	OnDelete string   `json:"onDelete,omitempty"`
	OnUpdate string   `json:"onUpdate,omitempty"`
}

// ERGraph 是一个库的表与外键关系。
type ERGraph struct {
	Nodes []ERNode `json:"nodes"`
	Edges []EREdge `json:"edges"`
}

// Node 按表名取节点。
func (g *ERGraph) Node(name string) (ERNode, bool) {
	for _, n := range g.Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return ERNode{}, false
}

// Neighborhood 返回 focus 表和它向外 depth 层相连的表（不分外键方向），以及它们之间的边。
// depth 小于 0 返回整张图；focus 不存在时返回空图。结果里节点和边的顺序同原图。
func (g *ERGraph) Neighborhood(focus string, depth int) *ERGraph {
	if depth < 0 {
		return g
	}
	if _, ok := g.Node(focus); !ok {
		return &ERGraph{}
	}
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	keep := map[string]bool{focus: true}
	frontier := []string{focus}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, n := range frontier {
			for _, m := range adj[n] {
				if !keep[m] {
					keep[m] = true
					next = append(next, m)
				}
			}
		}
		frontier = next
	}
	out := &ERGraph{}
	for _, n := range g.Nodes {
		if keep[n.Name] {
			out.Nodes = append(out.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if keep[e.From] && keep[e.To] {
			out.Edges = append(out.Edges, e)
		}
	}
	return out
}

// Degree 是每张表连着几条边（自引用算一条）。
func (g *ERGraph) Degree() map[string]int {
	d := map[string]int{}
	for _, e := range g.Edges {
		d[e.From]++
		if e.To != e.From {
			d[e.To]++
		}
	}
	return d
}

// Sorted 按表名排序节点、按 From、To、名称排序边，让同样的库每次得到同样的图。
func (g *ERGraph) Sorted() {
	sort.SliceStable(g.Nodes, func(i, j int) bool { return g.Nodes[i].Name < g.Nodes[j].Name })
	sort.SliceStable(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Name < b.Name
	})
}
