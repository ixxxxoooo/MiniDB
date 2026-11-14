package ui

import "testing"

// 拖出来的栏宽、收起的侧栏、日志面板、侧栏折叠的分组，关闭再打开都在。
func TestLayoutIsRemembered(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.sidebarW, w.panelW, w.sidebarHidden = 333, 444, true
	w.side.seg = sideHistory
	w.side.collapsed = map[string]bool{"views": true, "day:2026-11-14": true}
	e.sh.consoleOpen, e.sh.consolePct = true, 61
	w.openTable("users")
	tt := w.tabs[0].(*tableTab)
	tt.showPreview, tt.previewW = true, 400
	e.sh.SaveState()

	// 重新「启动」：布局和会话都从存档里读
	e.sh.layout, e.sh.consoleOpen, e.sh.consolePct = layoutState{}, false, 0
	e.sh.loadLayout()
	if !e.sh.consoleOpen || e.sh.consolePct != 61 {
		t.Fatalf("日志面板: open=%v pct=%v", e.sh.consoleOpen, e.sh.consolePct)
	}
	w2 := newWorkspace(e.sh, w.cfg)
	if w2.sidebarW != 333 || w2.panelW != 444 || !w2.sidebarHidden {
		t.Fatalf("新工作区应按记住的布局: sidebar=%v panel=%v hidden=%v", w2.sidebarW, w2.panelW, w2.sidebarHidden)
	}
	var st wsState
	if err := e.sh.app.UIState.LoadWorkspaceState(w.cfg.ID, &st); err != nil {
		t.Fatal(err)
	}
	w2.restore(st)
	if w2.side.seg != sideHistory || !w2.side.collapsed["views"] || !w2.side.collapsed["day:2026-11-14"] {
		t.Fatalf("侧栏分段与折叠: seg=%d collapsed=%v", w2.side.seg, w2.side.collapsed)
	}
	rt, ok := w2.tabs[0].(*tableTab)
	if !ok || !rt.showPreview || rt.previewW != 400 {
		t.Fatalf("行预览栏应保持打开和宽度: %+v", w2.tabs)
	}
	// 坏数据不会把界面撑坏
	if clampWidth(99999, 160, 520, 260) != 520 || clampWidth(-5, 160, 520, 260) != 260 {
		t.Fatal("clampWidth")
	}
}
