package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
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
