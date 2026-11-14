package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// erEnv 建一个有外键的库：users ← items ← notes，另有孤立的 lonely 和原有的 orders。
func erEnv(t *testing.T) (*env, *Workspace, *erTab) {
	e := newEnv(t)
	w := openWorkspace(e)
	execSQL(t, e, w, `CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`)
	execSQL(t, e, w, `CREATE TABLE notes (id INTEGER PRIMARY KEY, item_id INTEGER REFERENCES items(id), body TEXT)`)
	execSQL(t, e, w, `CREATE TABLE lonely (id INTEGER PRIMARY KEY)`)
	w.loadTables()
	w.openER("")
	e.tt.Frame()
	tab, ok := w.tabs[len(w.tabs)-1].(*erTab)
	if !ok {
		t.Fatalf("最后一个标签不是 ER 图: %T", w.tabs[len(w.tabs)-1])
	}
	return e, w, tab
}

func TestERTabLoadsWholeSmallDatabase(t *testing.T) {
	e, _, tab := erEnv(t)
	if tab.err != "" || tab.full == nil {
		t.Fatalf("err=%q full=%v", tab.err, tab.full)
	}
	if len(tab.full.Nodes) != 5 || len(tab.full.Edges) != 2 {
		t.Fatalf("nodes=%d edges=%d", len(tab.full.Nodes), len(tab.full.Edges))
	}
	if tab.focus != "" || tab.depth != -1 || len(tab.shown.Nodes) != 5 {
		t.Fatalf("表不多应整库显示: focus=%q depth=%d shown=%d", tab.focus, tab.depth, len(tab.shown.Nodes))
	}
	if tab.vw <= 0 || tab.zoom <= 0 {
		t.Fatalf("绘制后应有画布大小和缩放: %v %v", tab.vw, tab.zoom)
	}
	e.has("ER 图")
	e.has("5 张表")
	e.shot("er-whole")
}

func TestERTabFocusNeighborhood(t *testing.T) {
	e, w, tab := erEnv(t)
	w.openER("items") // 再次打开应复用同一个标签
	e.tt.Frame()
	n := 0
	for _, tb := range w.tabs {
		if _, ok := tb.(*erTab); ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("ER 图标签应只有一个: %d", n)
	}
	if tab.focus != "items" || tab.depth != 1 || len(tab.shown.Nodes) != 3 {
		t.Fatalf("以 items 为中心一层应是 users/items/notes: focus=%q depth=%d shown=%v", tab.focus, tab.depth, tab.shown.Nodes)
	}
	if _, in := tab.rects["lonely"]; in {
		t.Fatal("孤立的表不该在邻居子图里")
	}
	tab.setFocus("", -1)
	e.tt.Frame()
	if len(tab.shown.Nodes) != 5 {
		t.Fatalf("整库: %d", len(tab.shown.Nodes))
	}
}

func TestERTabPanZoomAndDrag(t *testing.T) {
	e, _, tab := erEnv(t)
	z0 := tab.zoom
	// 在画布中心缩放：中心下面的世界坐标不动
	cx, cy := tab.vw/2, tab.vh/2
	wx, wy := cx/tab.zoom+tab.panX, cy/tab.zoom+tab.panY
	tab.handle(mui.InputEvent{Kind: mui.InputZoom, X: cx, Y: cy, Scale: 1.5})
	if tab.zoom <= z0 {
		t.Fatalf("应放大: %v → %v", z0, tab.zoom)
	}
	if ax, ay := cx/tab.zoom+tab.panX, cy/tab.zoom+tab.panY; abs32(ax-wx) > 0.01 || abs32(ay-wy) > 0.01 {
		t.Fatalf("缩放中心下的内容应不动: (%v,%v) → (%v,%v)", wx, wy, ax, ay)
	}
	// 缩放有上下限
	for i := 0; i < 40; i++ {
		tab.handle(mui.InputEvent{Kind: mui.InputZoom, X: cx, Y: cy, Scale: 2})
	}
	if tab.zoom > 4.0001 {
		t.Fatalf("缩放上限: %v", tab.zoom)
	}
	tab.fit()

	// 拖空白处平移
	ex, ey := tab.panX, tab.panY
	tab.handle(mui.InputEvent{Kind: mui.InputPointerDown, X: 2, Y: 2})
	tab.handle(mui.InputEvent{Kind: mui.InputPointerMove, X: 22, Y: 12})
	tab.handle(mui.InputEvent{Kind: mui.InputPointerUp, X: 22, Y: 12})
	if tab.panX >= ex || tab.panY >= ey {
		t.Fatalf("向右下拖应让 pan 减小: (%v,%v) → (%v,%v)", ex, ey, tab.panX, tab.panY)
	}

	// 拖表：落在 users 的表头上
	r := tab.rects["users"]
	sx, sy := (r.X+r.W/2-tab.panX)*tab.zoom, (r.Y+4-tab.panY)*tab.zoom
	tab.handle(mui.InputEvent{Kind: mui.InputPointerDown, X: sx, Y: sy})
	if tab.sel != "users" {
		t.Fatalf("按在表上应选中它: %q", tab.sel)
	}
	tab.handle(mui.InputEvent{Kind: mui.InputPointerMove, X: sx + 30, Y: sy + 20})
	tab.handle(mui.InputEvent{Kind: mui.InputPointerUp, X: sx + 30, Y: sy + 20})
	r2 := tab.rects["users"]
	if abs32(r2.X-r.X-30/tab.zoom) > 0.01 || abs32(r2.Y-r.Y-20/tab.zoom) > 0.01 {
		t.Fatalf("表应跟着移动: %+v → %+v", r, r2)
	}
	if got := tab.manual["users"]; got != [2]float32{r2.X, r2.Y} {
		t.Fatalf("拖动后应记下位置: %v", got)
	}
	e.tt.Frame()
	e.shot("er-dragged")

	// 重新布局：回到自动位置
	tab.relayout()
	if len(tab.manual) != 0 || tab.rects["users"] == r2 {
		t.Fatalf("重新布局应丢掉手动位置: %v", tab.manual)
	}
}

func TestERTabPositionsPersist(t *testing.T) {
	e, w, tab := erEnv(t)
	tab.manual = map[string][2]float32{"users": {500, 600}}
	tab.saveLayout()
	tab2 := &erTab{w: w, zoom: 1, depth: -1}
	w.tabs = append(w.tabs, tab2)
	tab2.load()
	e.tt.Frame()
	if r := tab2.rects["users"]; r.X != 500 || r.Y != 600 {
		t.Fatalf("保存的位置应在重新加载时用上: %+v", r)
	}
}

func TestERTabDoubleClickOpensTable(t *testing.T) {
	e, w, tab := erEnv(t)
	r := tab.rects["notes"]
	sx, sy := (r.X+r.W/2-tab.panX)*tab.zoom, (r.Y+4-tab.panY)*tab.zoom
	tab.handle(mui.InputEvent{Kind: mui.InputPointerDown, X: sx, Y: sy, Clicks: 2})
	e.tt.Frame()
	tt, ok := w.tabs[w.active].(*tableTab)
	if !ok || tt.table != "notes" {
		t.Fatalf("双击应打开 notes 表: %T", w.tabs[w.active])
	}
}

func TestERTabSearchAndSession(t *testing.T) {
	e, w, tab := erEnv(t)
	tab.setFocus("users", 1)
	tab.q = "note"
	tab.search() // notes 不在 users 一层内 → 以它为中心重取
	if tab.focus != "notes" || tab.sel != "notes" {
		t.Fatalf("搜索应切到 notes: focus=%q sel=%q", tab.focus, tab.sel)
	}
	st := w.snapshot()
	last := st.Tabs[len(st.Tabs)-1]
	if last.Kind != KindER || last.Table != "notes" {
		t.Fatalf("会话应记下 ER 图和中心表: %+v", last)
	}
	e.tt.Frame()
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
