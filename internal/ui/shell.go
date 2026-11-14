// Package ui is the interface layer. It reads and writes view state and calls
// the application layer; it performs no I/O itself, because MyGo runs the view
// function on every frame. Work that blocks goes through uiThread.bg.
package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// Shell owns the main window: the workspace bar, the welcome screen and one
// Workspace per open connection.
//
// 字段按职责分进几个内嵌的小结构（见 shell_parts.go）：连接状态、会话存档、任务托盘；
// 对话框、日志面板、通知面板本来就各自成组。内嵌后字段名不变，调用方不用改，
// 但每一组的不变量写在各自的类型上，不再是一个 50 多个字段的平铺大结构。
type Shell struct {
	app  *app.App
	host app.Host
	win  *mygo.Window
	up   uiThread

	connTracker // 已保存的连接列表、正在连接 / 重连 / 丢失的连接
	sessionCtrl // 会话与状态存档
	taskTray    // 通知条与后台任务

	spaces    []*Workspace
	prefs     uiPrefs
	keyCache  map[string]chord // 命令现在的快捷键（改键后清空）
	rec       keyRecorder      // 设置页里正在录制的快捷键
	lastKey   string           // for fire: the last shortcut and when it ran
	lastKeyAt time.Time
	active    int // index into spaces; -1 shows the welcome screen

	spaceSeq     int                     // 工作区的序号计数，见 Workspace.uid
	popupKey     func(k ui.TextKey) bool // extra keys of the popup being drawn (Tab and ⌘↩ in Open Anything)
	clip         string                  // text a background job wants on the clipboard; the next frame writes it
	quitting     bool
	requestQuit  func()
	runs         map[string]*chatMsg
	dialogs                          // 各个对话框的状态（dialogs.go 里按类说明）
	consoleState                     // 日志面板（workbench.go）
	notesState                       // 通知面板（workbench.go）
	upd          updateState         // 自动更新的状态（updates.go）
	layout       layoutState         // 上次关闭时的布局（栏宽、显隐），新开的工作区按它来
	probe        func(c *ui.Context) // 测试用：每帧应用主题后调用，读取当帧的主题与配色
	unsubs       []func()            // 事件总线的退订函数，Close 时统一调用
}

// subscribe 记下一个事件订阅的退订函数。
func (s *Shell) subscribe(off func()) { s.unsubs = append(s.unsubs, off) }

// Close 退订所有事件，窗口关闭时调用。
func (s *Shell) Close() {
	if s.stopSave != nil {
		close(s.stopSave)
		s.stopSave = nil
	}
	for _, off := range s.unsubs {
		off()
	}
	s.unsubs = nil
}

type toastAction struct {
	msg, label string
	fn         func()
}

// NewShell builds the shell over the application layer. sync makes background
// work run inline, for tests.
func NewShell(a *app.App, host app.Host, sync bool) *Shell {
	s := &Shell{app: a, host: host, active: -1, connTracker: newConnTracker()}
	s.requestQuit = func() { mygo.App.Quit() }
	s.up = newUIThread(func() *mygo.Window { return s.win }, sync)
	s.writer.sync = sync
	s.subscribe(app.Subscribe(a.Bus, app.EventConnectionStatus, func(ev app.ConnectionStatusEvent) {
		s.up.post(func() {
			name := ev.ConnID
			for _, c := range s.conns {
				if c.ID == ev.ConnID {
					name = c.Name
				}
			}
			if ev.Status == "lost" {
				s.lost[ev.ConnID] = true
				s.toastWarn(locales.ConnLost(name))
			} else {
				delete(s.lost, ev.ConnID)
				s.toastOK(locales.ConnRestored(name))
			}
		})
	}))
	s.subscribeTasks()
	s.subscribeAgent()
	s.subscribeIndex()
	return s
}

// Host returns the platform adapter for app.New.
func Host(get func() *mygo.Window) app.Host { return host{win: get} }

// Open creates the main window; call it once the app is ready.
func (s *Shell) Open() {
	s.ApplyPrefs()
	s.menuApply()
	mygo.App.OnLocaleChanged(func(string) {
		s.menuApply()
		if s.win != nil {
			s.win.SetTitle(locales.AppName())
		}
	})
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:    locales.AppName(),
		Width:    1280,
		Height:   800,
		MinWidth: 960, MinHeight: 600,
		StateKey:      "main",
		TitleBarStyle: mygo.TitleBarHidden,
		// 红绿灯按钮（约 14px 高）与 36px 的工具栏垂直居中：(36-14)/2 = 11
		TrafficLightPosition: &mygo.Point{X: 14, Y: (sizeToolbar - 14) / 2},
		Content:              ui.View(s.timedView),
	})
	s.loadConnections()
	s.startAutosave()
	s.startUpdater()
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if s.quitting || e.SessionEnding {
			s.SaveState()
			return
		}
		if s.quitGuard() {
			e.PreventDefault()
			return
		}
		s.SaveState()
	})
}

// Window returns the main window, nil before Open.
func (s *Shell) Window() *mygo.Window { return s.win }

// Focus raises the main window.
func (s *Shell) Focus() {
	if s.win != nil {
		s.win.Restore()
		s.win.Focus()
	}
}

func (s *Shell) toastAction(msg, label string, fn func()) {
	s.toastActs = append(s.toastActs, toastAction{msg, label, fn})
	if s.win != nil {
		s.win.Invalidate()
	}
}

// openConnection 打开一个连接：每次都新开一个工作区，哪怕这个连接已经有工作区开着；
// 只有在工作区里切换数据库才是就地切换（switchInPlace）。
func (s *Shell) openConnection(cfg model.ConnectionConfig) { s.openConnectionThen(cfg, nil) }

// hasSpaceOf 报告连接 id 现在有没有工作区开着。
func (s *Shell) hasSpaceOf(id string) bool {
	for _, w := range s.spaces {
		if w.cfg.ID == id {
			return true
		}
	}
	return false
}

// addWorkspace 给 cfg 新开一个工作区并切到它。st、hasState 是要恢复的上次会话，没有就传零值和 false。
func (s *Shell) addWorkspace(cfg model.ConnectionConfig, st wsState, hasState bool) {
	w := newWorkspace(s, cfg)
	db, restore := initialDatabase(cfg, st, hasState)
	w.db = db
	s.spaces = append(s.spaces, w)
	s.active = len(s.spaces) - 1
	if restore {
		w.restore(st)
	}
	w.loadDatabases()
}

// openConnectionThen 同 openConnection，并在这次打开有了结果（成功、失败）之后调用 done；
// 恢复上次的连接时靠它把几个连接按顺序一个个打开。
func (s *Shell) openConnectionThen(cfg model.ConnectionConfig, done func()) {
	finish := func() {
		if done != nil {
			done()
		}
	}
	if s.connecting[cfg.ID] { // 正在连：连点两下不要开出两个
		finish()
		return
	}
	more := s.hasSpaceOf(cfg.ID) // 这个连接已经有工作区了：新开的是"另一个"，不恢复上次的标签页，免得和那个重复
	if more && !s.lost[cfg.ID] {
		// 连接还连着，新工作区直接复用它：不用再建连，也不用再输一遍密码
		s.addWorkspace(cfg, wsState{}, false)
		finish()
		return
	}
	if app.NeedsPassword(cfg) && !s.passwords[cfg.ID].set {
		finish() // 等用户输密码，不拦着后面的连接
		s.askConnectPassword(cfg, func(pw string) {
			s.passwords[cfg.ID] = typedPassword{set: true, pw: pw}
			s.openConnection(cfg)
		})
		return
	}
	pw := s.passwords[cfg.ID].pw
	delete(s.passwords, cfg.ID)
	s.connecting[cfg.ID] = true
	var connErr error
	var st wsState
	var hasState bool
	s.up.bg(func() {
		connErr = s.app.Connections.ConnectWithPassword(cfg.ID, pw)
		hasState = !more && s.app.UIState.LoadWorkspaceState(cfg.ID, &st) == nil // the connection's last session
	}, func() {
		delete(s.connecting, cfg.ID)
		if connErr != nil {
			s.toastErr(locales.ConnConnectFail(cfg.Name, errMessage(connErr)))
			finish()
			return
		}
		delete(s.lost, cfg.ID)
		s.addWorkspace(cfg, st, hasState)
		finish()
	})
}

// closeSpace closes one workspace (a connection's database); the connection
// itself is dropped with its last workspace.
// initialDatabase is the database a connection opens on: the default one set
// in the connection, if it has one, whatever the last session had open; with
// none set, the last session's. restore says whether that session's tabs
// belong to it.
func initialDatabase(cfg model.ConnectionConfig, st wsState, hasState bool) (db string, restore bool) {
	if app.CapsOf(cfg.Type).MultiDatabase && strings.TrimSpace(cfg.Database) != "" {
		return cfg.Database, hasState && st.Database == cfg.Database
	}
	if hasState && st.Database != "" {
		return st.Database, true
	}
	return cfg.Database, hasState
}

// systemDatabases are what a server lists besides the user's own.
var systemDatabases = map[string]bool{"information_schema": true, "mysql": true, "performance_schema": true, "sys": true}

// firstUserDatabase is the first database that is not a system one, or the
// first at all.
func firstUserDatabase(dbs []model.DatabaseInfo) string {
	for _, d := range dbs {
		if !systemDatabases[strings.ToLower(d.Name)] {
			return d.Name
		}
	}
	if len(dbs) > 0 {
		return dbs[0].Name
	}
	return ""
}

func (s *Shell) closeSpace(i int) {
	w := s.spaces[i]
	s.saveWorkspace(w)
	s.releaseTx(w, w.db)
	w.schemaLd.cancel() // 还在等待的补全数据加载不必再取
	last := true
	for j, o := range s.spaces {
		if j != i && o.cfg.ID == w.cfg.ID {
			last = false
		}
	}
	if last {
		id := w.cfg.ID
		s.up.bg(func() { _ = s.app.Connections.Disconnect(id) }, func() {})
	}
	s.spaces = append(s.spaces[:i], s.spaces[i+1:]...)
	switch {
	case len(s.spaces) == 0:
		s.active = -1
	case s.active >= len(s.spaces):
		s.active = len(s.spaces) - 1
	case s.active > i:
		s.active--
	}
}

// openDatabase 在 w 这个工作区里就地切换到另一个库：同一个连接只占一个工作区，
// 只有"新建/打开另一个连接"才会在左边多出一个工作区。当前库的标签页会存起来，
// 下次切回来时恢复；有没提交的修改时先问一声。
func (s *Shell) openDatabase(w *Workspace, name string) {
	if name == "" || name == w.db {
		return
	}
	s.discardGuard(w.pendingTotal(), func() {
		s.txGuard(w.txPendingTotal(), func() { s.switchInPlace(w, name) })
	})
}

// switchInPlace 把工作区换成另一个库：存下旧库的会话，清掉属于旧库的表、标签页与补全数据，
// 再恢复新库上次的会话并重新取表。
func (s *Shell) switchInPlace(w *Workspace, name string) {
	if name == "" || name == w.db {
		return
	}
	s.saveWorkspace(w)
	s.releaseTx(w, w.db)
	w.tabs, w.active, w.lastActive = nil, 0, 0
	w.back, w.fwd, w.navigating = nil, nil, false
	w.tables, w.routines, w.schema, w.keywords, w.favs = nil, nil, nil, nil, nil
	w.schemaLd.cancel() // 旧库还在等待 / 在途的补全数据加载作废
	w.filter, w.loadErr, w.tableSel = "", "", ui.ListState{}
	w.side.picked, w.side.queries, w.side.queriesAt = "", nil, time.Time{}
	w.idx = indexView{}
	if !w.ai.running { // 对话是针对旧库的；正在生成时不打断
		w.ai = newAssistant(w)
	}
	w.db, w.dbSel = name, name
	var st wsState
	if s.app.UIState.LoadWorkspaceState(w.dbKey(), &st) == nil && st.Database == name {
		w.restore(st)
	}
	w.loadTables()
}

// typedPassword is the password just typed for a connection that asks.
type typedPassword struct {
	set bool
	pw  string
}

// copyText puts text on the clipboard from outside a view function.
func (s *Shell) copyText(text string) {
	s.clip = text
	s.toastOK(locales.StatusCopied())
}
