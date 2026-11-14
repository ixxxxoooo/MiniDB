package ui

import (
	"fmt"
	"reflect"
	"testing"

	"minidb/internal/model"
)

func erTestGraph(edges ...[2]string) *model.ERGraph {
	seen := map[string]bool{}
	g := &model.ERGraph{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			g.Nodes = append(g.Nodes, model.ERNode{Name: n, Columns: []model.ERColumn{{Name: "id", Type: "int", PK: true}}})
		}
	}
	for _, e := range edges {
		add(e[0])
		add(e[1])
		g.Edges = append(g.Edges, model.EREdge{From: e[0], To: e[1], FromCols: []string{"id"}, ToCols: []string{"id"}})
	}
	g.Sorted()
	return g
}

func TestERLayersParentsLeft(t *testing.T) {
	// orders → users、items → orders、items → products：被引用的在左
	g := erTestGraph([2]string{"orders", "users"}, [2]string{"items", "orders"}, [2]string{"items", "products"})
	l := erLayers(g)
	if l["users"] != 0 || l["products"] != 0 || l["orders"] != 1 || l["items"] != 2 {
		t.Fatalf("层: %v", l)
	}
	pos := erLayout(g)
	if !(pos["users"].X < pos["orders"].X && pos["orders"].X < pos["items"].X) {
		t.Fatalf("被引用的表应在左边: %+v", pos)
	}
}

func TestERLayersCycleAndSelfReference(t *testing.T) {
	// a → b → c → a 成环，d 自引用：不能死循环，每张表都有位置
	g := erTestGraph([2]string{"a", "b"}, [2]string{"b", "c"}, [2]string{"c", "a"}, [2]string{"d", "d"})
	l := erLayers(g)
	if len(l) != 4 {
		t.Fatalf("层: %v", l)
	}
	pos := erLayout(g)
	if len(pos) != 4 {
		t.Fatalf("位置: %v", pos)
	}
	for n, r := range pos {
		if r.W <= 0 || r.H <= 0 {
			t.Errorf("%s 尺寸: %+v", n, r)
		}
	}
	assertNoOverlap(t, pos)
}

func assertNoOverlap(t *testing.T, pos map[string]erRect) {
	t.Helper()
	for a, ra := range pos {
		for b, rb := range pos {
			if a < b && ra.overlaps(rb, 0) {
				t.Errorf("%s 与 %s 重叠: %+v %+v", a, b, ra, rb)
			}
		}
	}
}

func TestERLayoutDeterministicAndNoOverlap(t *testing.T) {
	var edges [][2]string
	for i := 0; i < 30; i++ {
		edges = append(edges, [2]string{fmt.Sprintf("t%02d", i), fmt.Sprintf("t%02d", (i*7+3)%30)})
	}
	g := erTestGraph(edges...)
	for i := 0; i < 5; i++ { // 孤立的表
		g.Nodes = append(g.Nodes, model.ERNode{Name: fmt.Sprintf("iso%d", i), Columns: []model.ERColumn{{Name: "id", Type: "int"}}})
	}
	g.Sorted()
	first := erLayout(g)
	for i := 0; i < 5; i++ {
		if !reflect.DeepEqual(first, erLayout(g)) {
			t.Fatal("同样的图应得到同样的布局")
		}
	}
	if len(first) != 35 {
		t.Fatalf("节点数: %d", len(first))
	}
	assertNoOverlap(t, first)
	// 孤立的表在连通部分下面
	b, _ := erBounds(map[string]erRect{"t00": first["t00"]})
	if first["iso0"].Y <= b.Y {
		t.Errorf("孤立的表应排在下方: %+v", first["iso0"])
	}
}

func TestERResolvePushesApart(t *testing.T) {
	g := erTestGraph([2]string{"a", "b"})
	rects := map[string]erRect{"a": {X: 0, Y: 0, W: 100, H: 50}, "b": {X: 20, Y: 10, W: 100, H: 50}}
	erResolve(g, rects)
	if rects["a"].overlaps(rects["b"], 0) {
		t.Fatalf("应被推开: %+v", rects)
	}
}

func TestERSizeAndRows(t *testing.T) {
	n := model.ERNode{Name: "t"}
	for i := 0; i < 100; i++ {
		n.Columns = append(n.Columns, model.ERColumn{Name: fmt.Sprintf("c%d", i), Type: "int", PK: i == 77, FK: i == 90})
	}
	rows, hidden := erRows(n)
	if len(rows) != erMaxRows || hidden != 100-erMaxRows {
		t.Fatalf("rows=%d hidden=%d", len(rows), hidden)
	}
	has := map[string]bool{}
	for _, r := range rows {
		has[r.Name] = true
	}
	if !has["c77"] || !has["c90"] {
		t.Fatal("太多列时主键、外键要留下")
	}
	_, h := erSize(n)
	if h != erHeadH+float32(erMaxRows+1)*erRowH+6 {
		t.Fatalf("高度应含「还有 N 列」一行: %v", h)
	}
	w, _ := erSize(model.ERNode{Name: "very_long_table_name_" + string(make([]byte, 0)), Columns: []model.ERColumn{{Name: "a_very_long_column_name_that_keeps_going_and_going_and_going", Type: "varchar(255)"}}})
	if w != erMaxW {
		t.Fatalf("宽度封顶: %v", w)
	}
	if y := erAnchorY(n, "c77"); y <= erHeadH {
		t.Fatalf("锚点: %v", y)
	}
	if y := erAnchorY(n, "nope"); y != erHeadH/2 {
		t.Fatalf("不存在的列取表头中心: %v", y)
	}
}
