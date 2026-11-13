package ui

import (
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
)

func tableTitles(w *Workspace) (out []string) {
	for _, tb := range w.tabs {
		out = append(out, tb.title())
	}
	return
}

// 单击侧栏的表只开预览标签；再单击别的表，替换这个预览标签。
func TestSingleClickOpensPreviewTabThatIsReplaced(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if err := e.tt.Click("orders"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.tabs) != 1 || !isPreviewTab(w.tabs[0]) || w.tabs[0].title() != "orders" {
		t.Fatalf("应有一个 orders 预览标签: %v", tableTitles(w))
	}
	if err := e.tt.Click("users"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.tabs) != 1 || w.tabs[0].title() != "users" || !isPreviewTab(w.tabs[0]) {
		t.Fatalf("预览标签应被替换: %v", tableTitles(w))
	}
}

// 双击固定：下一次单击会另开预览标签，而不是替换它。
func TestDoubleClickKeepsTabOpen(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	r, ok := e.tt.Find("orders")
	if !ok {
		t.Fatal("侧栏里没找到 orders")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	e.tt.ClickAt(x, y)
	e.tt.ClickAt(x, y)
	e.tt.Frame()
	if len(w.tabs) != 1 || isPreviewTab(w.tabs[0]) {
		t.Fatalf("双击应固定标签: %v preview=%v", tableTitles(w), len(w.tabs) > 0 && isPreviewTab(w.tabs[0]))
	}
	if err := e.tt.Click("users"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.tabs) != 2 {
		t.Fatalf("固定后单击别的表应另开标签: %v", tableTitles(w))
	}
}

// 有未提交修改的预览标签自动转为正式标签，不会被下一次单击顶掉。
func TestEditingPromotesPreviewTab(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.previewTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	tt.addRow()
	e.tt.Frame()
	if tt.preview {
		t.Fatal("有修改的预览标签应被固定")
	}
	w.previewTable("orders")
	if len(w.tabs) != 2 {
		t.Fatalf("应另开标签: %v", tableTitles(w))
	}
}

// 表、视图分区带数量；固定的表排在最前，并且重新打开后仍然固定。
func TestSidebarSectionsAndPins(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.tables = append(w.tables, database.TableInfo{Name: "v_users", Type: "VIEW"})
	rows := w.sideRows()
	var heads []string
	for _, r := range rows {
		if r.header {
			heads = append(heads, r.section)
		}
	}
	if len(heads) != 2 || heads[0] != "tables" || heads[1] != "views" {
		t.Fatalf("应有 tables 与 views 两个分区: %v", heads)
	}

	w.togglePin("users")
	rows = w.sideRows()
	if !rows[0].header || rows[0].section != "pinned" || rows[1].info.Name != "users" {
		t.Fatalf("固定的表应在最前: %+v", rows[:2])
	}
	for _, r := range rows {
		if !r.header && r.section == "tables" && r.info.Name == "users" {
			t.Fatal("固定的表不应在 Tables 分区重复出现")
		}
	}
	w.side.pins = nil
	w.loadPins()
	if len(w.side.pins) != 1 || w.side.pins[0] != "users" {
		t.Fatalf("固定应保存: %v", w.side.pins)
	}
	w.togglePin("users")
	if len(w.side.pins) != 0 {
		t.Fatal("取消固定后应为空")
	}

	w.previewTable("orders")
	e.shot("95-sidebar")
	w.side.collapsed = map[string]bool{"tables": true}
	for _, r := range w.sideRows() {
		if !r.header && r.section == "tables" {
			t.Fatal("折叠的分区不应列出表")
		}
	}
}

// Queries 分段只列收藏，History 分段列全部；过滤框对两者都生效。
func TestSidebarQueriesAndHistorySegments(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	hist := e.sh.app.History
	_ = hist.Add(w.cfg.ID, w.db, "SELECT 1 AS first_one")
	_ = hist.Add(w.cfg.ID, w.db, "SELECT 2 AS second_one")
	items, _ := hist.List(w.cfg.ID, "", 10)
	for _, it := range items {
		if it.SQL == "SELECT 2 AS second_one" {
			_ = hist.ToggleFavorite(it.ID)
		}
	}

	w.side.seg = sideHistory
	w.loadSideQueries()
	if len(w.side.queries) != 2 {
		t.Fatalf("历史应有 2 条: %d", len(w.side.queries))
	}
	w.side.seg = sideQueries
	w.loadSideQueries()
	if len(w.side.queries) != 1 || w.side.queries[0].SQL != "SELECT 2 AS second_one" {
		t.Fatalf("Queries 只列收藏: %+v", w.side.queries)
	}
	e.tt.Frame()
	e.has("second_one")

	w.filter = "first"
	w.side.seg = sideHistory
	w.loadSideQueries()
	if len(w.side.queries) != 1 || w.side.queries[0].SQL != "SELECT 1 AS first_one" {
		t.Fatalf("过滤应对历史生效: %+v", w.side.queries)
	}
}

// ⌘⇧F 把键盘交给侧栏的过滤框（查询标签里它仍是格式化）。
func TestSidebarFilterShortcut(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.sidebarHidden = true
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyF)
	e.tt.Frame()
	if w.sidebarHidden {
		t.Fatal("⌘⇧F 应展开侧栏")
	}
}

// 点击分区标题可以收起、再点展开，默认是展开的。
func TestSidebarSectionHeaderClickCollapses(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	e.has("orders")
	e.shot("96-sidebar-pills")
	r, ok := e.tt.Find("(2)")
	if !ok {
		t.Fatal("找不到分区标题")
	}
	e.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	e.tt.Frame()
	if !w.side.collapsed["tables"] {
		t.Fatal("点分区标题应收起")
	}
	e.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	e.tt.Frame()
	if w.side.collapsed["tables"] {
		t.Fatal("再点一次应展开")
	}
	e.has("orders")
}

// 历史按天分组：次序不变，今天、昨天写成文字，其余写日期。
func TestGroupHistoryByDay(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local)
	at := func(days int, hour int) time.Time {
		return time.Date(2026, 11, 14+days, hour, 0, 0, 0, time.Local)
	}
	items := []app.HistoryItem{
		{ID: "a", ExecutedAt: at(0, 9)}, {ID: "b", ExecutedAt: at(0, 8)},
		{ID: "c", ExecutedAt: at(-1, 23)}, {ID: "d", ExecutedAt: at(-3, 12)},
	}
	days := groupHistoryByDay(items, now)
	if len(days) != 3 || len(days[0].items) != 2 || days[0].items[0].ID != "a" {
		t.Fatalf("days=%+v", days)
	}
	if days[0].label != "今天" || days[1].label != "昨天" || days[2].label != "2026-11-11" {
		t.Fatalf("labels: %q %q %q", days[0].label, days[1].label, days[2].label)
	}
	if got := groupHistoryByDay(nil, now); len(got) != 0 {
		t.Fatalf("空历史应没有分组: %+v", got)
	}
}

// 侧栏历史页：每天一个标题，点标题折叠当天的语句，再点展开；收藏页仍是平铺。
func TestSidebarHistoryFoldsByDay(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	_ = e.sh.app.History.Add(w.cfg.ID, w.db, "SELECT 1 AS fold_me")
	w.side.seg = sideHistory
	w.loadSideQueries()
	e.tt.Frame()
	e.has("今天")
	e.has("fold_me")
	e.shot("24-history-by-day")
	if err := e.tt.Click("今天"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.hasNot("fold_me")
	e.has("今天")
	if err := e.tt.Click("今天"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.has("fold_me")
}
