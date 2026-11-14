// Package ui is the interface layer. It reads and writes view state and calls
// the application layer; it performs no I/O itself, because MyGo runs the view
// function on every frame. Work that blocks goes through uiThread.bg.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/locales"
)

// Shell owns the main window: the workspace bar, the welcome screen and one
// Workspace per open connection.
type Shell struct {
	app  *app.App
	host app.Host
	win  *mygo.Window
	up   uiThread

	conns      []database.ConnectionConfig
	connsErr   string
	connsReady bool
	selected   string // connection chosen on the welcome screen

	spaces    []*Workspace
	prefs     uiPrefs
	keyCache  map[string]chord // 命令现在的快捷键（改键后清空）
	rec       keyRecorder      // 设置页里正在录制的快捷键
	lastKey   string           // for fire: the last shortcut and when it ran
	lastKeyAt time.Time
	active    int // index into spaces; -1 shows the welcome screen

	connecting     map[string]bool
	dlg            connDialog
	confirm        confirmDialog
	pwp            pwPrompt
	prm            promptDialog
	edOpt          bool // the editor options dialog
	srch           dbSearch
	lost           map[string]bool          // connections the keepalive lost and is retrying
	spaceSeq       int                      // 工作区的序号计数，见 Workspace.uid
	popupKey       func(k ui.TextKey) bool  // extra keys of the popup being drawn (Tab and ⌘↩ in Open Anything)
	clip           string                   // text a background job wants on the clipboard; the next frame writes it
	passwords      map[string]typedPassword // typed for a connection that asks, until it opens
	toasts         []toastReq
	toastActs      []toastAction
	tasks          []*task
	imp            importDialog
	set            settingsDialog
	pal            palette
	sw             dbSwitcher
	quitting       bool
	requestQuit    func()
	proc           processDialog
	runs           map[string]*chatMsg
	exp            exportDialog
	bak            dumpDialog
	usr            usersDialog
	rst            restoreDialog
	notes          []note
	notesOpen      bool
	notesSeen      int // how many notes the user has seen
	console        []consoleEntry
	consoleOpen    bool
	consoleErrOnly bool           // 日志只看失败的
	consoleScroll  ui.ScrollState // 日志的滚动位置（要自己控制跟随末尾）
	consoleSeen    int            // 上次画日志时有多少条，用来发现新语句
	consolePct     float32        // 主区域占窗口的百分比，其余是日志；拖动分隔条改变
	layout         layoutState    // 上次关闭时的布局（栏宽、显隐），新开的工作区按它来
	wsPop          wsPopup
	probe          func(c *ui.Context) // 测试用：每帧应用主题后调用，读取当帧的主题与配色
	reconnecting   map[string]bool     // 正在重连的连接
	reconnects     int                 // 重连触发的次数（测试用）
	restored       bool                // 上次的连接已经（或不需要）恢复过了
	sessionReady   bool                // 恢复完成后才允许存档，避免用空列表覆盖上次的会话
}

type toastAction struct {
	msg, label string
	fn         func()
}

// NewShell builds the shell over the application layer. sync makes background
// work run inline, for tests.
func NewShell(a *app.App, host app.Host, sync bool) *Shell {
	s := &Shell{app: a, host: host, active: -1, connecting: map[string]bool{}, lost: map[string]bool{}, reconnecting: map[string]bool{}, passwords: map[string]typedPassword{}}
	s.requestQuit = func() { mygo.App.Quit() }
	s.up = uiThread{win: func() *mygo.Window { return s.win }, sync: sync}
	a.Bus.On(app.EventConnectionStatus, func(p any) {
		ev, ok := p.(app.ConnectionStatusEvent)
		if !ok {
			return
		}
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
	})
	tipRedraw = func() { s.up.post(func() {}) } // 悬浮提示计时到点后叫醒界面
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
		Content:              ui.View(s.view),
	})
	s.loadConnections()
	s.startAutosave()
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

func (s *Shell) loadConnections() {
	var conns []database.ConnectionConfig
	var err error
	s.up.bg(func() { conns, err = s.app.Connections.GetConnections() }, func() {
		s.connsReady = true
		if err != nil {
			s.connsErr = err.Error()
			return
		}
		s.connsErr = ""
		s.conns = conns
		s.restoreSession()
	})
}

// openConnection 打开一个连接：每次都新开一个工作区，哪怕这个连接已经有工作区开着；
// 只有在工作区里切换数据库才是就地切换（switchInPlace）。
func (s *Shell) openConnection(cfg database.ConnectionConfig) { s.openConnectionThen(cfg, nil) }

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
func (s *Shell) addWorkspace(cfg database.ConnectionConfig, st wsState, hasState bool) {
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
func (s *Shell) openConnectionThen(cfg database.ConnectionConfig, done func()) {
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
	var ok bool
	var msg string
	var st wsState
	var hasState bool
	s.up.bg(func() {
		ok, msg = s.app.Connections.ConnectWithPassword(cfg.ID, pw)
		hasState = !more && s.app.Store.Get(stateBucket, cfg.ID, &st) == nil // the connection's last session
	}, func() {
		delete(s.connecting, cfg.ID)
		if !ok {
			s.toastErr(locales.ConnConnectFail(cfg.Name, msg))
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
func initialDatabase(cfg database.ConnectionConfig, st wsState, hasState bool) (db string, restore bool) {
	if cfg.Type != "sqlite" && strings.TrimSpace(cfg.Database) != "" {
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
func firstUserDatabase(dbs []database.DatabaseInfo) string {
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

// closeSpaceGuarded asks first when the workspace has uncommitted changes.
func (s *Shell) closeSpaceGuarded(w *Workspace) {
	s.discardGuard(w.pendingTotal(), func() {
		for i, o := range s.spaces {
			if o == w {
				s.closeSpace(i)
				return
			}
		}
	})
}

// closeConnection closes every workspace of a connection (the toolbar's
// disconnect), after asking if any holds uncommitted changes.
func (s *Shell) closeConnection(id string) {
	n := 0
	for _, w := range s.spaces {
		if w.cfg.ID == id {
			n += w.pendingTotal()
		}
	}
	s.discardGuard(n, func() {
		for i := len(s.spaces) - 1; i >= 0; i-- {
			if s.spaces[i].cfg.ID == id {
				s.closeSpace(i)
			}
		}
	})
}

// openDatabase 在 w 这个工作区里就地切换到另一个库：同一个连接只占一个工作区，
// 只有"新建/打开另一个连接"才会在左边多出一个工作区。当前库的标签页会存起来，
// 下次切回来时恢复；有没提交的修改时先问一声。
func (s *Shell) openDatabase(w *Workspace, name string) {
	if name == "" || name == w.db {
		return
	}
	s.discardGuard(w.pendingTotal(), func() { s.switchInPlace(w, name) })
}

// switchInPlace 把工作区换成另一个库：存下旧库的会话，清掉属于旧库的表、标签页与补全数据，
// 再恢复新库上次的会话并重新取表。
func (s *Shell) switchInPlace(w *Workspace, name string) {
	if name == "" || name == w.db {
		return
	}
	s.saveWorkspace(w)
	w.tabs, w.active, w.lastActive = nil, 0, 0
	w.back, w.fwd, w.navigating = nil, nil, false
	w.tables, w.routines, w.schema, w.keywords, w.favs = nil, nil, nil, nil, nil
	w.filter, w.loadErr, w.tableSel = "", "", ui.ListState{}
	w.side.picked, w.side.queries, w.side.queriesAt = "", nil, time.Time{}
	w.idx = indexView{}
	if !w.ai.running { // 对话是针对旧库的；正在生成时不打断
		w.ai = newAssistant(w)
	}
	w.db, w.dbSel = name, name
	var st wsState
	if s.app.Store.Get(stateBucket, w.dbKey(), &st) == nil && st.Database == name {
		w.restore(st)
	}
	w.loadTables()
}

// confirmDialog asks before something destructive.
// 确认框的基调：决定图标和确认按钮的颜色。
const (
	toneInfo   = iota // 普通确认：强调色
	toneWarn          // 会丢掉没保存的东西：橙色
	toneDanger        // 不可恢复的破坏性操作：红色
)

type confirmDialog struct {
	open    bool
	title   string
	message string
	action  func()
	label   string
	tone    int
	opts    []*confirmOpt // 说明下面的复选项（比如清空、删除表时的 Cascade），空则没有
}

// confirmOpt 是确认框里的一个复选项；点确认时 on 就是用户的选择。
type confirmOpt struct {
	label string
	on    bool
}

// askDangerOpts 同 askDanger，另在说明下面列出复选项 opts，action 里读 opts 的 on。
func (s *Shell) askDangerOpts(title, message, label string, opts []*confirmOpt, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, action: action, label: label, tone: toneDanger, opts: opts}
}

func (s *Shell) askWarn(title, message, label string, action func()) {
	s.askTone(toneWarn, title, message, label, action)
}

func (s *Shell) askDanger(title, message, label string, action func()) {
	s.askTone(toneDanger, title, message, label, action)
}

func (s *Shell) askTone(tone int, title, message, label string, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, action: action, label: label, tone: tone}
}

// discardGuard runs fn at once when nothing would be lost, else after the user
// agrees to discard n uncommitted changes.
func (s *Shell) discardGuard(n int, fn func()) {
	if n == 0 {
		fn()
		return
	}
	s.askWarn(locales.EditDiscardTitle(), locales.EditDiscardMsg(n), locales.EditDiscard(), fn)
}

// confirmView 是应用统一的确认框：左上一枚按基调配色的图标，标题与说明，
// 右下取消和确认两个按钮。破坏性操作的确认按钮是红色，默认焦点在"取消"上，
// 回车不会直接删东西；其余情况默认焦点在确认键，回车即确认；Esc 取消。
func (s *Shell) confirmView(c *ui.Context) {
	d := &s.confirm
	if !d.open {
		return
	}
	p := s.tokens(c)
	col, icon := p.accent, icQuestion
	switch d.tone {
	case toneWarn:
		col, icon = p.warning, icAlertTri
	case toneDanger:
		col, icon = p.danger, icTrash
	}
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	confirmed, cancelled := false, false
	ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(400).Children(func() {
			ui.Row(c).Gap(14).AlignItems(ui.Start).Padding(20, 22, 18, 20).FillWidth().Children(func() {
				ui.Row(c).Size(40, 40).Shrink(0).Center().Radius(20).Background(col.Alpha(0.13)).Children(func() {
					ui.Icon(c, icon).FontSize(19).TextColor(col)
				})
				ui.Column(c).Gap(6).Grow(1).MinWidth(0).Padding(2, 0, 0, 0).Children(func() {
					ui.Text(c, d.title).FontWeight(600).FontSize(15)
					ui.Text(c, d.message).FontSize(fontXS).TextColor(p.fgSecondary).Selectable()
					for i, o := range d.opts {
						ui.Checkbox(c.Key("confirm-opt"+itoa(i)), &o.on, o.label).Margin(4, 0, 0, 0)
					}
				})
			})
			hairline(c, p.borderSubtle)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Padding(10, 14).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
				// 默认焦点：破坏性操作落在"取消"上（回车是安全的），其余落在确认键上
				cancel := footBtn(c.Key("confirm-cancel"), locales.CommonCancel())
				ok := footBtn(c.Key("confirm-ok"), d.label).FontWeight(600).TextColor(ui.Hex("#ffffff")).Background(col).BorderWidth(0)
				if d.tone == toneDanger {
					cancel.AutoFocus()
				} else {
					ok.AutoFocus()
				}
				if cancel.Clicked() {
					cancelled = true
				}
				if ok.Hovered() {
					ok.Background(col.Alpha(0.88))
				}
				if ok.Clicked() {
					confirmed = true
				}
			})
		})
	}).Padding(0).Clip()
	switch {
	case confirmed:
		act := d.action
		s.confirm = confirmDialog{}
		if act != nil {
			act()
		}
	case cancelled || !d.open:
		s.confirm = confirmDialog{}
	}
}

var _ = fmt.Sprintf

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
