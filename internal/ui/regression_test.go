package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 本地排序：查询结果的排序在客户端完成，三态（升序 → 降序 → 不排序）。
func TestQueryResultLocalSortAndPreview(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT id, name, age FROM users WHERE id <= 5 ORDER BY id")
	q.run(false)
	e.tt.Frame()
	if len(q.results) != 1 {
		t.Fatalf("results=%d", len(q.results))
	}
	g := q.results[0].g
	if !g.local {
		t.Fatal("查询结果应该是本地排序")
	}
	first := func() any { return g.rows[0]["id"] }
	// 降序
	g.sort = mui.SortOrder{Column: "id", Descending: true}
	g.rows = g.arranged(g.src)
	if first() != int64(5) {
		t.Fatalf("降序后首行 id=%v", first())
	}
	// 不排序：回到数据库给的顺序
	g.sort = mui.SortOrder{}
	g.rows = g.arranged(g.src)
	if first() != int64(1) {
		t.Fatalf("不排序后首行 id=%v", first())
	}

	// 行预览：没选行时打开会选中第一行
	q.results[0].togglePreview()
	if !q.results[0].showPreview || len(q.results[0].previewRows()) != 1 {
		t.Fatal("打开预览应选中第一行")
	}
	e.has("用户001")
	e.shot("48-query-preview")
	// 和表数据页共用同一个行预览组件：只读，没有字段右端的下拉箭头
	if q.results[0].pv.sig == nil {
		t.Fatal("查询结果的行预览应由共用的组件画出")
	}
}

// 行预览和 Quick Look 里的 JSON 编辑器：光标靠近括号时不描配对的外框。
func TestNoBracketMatchOutline(t *testing.T) {
	ed := &editor.State{}
	noBracketMatch(ed)
	if ed.Theme == nil || ed.Theme.MatchingBracket.A == 0 || ed.Theme.MatchingBracket.A > 2 {
		t.Fatalf("括号配对描边应当几乎透明（但不能全 0，否则编辑器会换回默认色）: %+v", ed.Theme)
	}
}

// 通知从铃铛处弹出，而不是居中的对话框。
func TestNotesPopover(t *testing.T) {
	e := newEnv(t)
	e.sh.toastOK("第一条通知")
	e.sh.toastOK("第二条通知")
	e.tt.Frame()
	e.sh.notesOpen = true
	e.has("第一条通知")
	e.shot("49-notes-popover")
}

func TestClipText(t *testing.T) {
	if got := clipText("abcdef", 3, 5); got != "abc…" {
		t.Fatalf("按字数截断: %q", got)
	}
	if got := clipText("a\nb\nc\nd", 100, 2); got != "a\nb…" {
		t.Fatalf("按行截断: %q", got)
	}
	if got := clipText("short", 100, 5); got != "short" {
		t.Fatalf("短文本不应变: %q", got)
	}
}

// 点格子（只是改变选中）不能触发重新加载；点表头才排序，且是 升序 → 降序 → 不排序 三态。
func TestGridClickDoesNotReloadAndHeaderCycles(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	settle := func() {
		for i := 0; i < 200 && tab.loading; i++ {
			sleepMs(10)
			e.tt.Frame()
		}
		e.tt.Frame()
	}
	settle()
	e.tt.ClickAt(450, 150) // 一个格子
	e.tt.Frame()
	if tab.loading {
		t.Fatal("点格子不应触发重新加载")
	}

	clickHeader := func(title string) {
		if err := e.tt.Click(title); err != nil {
			t.Fatalf("%v", err)
		}
		settle()
	}
	clickHeader("name")
	if tab.g.sort.Column != "name" || tab.g.sort.Descending {
		t.Fatalf("第一次点击应升序: %+v", tab.g.sort)
	}
	// 排序标记是图标，不再往列名后面拼 ↑/↓ 字符，所以列名文字保持不变
	if m, ok := tab.g.mark("name"); !ok || m.desc {
		t.Fatalf("升序时 name 列应有朝上的标记: %+v %v", m, ok)
	}
	e.has("name")
	e.shot("53-sort-asc")
	clickHeader("name")
	if !tab.g.sort.Descending {
		t.Fatalf("第二次点击应降序: %+v", tab.g.sort)
	}
	if m, ok := tab.g.mark("name"); !ok || !m.desc {
		t.Fatalf("降序时 name 列应有朝下的标记: %+v %v", m, ok)
	}
	clickHeader("name")
	if _, ok := tab.g.mark("name"); ok {
		t.Fatal("取消排序后不应再有标记")
	}
	if tab.g.sort.Column != "" {
		t.Fatalf("第三次点击应取消排序: %+v", tab.g.sort)
	}
}
