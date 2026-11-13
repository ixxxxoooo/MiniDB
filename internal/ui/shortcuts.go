package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The original's keyboard shortcuts (Settings → Shortcuts), handled here for
// the whole window so they work wherever the focus is. The menu bar has the
// same accelerators; fire keeps a key that reaches both from running twice.

// modalOpen reports whether a dialog holds the keyboard.
func (s *Shell) modalOpen() bool {
	return s.dlg.open || s.set.open || s.exp.open || s.imp.open || s.proc.open ||
		s.sw.open || s.pal.open || s.confirm.open || s.pwp.open || s.wsPop.open || s.notesOpen
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
	type sc struct {
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
		{ui.Cmd, ui.KeyN, "new-conn", func() { s.newConnectionDialog() }},
		{ui.Cmd, ui.KeyO, "open-conn", func() { s.newConnectionDialog() }},
		{ui.Cmd, ui.KeyT, "new-query", func() {
			if w != nil {
				w.newQuery("")
			}
		}},
		{ui.Cmd, ui.KeyK, "switcher", func() { s.openSwitcher() }},
		{ui.Cmd, ui.KeyP, "palette", func() { s.openPalette() }},
		{ui.Cmd, ui.KeyComma, "settings", func() { s.openSettings(0) }},
		{ui.Cmd, ui.KeyW, "close-tab", func() {
			if w != nil && len(w.tabs) > 0 {
				w.closeTab(w.active)
			}
		}},
		{ui.Cmd, ui.KeyI, "ai", func() {
			if w != nil {
				w.togglePanel(2)
			}
		}},
		{ui.Cmd, ui.KeyR, "refresh", s.cmdRefresh},
		{ui.Cmd, ui.KeyBracketLeft, "tab-prev", func() { s.cmdCycleTab(-1) }},
		{ui.Cmd, ui.KeyBracketRight, "tab-next", func() { s.cmdCycleTab(1) }},
		{ui.Cmd | ui.Shift, ui.KeyBracketLeft, "ws-prev", func() { cycleWS(-1) }},
		{ui.Cmd | ui.Shift, ui.KeyBracketRight, "ws-next", func() { cycleWS(1) }},
		{ui.Cmd | ui.Alt, ui.KeyBracketLeft, "sub-prev", func() { subView(-1) }},
		{ui.Cmd | ui.Alt, ui.KeyBracketRight, "sub-next", func() { subView(1) }},
		{ui.Cmd, ui.KeyF, "filter", func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.toggleFilter()
			}
		}},
		{ui.Cmd | ui.Shift, ui.KeyF, "side-filter", func() {
			// 查询标签里 ⌘⇧F 是格式化，由编辑器处理
			if w != nil && s.activeQuery() == nil {
				w.sidebarHidden = false
				w.side.seg = sideItems
				w.side.focusFilter = true
			}
		}},
		{ui.Cmd | ui.Shift, ui.KeyK, "open-conn2", func() { s.newConnectionDialog() }},
		{ui.Cmd | ui.Alt, ui.KeyPeriod, "disconnect", func() {
			if w != nil {
				s.closeConnection(w.cfg.ID)
			}
		}},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketLeft, "sub-prev2", func() { subView(-1) }},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketRight, "sub-next2", func() { subView(1) }},
		{ui.Cmd | ui.Ctrl, ui.KeyF, "fullscreen", func() {
			if s.win != nil {
				s.win.ToggleFullScreen()
			}
		}},
		{ui.Cmd, ui.KeyS, "commit", s.commitPending},
		// 待提交变更：预览 SQL、丢弃（提交本身不弹窗）
		{ui.Cmd | ui.Shift, ui.KeyP, "preview", s.previewPending},
		{ui.Cmd | ui.Shift, ui.KeyBackspace, "discard", s.discardPending},
		// 导航与工具栏
		{ui.Cmd, ui.Key0, "sidebar", func() {
			if w != nil {
				w.sidebarHidden = !w.sidebarHidden
			}
		}},
		{ui.Cmd | ui.Shift, ui.KeyBackslash, "workspaces", s.openWorkspaces},
		{ui.Cmd, ui.KeyE, "new-sql", func() {
			if w != nil {
				w.newQuery("")
			}
		}},
		{ui.Cmd, ui.KeyPeriod, "processes", s.cmdProcesses},
		{ui.Cmd | ui.Shift, ui.KeyE, "export", s.cmdExport},
		{ui.Cmd | ui.Shift, ui.KeyR, "reconnect", func() {
			if w != nil {
				s.reconnect(w)
			}
		}},
		{ui.Cmd | ui.Alt, ui.KeyR, "refresh-tab", s.cmdRefreshTab},
		{ui.Cmd | ui.Ctrl, ui.KeyLeft, "back", func() {
			if w != nil {
				w.goBack()
			}
		}},
		{ui.Cmd | ui.Ctrl, ui.KeyRight, "forward", func() {
			if w != nil {
				w.goForward()
			}
		}},
		{ui.Cmd | ui.Ctrl, ui.KeyC, "console", func() { s.consoleOpen = !s.consoleOpen }},
		// 编辑器字号
		{ui.Cmd, ui.KeyEqual, "font-up", func() { s.zoomEditor(1) }},
		{ui.Cmd, ui.KeyMinus, "font-down", func() { s.zoomEditor(-1) }},
		{0, ui.KeyEscape, "deselect", func() {
			// TablePlus: Esc deselects all rows (the filter bar, a cell being
			// edited and the dialogs take Esc first)
			if t := s.activeTable(); t != nil && t.mode == 0 && !t.showFilter && t.es.editing == nil && !t.qlOpen && t.quick == nil {
				t.g.chosen.Clear()
			}
		}},
		{ui.Cmd | ui.Alt, ui.KeyF, "columns", func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.colsOpen = !t.colsOpen
			}
		}},
		{ui.Cmd, ui.KeyD, "favorite", func() {
			if q := s.activeQuery(); q != nil {
				q.favoriteCurrent()
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
		list = append(list, sc{ui.Cmd, key, fmt.Sprint("tab", i), func() {
			if w != nil && i < len(w.tabs) {
				w.active = i
			}
		}})
	}
	for _, k := range list {
		if c.Shortcut(k.mods, k.key) {
			s.fire(k.name, k.fn)
			s.redraw()
		}
	}
}

// favoriteCurrent stars the editor's SQL in the history.
func (q *queryTab) favoriteCurrent() {
	sql := strings.TrimSpace(q.ed.Buffer.String())
	if sql == "" {
		return
	}
	w := q.w
	var done bool
	w.sh.up.bg(func() {
		_ = w.sh.app.History.Add(w.cfg.ID, w.db, sql)
		items, _ := w.sh.app.History.List(w.cfg.ID, "", 200)
		for _, it := range items {
			if strings.TrimSpace(it.SQL) == sql {
				if !it.Favorite {
					_ = w.sh.app.History.ToggleFavorite(it.ID)
				}
				done = true
				return
			}
		}
	}, func() {
		if done {
			w.sh.toast("★")
		}
	})
}
