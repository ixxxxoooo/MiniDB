package ui

import (
	"github.com/egoist/mygo/ui"
)

// ---- resizable panes ----

// vHandle is the thin line between two side-by-side panes. It reports how far
// it was dragged this frame, and a double click.
func vHandle(c *ui.Context, p tokens, key string) (dx float32, reset bool) {
	// 布局里只占 1px 的分割线，两侧不留空隙；真正用来抓取的 7px 宽区域浮在线的上方。
	hover := false
	line := ui.Box(c.Key(key)).Width(1).FillHeight().Shrink(0).Background(p.border)
	line.Children(func() {
		grab := ui.Box(c.Key(key + "-grab")).Absolute().Top(0).Bottom(0).Left(-3).Width(7).Cursor(ui.CursorResizeEW).Background(ui.Transparent)
		if x, _, ok := grab.Dragged(); ok {
			dx = x
		}
		reset = grab.DoubleClicked()
		hover = grab.Hovered()
	})
	line.Draw(func(pt *ui.Painter, r ui.Rect) {
		if hover {
			pt.Fill(r, p.accent, 0)
		}
	})
	return dx, reset
}

// view lays out a workspace: the sidebar (which ⌘0 hides), the content, and
// the AI panel when open. Every pane keeps its place in the tree whether or
// not its neighbours are shown, so opening one never resets another's scroll.
func (w *Workspace) view(c *ui.Context, p tokens) {
	w.trackTabs()
	ui.Row(c).Grow(1).FillWidth().FillHeight().MinWidth(0).Children(func() {
		side := ui.Column(c.Key("sidebar")).FillHeight().Shrink(0)
		w.sideHover = !w.sidebarHidden && side.Hovered() // 鼠标在侧栏上：⌘F 搜的是侧栏
		if w.sidebarHidden {
			side.Width(0).Children(func() {})
		} else {
			side.Width(w.sidebarW).Children(func() { perfBuild("ui.build.sidebar", func() { w.sidebar(c, p) }) })
			dx, reset := vHandle(c, p, "sidebar-handle")
			w.sidebarW = min(max(w.sidebarW+dx, 160), 520)
			if reset {
				w.sidebarW = 260
			}
		}
		ui.Column(c.Key("content")).Grow(1).MinWidth(0).FillHeight().Children(func() { w.content(c, p) })
	})
}

// content is the tabs, with the AI panel beside them when it is open.
func (w *Workspace) content(c *ui.Context, p tokens) {
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		ui.Column(c.Key("main")).Grow(1).MinWidth(0).FillHeight().Children(func() { w.main(c, p) })
		if w.panel == 2 {
			dx, reset := vHandle(c, p, "ai-handle")
			w.panelW = min(max(w.panelW-dx, 280), 720)
			if reset {
				w.panelW = 340
			}
			ui.Column(c.Key("ai")).Width(w.panelW).FillHeight().Shrink(0).Children(func() { perfBuild("ui.build.assistant", func() { w.ai.view(c, p) }) })
		}
	})
}

// ---- tab history (⌘⌃← and ⌘⌃→) ----

// trackTabs remembers the tabs the user came from, for back and forward.
func (w *Workspace) trackTabs() {
	if w.active == w.lastActive {
		return
	}
	if !w.navigating {
		w.back = append(w.back, w.lastActive)
		w.fwd = nil
	}
	w.navigating = false
	w.lastActive = w.active
}

// goBack and goForward move through the tabs visited.
func (w *Workspace) goBack()    { w.go1(&w.back, &w.fwd) }
func (w *Workspace) goForward() { w.go1(&w.fwd, &w.back) }

func (w *Workspace) go1(from, to *[]int) {
	for len(*from) > 0 {
		i := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		if i >= 0 && i < len(w.tabs) && i != w.active {
			*to = append(*to, w.active)
			w.navigating = true
			w.active = i
			return
		}
	}
}

// ---- notifications and the console ----

// ---- pending changes: Discard / Preview / Commit ----

// ---- the list of workspaces (⌘⇧\) ----
