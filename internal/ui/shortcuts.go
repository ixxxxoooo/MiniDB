package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// The original's keyboard shortcuts (Settings → Shortcuts), handled here for
// the whole window so they work wherever the focus is. The menu bar has the
// same accelerators; fire keeps a key that reaches both from running twice.

// modalOpen reports whether a dialog holds the keyboard.
func (s *Shell) modalOpen() bool {
	return s.dlg.open || s.set.open || s.exp.open || s.bak.open || s.rst.open || s.usr.open || s.imp.open || s.proc.open ||
		s.sw.open || s.pal.open || s.confirm.open || s.pwp.open || s.wsPop.open || s.notesOpen || s.edOpt || s.prm.open || s.srch.open
}

// fire runs fn unless the same command ran a moment ago.
func (s *Shell) fire(name string, fn func()) {
	now := time.Now()
	if s.lastKey == name && now.Sub(s.lastKeyAt) < 150*time.Millisecond {
		return
	}
	s.lastKey, s.lastKeyAt = name, now
	fn()
}

func (s *Shell) activeTable() *tableTab {
	if w := s.activeWorkspace(); w != nil && w.active < len(w.tabs) {
		t, _ := w.tabs[w.active].(*tableTab)
		return t
	}
	return nil
}

func (s *Shell) shortcuts(c *ui.Context) {
	if s.modalOpen() {
		return
	}
	// list 是可以在设置里改键的命令，键由 keymap 决定。
	type sc struct {
		id string
		fn func()
	}
	// fixed 是改不了的别名和单键，和 fixedChords 对应。
	type fixedSC struct {
		mods ui.Modifiers
		key  ui.Key
		name string
		fn   func()
	}
	w := s.activeWorkspace()
	cycleWS := func(d int) {
		if n := len(s.spaces); n > 1 {
			s.active = (s.active + d + n) % n
		}
	}
	subView := func(d int) {
		if t := s.activeTable(); t != nil {
			t.mode = (t.mode + d + 4) % 4
			t.modeChanged()
		}
	}
	list := []sc{
		{"new-conn", func() { s.newConnectionDialog() }},
		{"open-conn", func() { s.newConnectionDialog() }},
		{"new-query", func() {
			if w != nil {
				w.newQuery("")
			}
		}},
		{"switcher", func() { s.openSwitcher() }},
		{"palette", func() { s.openPalette() }},
		{"settings", func() { s.openSettings(0) }},
		{"close-tab", func() {
			if w != nil && len(w.tabs) > 0 {
				w.closeTab(w.active)
			}
		}},
		{"ai", func() {
			if t := s.activeTable(); t != nil && t.filterFocused {
				return // in the filter bar ⌘I adds a condition
			}
			if w != nil {
				w.togglePanel(2)
			}
		}},
		{"refresh", s.cmdRefresh},
		{"tab-prev", func() { s.cmdCycleTab(-1) }},
		{"tab-next", func() { s.cmdCycleTab(1) }},
		{"ws-prev", func() { cycleWS(-1) }},
		{"ws-next", func() { cycleWS(1) }},
		{"sub-prev", func() { subView(-1) }},
		{"sub-next", func() { subView(1) }},
		{"filter", func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.toggleFilter()
			}
		}},
		{"side-filter", func() {
			// 查询标签里 ⌘⇧F 是格式化，由编辑器处理
			if w != nil && s.activeQuery() == nil {
				w.sidebarHidden = false
				w.side.seg = sideItems
				w.side.focusFilter = true
			}
		}},
		{"disconnect", func() {
			if w != nil {
				s.closeConnection(w.cfg.ID)
			}
		}},
		{"fullscreen", func() {
			if s.win != nil {
				s.win.ToggleFullScreen()
			}
		}},
		{"commit", s.commitPending},
		// 待提交变更：预览 SQL、丢弃（提交本身不弹窗）
		{"preview", s.previewPending},
		{"discard", s.discardPending},
		// 导航与工具栏
		{"sidebar", func() {
			if w != nil {
				w.sidebarHidden = !w.sidebarHidden
			}
		}},
		{"workspaces", s.openWorkspaces},
		{"new-sql", func() {
			if w != nil {
				w.newQuery("")
			}
		}},
		{"processes", s.cmdProcesses},
		{"export", s.cmdExport},
		{"reconnect", func() {
			if w != nil {
				s.reconnect(w)
			}
		}},
		{"refresh-tab", s.cmdRefreshTab},
		{"back", func() {
			if w != nil {
				w.goBack()
			}
		}},
		{"forward", func() {
			if w != nil {
				w.goForward()
			}
		}},
		{"console", func() { s.consoleOpen = !s.consoleOpen }},
		// 编辑器字号
		{"font-up", func() { s.zoomEditor(1) }},
		{"font-down", func() { s.zoomEditor(-1) }},
		{"columns", func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.colsOpen = !t.colsOpen
			}
		}},
		{"favorite", func() {
			if q := s.activeQuery(); q != nil {
				q.toggleFavorite()
			}
		}},
	}
	fixed := []fixedSC{
		{ui.Cmd | ui.Shift, ui.KeyK, "open-conn2", func() { s.newConnectionDialog() }},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketLeft, "sub-prev2", func() { subView(-1) }},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketRight, "sub-next2", func() { subView(1) }},
		{0, ui.KeyEscape, "deselect", func() {
			// TablePlus: Esc deselects all rows (the filter bar, a cell being
			// edited and the dialogs take Esc first)
			if t := s.activeTable(); t != nil && t.mode == 0 && !t.showFilter && t.es.editing == nil && !t.qlOpen && t.quick == nil {
				t.g.chosen.Clear()
			}
		}},
		{0, ui.KeyInsert, "insert", func() {
			if t := s.activeTable(); t != nil && !t.w.cfg.ReadOnly {
				switch t.mode {
				case 0:
					t.addRow()
				case 1:
					t.st.addColumn()
				}
			}
		}},
	}
	// ⌘1 到 ⌘9 按序号切换标签
	for i, key := range []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9} {
		i := i
		fixed = append(fixed, fixedSC{ui.Cmd, key, fmt.Sprint("tab", i), func() {
			if w != nil && i < len(w.tabs) {
				w.active = i
			}
		}})
	}
	for _, k := range list {
		if s.hit(c, k.id) {
			s.fire(k.id, k.fn)
			s.redraw()
		}
	}
	for _, k := range fixed {
		if c.Shortcut(k.mods, k.key) {
			s.fire(k.name, k.fn)
			s.redraw()
		}
	}
}
