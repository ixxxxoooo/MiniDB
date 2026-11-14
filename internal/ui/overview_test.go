package ui

import (
	"minidb/internal/app"
	"testing"
)

// 项目概览：列出所有表和视图（SQLite 有精确行数），能排序、筛选、汇总，右键和侧栏一致。
func TestOverviewTab(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	execSQL(t, e, w, `CREATE VIEW v_all AS SELECT id FROM users`)
	w.loadTables()
	w.openOverview()
	e.tt.Frame()
	o := w.tabs[len(w.tabs)-1].(*overviewTab)
	if len(o.items) != 3 {
		t.Fatalf("items=%+v err=%q", o.items, o.err)
	}
	e.has("项目概览")
	e.has("users")
	e.has("v_all")
	e.shot("26-overview")
	byName := map[string]app.TableOverview{}
	for _, it := range o.items {
		byName[it.Name] = it
	}
	if byName["users"].Rows != 450 || byName["orders"].Rows != 0 || byName["users"].Type != "table" {
		t.Fatalf("users=%+v orders=%+v", byName["users"], byName["orders"])
	}
	if v := byName["v_all"]; v.Type != "view" || v.Rows != -1 {
		t.Fatalf("view=%+v", v)
	}

	// 默认按名称；按行数降序；筛选
	o.arrange()
	if o.items[o.shown[0]].Name != "orders" {
		t.Fatalf("默认应按名称排: %v", o.shown)
	}
	o.sort.Column, o.sort.Descending = "rows", true
	o.arrange()
	if o.items[o.shown[0]].Name != "users" {
		t.Fatalf("按行数降序首行应是 users: %v", o.shown)
	}
	o.q = "ord"
	o.arrange()
	if len(o.shown) != 1 || o.items[o.shown[0]].Name != "orders" {
		t.Fatalf("筛选: %v", o.shown)
	}
	o.q = ""

	if got := overviewSummary(o.items); got == "" {
		t.Fatal("汇总不应为空")
	}
	if overviewNum(-1) != "—" || overviewSize(-1) != "—" || orDash(" ") != "—" {
		t.Fatal("取不到的值应显示 —")
	}

	// 再次打开只是切过去，不会多开一个标签
	n := len(w.tabs)
	w.openOverview()
	if len(w.tabs) != n {
		t.Fatal("概览标签应只有一个")
	}
	// 会话里记下，还原时重新打开
	found := false
	for _, ts := range w.snapshot().Tabs {
		found = found || ts.Kind == "overview"
	}
	if !found {
		t.Fatal("概览应存进会话")
	}
}

// 概览行的右键菜单和侧栏的表一样；删表后概览跟着刷新。
func TestOverviewFollowsTableChanges(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openOverview()
	e.tt.Frame()
	o := w.tabs[len(w.tabs)-1].(*overviewTab)
	if len(o.items) != 2 {
		t.Fatalf("items=%d", len(o.items))
	}
	execSQL(t, e, w, `DROP TABLE orders`)
	w.loadTables()
	e.tt.Frame()
	if len(o.items) != 1 || o.items[0].Name != "users" {
		t.Fatalf("删表后概览应刷新: %+v", o.items)
	}
}
