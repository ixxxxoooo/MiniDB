package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/locales"
)

// dragTo 按住 from 的中心，经过一个小位移后移到 (x,y) 松手；overShot 不为空时在松手前截一张图。
func dragTo(e *env, from mui.Rect, x, y float32, overShot string) {
	cx, cy := from.X+from.W/2, from.Y+from.H/2
	e.tt.Press(cx, cy)
	e.tt.Move(cx+6, cy)
	e.tt.Move(x, y)
	e.tt.Frame()
	if overShot != "" {
		e.shot(overShot)
	}
	e.tt.Release(x, y)
	e.tt.Frame()
	e.tt.Frame()
}

// 把侧栏的表拖进 AI 输入框：拖在上面时盖「松手引用 xxx」，松手后文字末尾多一个 @表名，
// 再拖同一张表不重复加；拖进去之前输入框里已有的文字保留。
func TestDragSidebarTableIntoAssistantInput(t *testing.T) {
	e, w := assistantEnv(t)
	e.tt.Type("看看")
	e.tt.Frame()
	src, ok := e.tt.Find("users")
	if !ok {
		t.Fatal("找不到侧栏里的 users")
	}
	dst, ok := e.tt.Find(locales.AiHintSend())
	if !ok {
		t.Fatal("找不到输入框")
	}
	dragTo(e, src, dst.X+20, dst.Y-4, "r5-ai-drop-over")
	if !strings.HasPrefix(w.ai.input, "看看") || !strings.Contains(w.ai.input, "@users ") {
		t.Fatalf("松手后应在文字末尾补 @users: %q", w.ai.input)
	}
	e.has("users")
	e.shot("r5-ai-drop-done")
	n := strings.Count(w.ai.input, "@users")
	src, _ = e.tt.Find("users")
	dragTo(e, src, dst.X+20, dst.Y-4, "")
	if got := strings.Count(w.ai.input, "@users"); got != n {
		t.Fatalf("同一张表不该重复引用: %q", w.ai.input)
	}
}
