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
	lastKey   string // for fire: the last shortcut and when it ran
	lastKeyAt time.Time
	active    int // index into spaces; -1 shows the welcome screen

	connecting  map[string]bool
	dlg         connDialog
	confirm     confirmDialog
	pwp         pwPrompt
	lost        map[string]bool          // connections the keepalive lost and is retrying
	popupKey    func(k ui.TextKey) bool  // extra keys of the popup being drawn (Tab and ⌘↩ in Open Anything)
	clip        string                   // text a background job wants on the clipboard; the next frame writes it
	passwords   map[string]typedPassword // typed for a connection that asks, until it opens
	toasts      []string
	toastActs   []toastAction
	tasks       []*task
	imp         importDialog
	set         settingsDialog
	pal         palette
	sw          dbSwitcher
	quitting    bool
	requestQuit func()
	proc        processDialog
	runs        map[string]*chatMsg
	exp         exportDialog
	notes       []note
	notesOpen   bool
	notesSeen   int // how many notes the user has seen
	console     []consoleEntry
	consoleOpen bool
	wsPop       wsPopup
}

type toastAction struct {
	msg, label string
	fn         func()
}

// NewShell builds the shell over the application layer. sync makes background
// work run inline, for tests.
func NewShell(a *app.App, host app.Host, sync bool) *Shell {
	s := &Shell{app: a, host: host, active: -1, connecting: map[string]bool{}, lost: map[string]bool{}, passwords: map[string]typedPassword{}}
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
				s.toast(locales.ConnLost(name))
			} else {
				delete(s.lost, ev.ConnID)
				s.toast(locales.ConnRestored(name))
			}
		})
	})
	s.subscribeTasks()
	s.subscribeAgent()
	return s
}

// Host returns the platform adapter for app.New.
func Host(get func() *mygo.Window) app.Host { return host{win: get} }

// Open creates the main window; call it once the app is ready.
func (s *Shell) Open() {
	s.ApplyPrefs()
	mygo.App.SetMenu(s.Menu())
	mygo.App.OnLocaleChanged(func(string) {
		mygo.App.SetMenu(s.Menu())
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
		Content:       ui.View(s.view),
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

func (s *Shell) toast(msg string) {
	s.toasts = append(s.toasts, msg)
	s.notes = append(s.notes, note{at: time.Now(), text: msg})
	if len(s.notes) > 100 {
		s.notes = s.notes[len(s.notes)-100:]
		s.notesSeen = max(s.notesSeen-1, 0)
	}
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
	})
}

// openConnection connects (if needed) and shows the connection's workspace.
func (s *Shell) openConnection(cfg database.ConnectionConfig) {
	for i, w := range s.spaces {
		if w.cfg.ID == cfg.ID {
			s.active = i
			return
		}
	}
	if s.connecting[cfg.ID] {
		return
	}
	if app.NeedsPassword(cfg) && !s.passwords[cfg.ID].set {
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
		hasState = s.app.Store.Get(stateBucket, cfg.ID, &st) == nil // the connection's last session
	}, func() {
		delete(s.connecting, cfg.ID)
		if !ok {
			s.toast(locales.ConnConnectFail(cfg.Name, msg))
			return
		}
		w := newWorkspace(s, cfg)
		db, restore := initialDatabase(cfg, st, hasState)
		w.db = db
		s.spaces = append(s.spaces, w)
		s.active = len(s.spaces) - 1
		if restore {
			w.restore(st)
		}
		w.loadDatabases()
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

// openDatabase shows a database of w's connection in a workspace of its own,
// opening one when there is none yet (the original keys workspaces by
// connection and database).
func (s *Shell) openDatabase(w *Workspace, name string) {
	if name == "" || name == w.db {
		return
	}
	for i, o := range s.spaces {
		if o.cfg.ID == w.cfg.ID && o.db == name {
			s.active = i
			return
		}
	}
	nw := newWorkspace(s, w.cfg)
	nw.db, nw.dbSel, nw.dbs, nw.version = name, name, w.dbs, w.version
	var st wsState
	if s.app.Store.Get(stateBucket, nw.id(), &st) == nil && st.Database == name {
		nw.restore(st)
	}
	s.spaces = append(s.spaces, nw)
	s.active = len(s.spaces) - 1
	nw.loadTables()
}

// confirmDialog asks before something destructive.
type confirmDialog struct {
	open    bool
	title   string
	message string
	action  func()
	label   string
}

func (s *Shell) ask(title, message, label string, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, action: action, label: label}
}

// discardGuard runs fn at once when nothing would be lost, else after the user
// agrees to discard n uncommitted changes.
func (s *Shell) discardGuard(n int, fn func()) {
	if n == 0 {
		fn()
		return
	}
	s.ask(locales.EditDiscardTitle(), locales.EditDiscardMsg(n), locales.EditDiscard(), fn)
}

func (s *Shell) confirmView(c *ui.Context) {
	if !s.confirm.open {
		return
	}
	labels := []string{locales.CommonCancel(), s.confirm.label}
	switch ui.AlertDialog(c, &s.confirm.open, s.confirm.title, s.confirm.message, labels...) {
	case 1:
		act := s.confirm.action
		s.confirm = confirmDialog{}
		if act != nil {
			act()
		}
	case 0:
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
	s.toast(locales.StatusCopied())
}
