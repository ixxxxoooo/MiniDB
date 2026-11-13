package ui

import (
	"reflect"
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

func TestMentionAt(t *testing.T) {
	cases := []struct {
		text       string
		caret      int
		start, end int
		query      string
		ok         bool
	}{
		{"@", 1, 0, 1, "", true},
		{"看 @us", 5, 2, 5, "us", true},
		{"看 @users 的行数", 5, 2, 8, "us", true}, // 光标在名字中间：替换时整个名字都换掉
		{"看 @users 的行数", 9, 0, 0, "", false},  // 光标已在名字后面的文字里
		{"a@b", 3, 0, 0, "", false},           // 邮箱里的 @
		{"没有引用", 2, 0, 0, "", false},
		{"@@x", 3, 0, 0, "", false},
	}
	for _, c := range cases {
		start, end, query, ok := mentionAt([]rune(c.text), c.caret)
		if ok != c.ok || (ok && (start != c.start || end != c.end || query != c.query)) {
			t.Errorf("mentionAt(%q,%d) = %d,%d,%q,%v; want %d,%d,%q,%v", c.text, c.caret, start, end, query, ok, c.start, c.end, c.query, c.ok)
		}
	}
}

func TestMatchTablesRanksPrefixThenContainsThenComment(t *testing.T) {
	tables := []database.TableInfo{
		{Name: "order_users"}, {Name: "users"}, {Name: "audit", Comment: "记录用户操作 users"}, {Name: "orders"}, {Name: "user_roles"},
	}
	var names []string
	for _, tb := range matchTables(tables, "USER", 10) {
		names = append(names, tb.Name)
	}
	if !reflect.DeepEqual(names, []string{"users", "user_roles", "order_users", "audit"}) {
		t.Fatalf("前缀 → 表名包含 → 注释包含: %v", names)
	}
	if got := matchTables(tables, "", 3); len(got) != 3 {
		t.Fatalf("空查询给前几张: %d", len(got))
	}
	if got := matchTables(tables, "zzz", 3); len(got) != 0 {
		t.Fatal("没匹配的不该出现")
	}
}

func assistantEnv(t *testing.T) (*env, *Workspace) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	w.ai.ready, w.ai.checked = true, true
	e.tt.Frame()
	e.tt.Frame()
	return e, w
}

// 输入框里敲 @ 弹出表名；继续输入缩小范围；↑↓ 选，↵ 引用，文字里变成 @表名 加空格。
func TestMentionMenuKeyboardFlow(t *testing.T) {
	e, w := assistantEnv(t)
	e.tt.Type("看看 @")
	e.tt.Frame()
	e.tt.Frame()
	m := &w.ai.mention
	if !m.open || len(m.items) != len(w.tables) || len(m.items) < 2 {
		t.Fatalf("敲 @ 应列出全部的表: %+v", m)
	}
	e.has("↑↓ 选择")
	e.shot("29-mention-menu")

	e.tt.Type("us")
	e.tt.Frame()
	e.tt.Frame()
	if len(m.items) != 1 || m.items[0].Name != "users" {
		t.Fatalf("继续输入应缩小范围: %+v", m.items)
	}

	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	e.tt.Frame()
	if m.open {
		t.Fatal("Esc 应关掉列表")
	}
	e.tt.Type("e")
	e.tt.Frame()
	e.tt.Frame()
	if m.open {
		t.Fatal("关掉之后同一个片段不该再弹出")
	}
	e.tt.Type(" ") // 光标离开了片段
	e.tt.Type("@")
	e.tt.Frame()
	e.tt.Frame()
	if !m.open {
		t.Fatal("新的 @ 应重新弹出")
	}
	first := m.items[0].Name
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	if m.sel != 1 {
		t.Fatalf("↓ 应选中第二项: %d", m.sel)
	}
	e.tt.Key(0, mui.KeyUp)
	e.tt.Frame()
	if m.sel != 0 {
		t.Fatalf("↑ 回到第一项: %d", m.sel)
	}
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	e.tt.Frame()
	if want := "看看 @use @" + first + " "; w.ai.input != want {
		t.Fatalf("input = %q, want %q", w.ai.input, want)
	}
	if len(w.ai.msgs) != 0 {
		t.Fatal("菜单打开时的回车是引用，不是发送")
	}
	if m.open {
		t.Fatal("引用后列表应收起")
	}
	e.has(first) // chip
	e.shot("30-mention-chip")
}

// 点列表里的项也能引用；chip 上的 ✕ 删掉文字里对应的引用。
func TestMentionClickAndRemove(t *testing.T) {
	e, w := assistantEnv(t)
	e.tt.Type("比较 @")
	e.tt.Frame()
	e.tt.Frame()
	if !w.ai.mention.open {
		t.Fatal("应弹出列表")
	}
	last := len(w.ai.mention.items) - 1
	name := w.ai.mention.items[last].Name
	// 左边的侧栏里也有同名的表，所以按列表底部提示行的位置点最后一项
	hint, ok := e.tt.Find(locales.AiMentionHint())
	if !ok {
		t.Fatal("找不到列表")
	}
	e.tt.ClickAt(hint.X+20, hint.Y-14)
	e.tt.Frame()
	e.tt.Frame()
	if want := "比较 @" + name + " "; w.ai.input != want {
		t.Fatalf("input = %q, want %q", w.ai.input, want)
	}
	w.ai.mention.remove = name
	e.tt.Frame()
	e.tt.Frame()
	if w.ai.input != "比较 " {
		t.Fatalf("移除引用应连着后面的空格一起删: %q", w.ai.input)
	}
}

// 发出去的消息里引用是同样的 chip；保存的会话恢复出来也是；模型收到点名表的完整结构。
func TestMentionSurvivesSendingAndRestoring(t *testing.T) {
	e := newEnv(t)
	srv, bodies := sqlLLM(t, "共有 3 个用户")
	e.useLLM(srv)
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("统计 @users 的行数，再看看 @ghost")
	e.tt.Frame()
	e.has("users")
	e.has("ghost")
	if !strings.Contains(bodies()[0], "用户用 @ 点名的表") || !strings.Contains(bodies()[0], "CREATE TABLE users") {
		t.Fatalf("模型应收到点名的表: %s", bodies()[0])
	}
	e.shot("31-mention-message")

	stored, err := e.sh.app.AI.GetAISessionMessages(w.ai.session, 0)
	if err != nil || len(stored) == 0 || !strings.Contains(stored[0].Content, "@users") {
		t.Fatalf("会话里存的就是带 @users 的原文: %+v %v", stored, err)
	}
	session := w.ai.session
	w.ai.reset()
	w.ai.loadSession(session)
	e.tt.Frame()
	if len(w.ai.msgs) < 2 || !strings.Contains(w.ai.msgs[0].text, "@users") {
		t.Fatalf("恢复出来的提问应带着引用: %+v", w.ai.msgs)
	}
	e.has("users")
	e.has("ghost")
}
