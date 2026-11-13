package ui

import (
	"testing"
	"time"

	"minidb/locales"
)

// 悬浮提示要停留够久才出现；刚看过一个提示后移到下一个，也要重新等够时间，不能立即弹出。
func TestTooltipWaitsBeforeShowing(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.tt.Frame()
	settings, ok := e.tt.Find(locales.ToolbarSettings() + e.sh.keyHint("settings"))
	if !ok {
		t.Fatalf("找不到设置按钮: %q", e.tt.Texts())
	}
	search, ok := e.tt.Find(locales.ToolbarSearch() + e.sh.keyHint("palette"))
	if !ok {
		t.Fatal("找不到搜索按钮")
	}
	tipText := locales.ToolbarSettings() + e.sh.keyHint("settings")

	base := countText(e, tipText) // 按钮本身的无障碍标签也可能算作文字，以它为基准
	e.tt.Move(settings.X+settings.W/2, settings.Y+settings.H/2)
	e.tt.Frame()
	e.tt.Advance(200 * time.Millisecond)
	if countText(e, tipText) > base {
		t.Fatal("指针刚停上去就出现了提示")
	}
	e.tt.Advance(time.Second)
	e.tt.Frame()
	e.tt.Frame()
	e.shot("60-tooltip")
	if countText(e, tipText) <= base {
		t.Fatalf("停留够久后应出现提示 %q，屏幕上有 %q", tipText, e.tt.Texts())
	}

	// 紧接着移到相邻的按钮上：框架自带的热身逻辑会让它立即弹出，这里不允许。
	searchTip := locales.ToolbarSearch() + e.sh.keyHint("palette")
	baseSearch := countText(e, searchTip)
	e.tt.Move(search.X+search.W/2, search.Y+search.H/2)
	e.tt.Frame()
	e.tt.Advance(50 * time.Millisecond)
	e.tt.Frame()
	if countText(e, searchTip) > baseSearch {
		t.Fatal("相邻按钮的提示不该立即出现")
	}
}

// countText 数屏幕上这段文字出现了几次（按钮的无障碍标签也会命中，所以只数可见文字）。
func countText(e *env, s string) int {
	n := 0
	for _, x := range e.tt.Texts() {
		if x == s {
			n++
		}
	}
	return n
}
