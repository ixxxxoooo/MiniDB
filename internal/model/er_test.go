package model

import (
	"reflect"
	"testing"
)

func names(g *ERGraph) []string {
	var out []string
	for _, n := range g.Nodes {
		out = append(out, n.Name)
	}
	return out
}

func sampleER() *ERGraph {
	g := &ERGraph{}
	for _, n := range []string{"a", "b", "c", "d", "e", "iso"} {
		g.Nodes = append(g.Nodes, ERNode{Name: n})
	}
	// a ← b ← c ← d，e 自引用并引用 a
	g.Edges = []EREdge{{From: "b", To: "a"}, {From: "c", To: "b"}, {From: "d", To: "c"}, {From: "e", To: "e"}, {From: "e", To: "a"}}
	return g
}

func TestNeighborhood(t *testing.T) {
	g := sampleER()
	cases := []struct {
		focus string
		depth int
		want  []string
	}{
		{"a", 0, []string{"a"}},
		{"a", 1, []string{"a", "b", "e"}},
		{"a", 2, []string{"a", "b", "c", "e"}},
		{"a", 9, []string{"a", "b", "c", "d", "e"}},
		{"d", 1, []string{"c", "d"}},
		{"iso", 3, []string{"iso"}},
		{"nope", 1, nil},
	}
	for _, c := range cases {
		if got := names(g.Neighborhood(c.focus, c.depth)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s/%d: got %v want %v", c.focus, c.depth, got, c.want)
		}
	}
	if sub := g.Neighborhood("a", 1); len(sub.Edges) != 3 { // b→a、e→a、e→e
		t.Errorf("子图的边应只含两端都在内的: %+v", sub.Edges)
	}
	if g.Neighborhood("a", -1) != g {
		t.Error("depth<0 应返回整张图")
	}
}

func TestERDegreeAndSorted(t *testing.T) {
	g := sampleER()
	d := g.Degree()
	if d["a"] != 2 || d["e"] != 2 || d["iso"] != 0 {
		t.Errorf("度数: %v", d)
	}
	g.Nodes[0], g.Nodes[3] = g.Nodes[3], g.Nodes[0]
	g.Sorted()
	if got := names(g); !reflect.DeepEqual(got, []string{"a", "b", "c", "d", "e", "iso"}) {
		t.Errorf("排序: %v", got)
	}
}
