package ui

import "testing"

// @author ixxxxoooo

// totalWidth 是这一帧所有列的实际宽度之和（被补满的那一列取撑开后的宽度）。
func totalWidth(g *resultGrid) float32 {
	var sum float32
	for i := range g.uiCols {
		sum += g.colWidth(i)
	}
	return sum
}

func TestFillStretchesLastColumnWhenNarrow(t *testing.T) {
	g := testGrid("a", "b", "c")
	g.tableW = 2000
	cols := g.fillCols()
	last := len(cols) - 1
	if cols[last].Width != 0 || g.fillAt != last+1 {
		t.Fatalf("富余时最后一列应被撑开: width=%v fillAt=%d", cols[last].Width, g.fillAt)
	}
	if got, want := totalWidth(g), g.tableW-gridScrollTrack; got != want {
		t.Fatalf("各列加起来应正好补满表格: %v != %v", got, want)
	}
	if g.uiCols[last].Width == 0 {
		t.Fatal("g.uiCols 里的自然宽度不能被改掉")
	}
}

func TestFillLeavesWideTableAlone(t *testing.T) {
	g := testGrid("a", "b", "c")
	g.tableW = 100
	cols := g.fillCols()
	if g.fillAt != 0 || cols[len(cols)-1].Width == 0 {
		t.Fatalf("内容比表格宽时不撑开: fillAt=%d", g.fillAt)
	}
	g.tableW = 0
	g.fillCols()
	if g.fillAt != 0 {
		t.Fatal("还没量到表格宽度时不撑开")
	}
}

func TestFillRespectsDraggedWidth(t *testing.T) {
	g := testGrid("a", "b", "c")
	g.tableW = 2000
	g.list.Columns.Widths = map[string]float32{"c": 200}
	g.fillCols()
	if g.fillAt != 0 {
		t.Fatal("用户拖过最后一列的宽度后不再撑它")
	}
}

func TestFillFollowsDisplayOrder(t *testing.T) {
	g := testGrid("a", "b", "c")
	g.list.Columns.Order = []string{"c", "a", "b"}
	g.tableW = 2000
	g.fillCols()
	if g.fillAt == 0 || g.uiCols[g.fillAt-1].ID != "b" {
		t.Fatalf("应撑显示顺序里最右的列 b: fillAt=%d", g.fillAt)
	}
}

func TestShortType(t *testing.T) {
	for in, want := range map[string]string{
		"VARCHAR(255)": "varchar", "int": "int", "": "", "  Numeric(10,2) ": "numeric",
		"timestamp with time zone": "timestamp wi", "(x)": "(x)",
	} {
		if got := shortType(in); got != want {
			t.Errorf("shortType(%q) = %q, 想要 %q", in, got, want)
		}
	}
}

// nameColX 是测试库 users 表第一行 name 列里的一个横坐标（表头带类型标签后列比以前宽）。
const nameColX = 500
