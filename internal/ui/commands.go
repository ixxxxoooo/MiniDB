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
		w.loadTables() // 开着的项目概览会跟着刷新
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

// cmdInlineAI 打开 / 收起当前查询标签的内联 AI 栏（⌘J）。
func (s *Shell) cmdInlineAI() {
	if q := s.activeQuery(); q != nil {
		q.inline().toggle()
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

// cmd 是命令表里某个命令的菜单动作：执行时才取动作（工作区随时会变），做完重画。
func (s *Shell) cmd(id string) func() {
	return func() {
		if fn := s.commandRun(id); fn != nil {
			fn()
		}
		s.redraw()
	}
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
			item(locales.MenuNewConnection(), "new-conn", s.cmd("new-conn")),
			item(locales.MenuNewQuery(), "new-query", s.cmd("new-query")),
			mygo.Separator(),
			item(locales.MenuCloseTab(), "close-tab", s.cmd("close-tab")),
			{Role: mygo.RoleClose, Label: locales.MenuCloseWindow(), Accelerator: "CmdOrCtrl+Shift+W"},
			mygo.Separator(),
			item(locales.MenuSettings(), "settings", s.cmd("settings")),
		}},
		{Role: mygo.RoleEditMenu},
		{Label: locales.MenuQuery(), Submenu: []*mygo.MenuItem{
			item(locales.MenuRun(), "run", s.cmd("run")),
			item(locales.MenuRunAll(), "run-all", s.cmd("run-all")),
			item(locales.MenuFormat(), "format", s.cmd("format")),
			item(locales.ScInlineAI(), "inline-ai", s.cmd("inline-ai")),
			item(locales.ScExplain(), "explain", s.cmd("explain")),
			mygo.Separator(),
			item(locales.MonitorTitle(), "monitor", s.cmd("monitor")),
			item(locales.MenuProcesses(), "processes", s.cmd("processes")),
			item(locales.MenuExport(), "export", s.cmd("export")),
		}},
		{Label: locales.MenuView(), Submenu: []*mygo.MenuItem{
			item(locales.MenuPalette(), "palette", s.cmd("palette")),
			item(locales.ToolbarSwitchDatabase(), "switcher", s.cmd("switcher")),
			item(locales.MenuToggleAI(), "ai", s.cmdToggleAI),
			item(locales.MenuRefresh(), "refresh", s.cmd("refresh")),
			item(locales.MenuRefreshTab(), "refresh-tab", s.cmd("refresh-tab")),
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
			item(locales.WsAllTitle(), "workspaces", s.cmd("workspaces")),
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
			item(locales.MenuFontUp(), "font-up", s.cmd("font-up")),
			item(locales.MenuFontDown(), "font-down", s.cmd("font-down")),
			mygo.Separator(),
			item(locales.MenuNextTab(), "tab-next", s.cmd("tab-next")),
			item(locales.MenuPrevTab(), "tab-prev", s.cmd("tab-prev")),
			item(locales.KmNextPane(), "pane-next", s.cmd("pane-next")),
			item(locales.KmPrevPane(), "pane-prev", s.cmd("pane-prev")),
			mygo.Separator(),
			item(locales.KmShowData(), "show-data", s.cmd("show-data")),
			item(locales.KmShowStructure(), "show-structure", s.cmd("show-structure")),
			item(locales.ScColumns(), "columns", s.cmd("columns")),
			item(locales.ScJumpColumn(), "jump-column", s.cmd("jump-column")),
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// menuApply 把当前键位的菜单装到应用上。
func (s *Shell) menuApply() { mygo.App.SetMenu(s.Menu()) }
