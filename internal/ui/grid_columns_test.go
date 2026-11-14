package ui

import (
	"slices"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

func testGrid(names ...string) *resultGrid {
	g := newResultGrid(true)
	var cols []model.ColumnMeta
	row := map[string]any{}
	for _, n := range names {
		cols = append(cols, model.ColumnMeta{Name: n, Type: "text"})
		row[n] = "v-" + n
	}
	g.set(cols, []map[string]any{row})
	return g
}

func visNames(g *resultGrid) []string {
	var out []string
	for i := range g.vis {
		out = append(out, g.col(i+1).Name)
	}
	return out
}

func TestPinMovesColumnToFront(t *testing.T) {
	g := testGrid("a", "b", "c", "d")
	g.togglePin("c")
	if !slices.Equal(visNames(g), []string{"c", "a", "b", "d"}) {
		t.Fatalf("固定的列应排最前: %v", visNames(g))
	}
	if !g.uiCols[1].Fixed || g.uiCols[2].Fixed {
		t.Fatalf("只有固定的列锁定: %+v", g.uiCols)
	}
	g.togglePin("d")
	if !slices.Equal(visNames(g), []string{"c", "d", "a", "b"}) {
		t.Fatalf("多个固定列按原顺序: %v", visNames(g))
	}
	g.togglePin("c")
	g.togglePin("d")
	if !slices.Equal(visNames(g), []string{"a", "b", "c", "d"}) {
		t.Fatalf("取消固定后还原: %v", visNames(g))
	}
}

func TestMoveColumnAndDisplayOrder(t *testing.T) {
	g := testGrid("a", "b", "c", "d")
	g.moveColumn("c", -1)
	if got := g.displayOrder(); !slices.Equal(got, []string{"a", "c", "b", "d"}) {
		t.Fatalf("上移: %v", got)
	}
	g.moveColumn("a", -1) // 已在最前，不动
	g.moveColumn("d", 1)  // 已在最后，不动
	if got := g.displayOrder(); !slices.Equal(got, []string{"a", "c", "b", "d"}) {
		t.Fatalf("越界不动: %v", got)
	}
	g.togglePin("d")
	g.moveColumn("a", -1) // 不能挪到固定列前面
	if got := g.displayOrder(); got[0] != "d" {
		t.Fatalf("固定列始终在最前: %v", got)
	}
	g.moveColumn("d", 1) // 固定列自己不能挪
	if got := g.displayOrder(); got[0] != "d" {
		t.Fatalf("固定列不能挪: %v", got)
	}
}

func TestLayoutRoundTrip(t *testing.T) {
	g := testGrid("a", "b", "c", "d")
	var saved []colLayout
	g.onLayout = func(l colLayout) { saved = append(saved, l) }
	g.togglePin("b")
	g.setHidden("d", true)
	g.moveColumn("c", -1)
	g.list.Columns.Widths = map[string]float32{"a": 150}
	g.trackLayout()
	if len(saved) < 4 {
		t.Fatalf("每次变化都应通知保存: %d", len(saved))
	}
	l := saved[len(saved)-1]
	if !slices.Equal(l.Pinned, []string{"b"}) || !slices.Equal(l.Hidden, []string{"d"}) || l.Widths["a"] != 150 {
		t.Fatalf("布局不对: %+v", l)
	}
	before := len(saved)
	g.trackLayout()
	if len(saved) != before {
		t.Fatalf("没变化不应再通知: %d → %d", before, len(saved))
	}

	// 另一个同样的网格装载保存的布局（含已经不存在的列，要被忽略）
	l.Hidden = append(l.Hidden, "gone")
	g2 := newResultGrid(true)
	g2.pending = &l
	var cols []model.ColumnMeta
	for _, n := range []string{"a", "b", "c", "d"} {
		cols = append(cols, model.ColumnMeta{Name: n, Type: "text"})
	}
	g2.set(cols, []map[string]any{{"a": 1, "b": 2, "c": 3, "d": 4}})
	if !slices.Equal(visNames(g2), visNames(g)) {
		t.Fatalf("还原后列的排列不一致: %v vs %v", visNames(g2), visNames(g))
	}
	if g2.hidden["gone"] || !g2.hidden["d"] || !g2.pinned["b"] {
		t.Fatalf("hidden=%v pinned=%v", g2.hidden, g2.pinned)
	}
	if g2.colWidth(slices.Index(visNames(g2), "a")+1) != 150 {
		t.Fatalf("保存的列宽应生效: %v", g2.list.Columns.Widths)
	}
	if g2.pending != nil {
		t.Fatal("布局只套用一次")
	}
	g2.resetLayout()
	if len(g2.hidden) != 0 || len(g2.pinned) != 0 || g2.list.Columns.Widths != nil || !slices.Equal(visNames(g2), []string{"a", "b", "c", "d"}) {
		t.Fatalf("重置应回到默认: %v", visNames(g2))
	}
}

func TestApplyLayoutKeepsOneColumn(t *testing.T) {
	g := testGrid("a", "b")
	g.pending = &colLayout{Hidden: []string{"a", "b"}}
	g.sig = ""
	g.set(g.cols, g.rows)
	if len(g.vis) != 1 {
		t.Fatalf("至少留一列: %v", g.vis)
	}
}

func TestFuzzyColumns(t *testing.T) {
	names := []string{"id", "user_id", "created_at", "updated_at", "email", "ClientName"}
	cases := map[string][]string{
		"":      names,
		"ua":    {"updated_at"},
		"cr":    {"created_at"},
		"ID":    {"id", "user_id"},
		"cn":    {"ClientName"},
		"zzz":   nil,
		"email": {"email"},
	}
	for q, want := range cases {
		got := fuzzyColumns(names, q)
		if q == "ua" {
			// 「ua」可以匹配 user_id 之外的多个；只要求 updated_at 在内且排第一
			if len(got) == 0 || got[0] != "updated_at" {
				t.Errorf("%q: %v", q, got)
			}
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %v want %v", q, got, want)
		}
	}
	if got := fuzzyColumns([]string{"user_id", "id"}, "id"); got[0] != "id" {
		t.Errorf("前缀、更短的名字应排前: %v", got)
	}
}

// 表数据页：拖过的列宽、固定、隐藏会按「连接+库+表」存下来，重新打开同一张表时还原。
func TestTableTabLayoutPersists(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	tt.g.togglePin("email")
	tt.g.setHidden("bio", true)
	tt.g.list.Columns = mui.TableLayout{Order: tt.g.list.Columns.Order, Widths: map[string]float32{"name": 222}}
	tt.g.trackLayout()
	e.tt.Frame()

	// 同一库同一张表，新开一个标签
	tt2 := newTableTab(w, "users")
	tt2.load()
	e.tt.Frame()
	if !tt2.g.pinned["email"] || !tt2.g.hidden["bio"] {
		t.Fatalf("应还原固定与隐藏: pinned=%v hidden=%v", tt2.g.pinned, tt2.g.hidden)
	}
	if got := visNames(tt2.g); got[0] != "email" || slices.Contains(got, "bio") {
		t.Fatalf("列的排列: %v", got)
	}
	if tt2.g.list.Columns.Widths["name"] != 222 {
		t.Fatalf("列宽应还原: %v", tt2.g.list.Columns.Widths)
	}
	// 另一张表不受影响
	tt3 := newTableTab(w, "orders")
	tt3.load()
	e.tt.Frame()
	if len(tt3.g.pinned) != 0 || len(tt3.g.hidden) != 0 {
		t.Fatal("别的表不应带上这份布局")
	}
}

// ⌘L 打开跳到列弹窗，回车选中第一个匹配的列。
func TestJumpToColumn(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	e.tt.Key(mui.Cmd, mui.KeyL)
	e.tt.Frame()
	if !tt.jumpOpen {
		t.Fatal("⌘L 应打开跳到列")
	}
	e.shot("97-jump-column")
	tt.jumpQ = "ema"
	if got := tt.jumpCandidates(); len(got) != 1 || got[0] != "email" {
		t.Fatalf("候选: %v", got)
	}
	tt.g.jumpTo(0, "email")
	e.tt.Frame()
	if tt.g.list.Cells.Cursor != "email" || !tt.g.chosen.Has(0) {
		t.Fatalf("应选中第 0 行的 email: cursor=%q", tt.g.list.Cells.Cursor)
	}
	if tt.g.reveal != nil {
		t.Fatal("格子建出来后应清掉待办的滚动")
	}
}
