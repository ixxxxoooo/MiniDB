package ui

// commandRun 返回命令 id 对应的动作；没有登记的 id（编辑器自己处理的几个）返回 nil。
// 每次调用都按"此刻的活动工作区"重新取，所以菜单、快捷键、命令面板都可以随时调用，不会拿到过期的工作区。
func (s *Shell) commandRun(id string) func() { return s.commandRuns()[id] }

// editorHandled 是编辑器里由编辑器自己处理按键的命令：全局快捷键循环跳过它们，免得同一个键执行两次。
var editorHandled = map[string]bool{"run": true, "run-all": true, "format": true, "inline-ai": true, "explain": true}

// commandRuns 是命令 id → 动作 的表。命令本身（id、分组、默认键）只在 keymap.go 的 commands 里登记一次；
// 这里只放"按下以后做什么"。TestEveryCommandIsWired 保证两边对得上：加命令忘了写动作会直接失败。
func (s *Shell) commandRuns() map[string]func() {
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
	return map[string]func(){
		"new-conn":  func() { s.newConnectionDialog() },
		"open-conn": func() { s.newConnectionDialog() },
		"new-query": func() {
			if w != nil {
				w.newQuery("")
			}
		},
		"switcher": func() { s.openSwitcher() },
		"palette":  func() { s.openPalette() },
		"settings": func() { s.openSettings(0) },
		"close-tab": func() {
			if w != nil && len(w.tabs) > 0 {
				w.closeTab(w.active)
			}
		},
		"ai": func() {
			if t := s.activeTable(); t != nil && t.filterFocused {
				return // in the filter bar ⌘I adds a condition
			}
			if w != nil {
				w.togglePanel(2)
			}
		},
		"refresh":  s.cmdRefresh,
		"tab-prev": func() { s.cmdCycleTab(-1) },
		"tab-next": func() { s.cmdCycleTab(1) },
		"ws-prev":  func() { cycleWS(-1) },
		"ws-next":  func() { cycleWS(1) },
		"sub-prev": func() { subView(-1) },
		"sub-next": func() { subView(1) },
		"filter": func() {
			// 鼠标在左侧栏上时，⌘F 搜索的是侧栏里的列表（表、查询或历史，停在哪一段就搜哪一段）
			if w != nil && w.sideHover {
				w.side.focusFilter = true
				return
			}
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.toggleFilter()
			}
		},
		"side-filter": func() {
			// 查询标签里 ⌘⇧F 是格式化，由编辑器处理
			if w != nil && s.activeQuery() == nil {
				w.sidebarHidden = false
				w.side.seg = sideItems
				w.side.focusFilter = true
			}
		},
		"disconnect": func() {
			if w != nil {
				s.closeConnection(w.cfg.ID)
			}
		},
		"fullscreen": func() {
			if s.win != nil {
				s.win.ToggleFullScreen()
			}
		},
		"commit": s.commitPending,
		// 待提交变更：预览 SQL、丢弃（提交本身不弹窗）
		"preview": s.previewPending,
		"discard": s.discardPending,
		// 导航与工具栏
		"sidebar": func() {
			if w != nil {
				w.sidebarHidden = !w.sidebarHidden
			}
		},
		"workspaces": s.openWorkspaces,
		"new-sql": func() {
			if w != nil {
				w.newQuery("")
			}
		},
		"processes": s.cmdProcesses,
		"export":    s.cmdExport,
		"reconnect": func() {
			if w != nil {
				s.reconnect(w)
			}
		},
		"refresh-tab": s.cmdRefreshTab,
		"back": func() {
			if w != nil {
				w.goBack()
			}
		},
		"forward": func() {
			if w != nil {
				w.goForward()
			}
		},
		"console": func() { s.consoleOpen = !s.consoleOpen },
		// 编辑器字号
		"font-up":   func() { s.zoomEditor(1) },
		"font-down": func() { s.zoomEditor(-1) },
		"columns": func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.colsOpen = !t.colsOpen
			}
		},
		"jump-column": func() {
			if t := s.activeTable(); t != nil && t.mode == 0 {
				t.openJump()
			}
		},
		"favorite": func() {
			if q := s.activeQuery(); q != nil {
				q.toggleFavorite()
			}
		},
		// 下面几个在编辑器里由编辑器处理按键，这里给菜单与命令面板用
		"run":       func() { s.cmdRun(false) },
		"run-all":   func() { s.cmdRun(true) },
		"format":    s.cmdFormat,
		"inline-ai": s.cmdInlineAI,
		"explain":   s.cmdExplain,
	}
}
