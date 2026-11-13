package ui

import (
	"github.com/egoist/mygo"

	"minidb/locales"
)

// activeWorkspace is the workspace being shown, or nil on the welcome screen.
func (s *Shell) activeWorkspace() *Workspace {
	if s.active >= 0 && s.active < len(s.spaces) {
		return s.spaces[s.active]
	}
	return nil
}

func (s *Shell) activeQuery() *queryTab {
	if w := s.activeWorkspace(); w != nil && w.active < len(w.tabs) {
		if q, ok := w.tabs[w.active].(*queryTab); ok {
			return q.focusedPane()
		}
	}
	return nil
}

func (s *Shell) redraw() {
	if s.win != nil {
		s.win.Invalidate()
	}
}

// Commands behind the menu bar and the command palette. Each runs on the UI
// thread.

func (s *Shell) cmdNewConnection() { s.newConnectionDialog(); s.redraw() }

func (s *Shell) cmdNewQuery() {
	if w := s.activeWorkspace(); w != nil {
		w.newQuery("")
		s.redraw()
	}
}

func (s *Shell) cmdCloseTab() {
	if w := s.activeWorkspace(); w != nil {
		w.closeTab(w.active)
		s.redraw()
	}
}

func (s *Shell) cmdSettings() { s.openSettings(0); s.redraw() }

func (s *Shell) cmdPalette() { s.openPalette(); s.redraw() }

func (s *Shell) cmdToggleAI() {
	if w := s.activeWorkspace(); w != nil {
		w.togglePanel(2)
		s.redraw()
	}
}

func (s *Shell) cmdRefresh() {
	if w := s.activeWorkspace(); w != nil {
		w.loadTables()
		if w.active < len(w.tabs) {
			if tt, ok := w.tabs[w.active].(*tableTab); ok {
				tt.reload(nil)
			}
		}
		s.redraw()
	}
}

func (s *Shell) cmdCycleTab(d int) {
	if w := s.activeWorkspace(); w != nil && len(w.tabs) > 0 {
		w.active = (w.active + d + len(w.tabs)) % len(w.tabs)
		s.redraw()
	}
}

func (s *Shell) cmdRun(all bool) {
	if q := s.activeQuery(); q != nil {
		q.run(all)
		s.redraw()
	}
}

func (s *Shell) cmdFormat() {
	if q := s.activeQuery(); q != nil {
		q.format()
		s.redraw()
	}
}

func (s *Shell) cmdProcesses() {
	if w := s.activeWorkspace(); w != nil {
		s.openProcesses(w)
		s.redraw()
	}
}

// openWorkspaces 打开“所有工作区”列表（⌘⇧\）。
func (s *Shell) openWorkspaces() {
	if len(s.spaces) == 0 {
		return
	}
	s.wsPop = wsPopup{open: true}
}

// cmdExport 导出当前标签：表标签导出整张表，查询标签导出当前结果的语句。
func (s *Shell) cmdExport() {
	w := s.activeWorkspace()
	if w == nil || w.active >= len(w.tabs) {
		return
	}
	switch t := w.tabs[w.active].(type) {
	case *tableTab:
		s.exportTable(w, t.table)
	case *queryTab:
		if t.shown >= 0 && t.shown < len(t.results) {
			s.exportQuery(w, t.results[t.shown].stmt)
		}
	}
	s.redraw()
}

// cmdRefreshTab 只刷新当前标签（⌘⌥R），不重新加载侧栏的表列表。
func (s *Shell) cmdRefreshTab() {
	w := s.activeWorkspace()
	if w == nil || w.active >= len(w.tabs) {
		return
	}
	if tt, ok := w.tabs[w.active].(*tableTab); ok {
		tt.reload(nil)
		s.redraw()
	}
}

// 编辑器字号的范围与默认值（DIP）。
const (
	editorFontMin     = 9
	editorFontMax     = 32
	editorFontDefault = 13
)

// zoomEditor 调整所有 SQL 编辑器的字号（⌘= / ⌘-），并记入偏好。
func (s *Shell) zoomEditor(d int) {
	if s.activeQuery() == nil {
		return
	}
	size := s.prefs.EditorFont
	if size == 0 {
		size = editorFontDefault
	}
	size = min(max(size+float32(d), editorFontMin), editorFontMax)
	s.prefs.EditorFont = size
	s.savePrefs()
	s.redraw()
}

// Menu builds the menu bar. Call again when the language changes.
func (s *Shell) Menu() *mygo.Menu {
	// id 是命令表里的命令，菜单上的快捷键跟着设置走；id 为空则不挂快捷键。
	// 正在录制新快捷键时先摘掉，免得按下的键被菜单抢去执行。
	item := func(label, id string, fn func()) *mygo.MenuItem {
		accel := ""
		if id != "" && !s.rec.active() {
			accel = s.chordOf(id).accelerator()
		}
		return &mygo.MenuItem{Label: label, Accelerator: accel, Click: func(*mygo.MenuItem, *mygo.Window) { fn() }}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.MenuFile(), Submenu: []*mygo.MenuItem{
			item(locales.MenuNewConnection(), "new-conn", s.cmdNewConnection),
			item(locales.MenuNewQuery(), "new-query", s.cmdNewQuery),
			mygo.Separator(),
			item(locales.MenuCloseTab(), "close-tab", s.cmdCloseTab),
			{Role: mygo.RoleClose, Label: locales.MenuCloseWindow(), Accelerator: "CmdOrCtrl+Shift+W"},
			mygo.Separator(),
			item(locales.MenuSettings(), "settings", s.cmdSettings),
		}},
		{Role: mygo.RoleEditMenu},
		{Label: locales.MenuQuery(), Submenu: []*mygo.MenuItem{
			item(locales.MenuRun(), "run", func() { s.cmdRun(false) }),
			item(locales.MenuRunAll(), "run-all", func() { s.cmdRun(true) }),
			item(locales.MenuFormat(), "format", s.cmdFormat),
			mygo.Separator(),
			item(locales.MenuProcesses(), "processes", s.cmdProcesses),
			item(locales.MenuExport(), "export", s.cmdExport),
		}},
		{Label: locales.MenuView(), Submenu: []*mygo.MenuItem{
			item(locales.MenuPalette(), "palette", s.cmdPalette),
			item(locales.ToolbarSwitchDatabase(), "switcher", s.openSwitcher),
			item(locales.MenuToggleAI(), "ai", s.cmdToggleAI),
			item(locales.MenuRefresh(), "refresh", s.cmdRefresh),
			item(locales.MenuRefreshTab(), "refresh-tab", s.cmdRefreshTab),
			item(locales.ToolbarReconnect(), "reconnect", func() {
				if w := s.activeWorkspace(); w != nil {
					s.reconnect(w)
				}
			}),
			mygo.Separator(),
			item(locales.ToolbarSidebar(), "sidebar", func() {
				if w := s.activeWorkspace(); w != nil {
					w.sidebarHidden = !w.sidebarHidden
					s.redraw()
				}
			}),
			item(locales.ToolbarConsole(), "", func() { s.consoleOpen = !s.consoleOpen; s.redraw() }),
			item(locales.WsAllTitle(), "workspaces", func() { s.openWorkspaces(); s.redraw() }),
			mygo.Separator(),
			item(locales.MenuBack(), "", func() {
				if w := s.activeWorkspace(); w != nil {
					w.goBack()
					s.redraw()
				}
			}),
			item(locales.MenuForward(), "", func() {
				if w := s.activeWorkspace(); w != nil {
					w.goForward()
					s.redraw()
				}
			}),
			item(locales.MenuFontUp(), "font-up", func() { s.zoomEditor(1) }),
			item(locales.MenuFontDown(), "font-down", func() { s.zoomEditor(-1) }),
			mygo.Separator(),
			item(locales.MenuNextTab(), "tab-next", func() { s.cmdCycleTab(1) }),
			item(locales.MenuPrevTab(), "tab-prev", func() { s.cmdCycleTab(-1) }),
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// menuApply 把当前键位的菜单装到应用上。
func (s *Shell) menuApply() { mygo.App.SetMenu(s.Menu()) }
