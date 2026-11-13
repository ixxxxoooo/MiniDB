package ui

import (
	"time"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/locales"
)

const stateBucket = "workspace_state"

// sessionKey 是"上次关闭时打开了哪些连接"的存档，放在 settings 桶里。
const sessionKey = "open_session"

// sessionState 是应用级的会话：关闭前依次打开的连接，以及当时在前台的那个。
// 每个连接自己的标签页、查询草稿由 wsState 另存，重新打开连接时一并恢复。
type sessionState struct {
	Conns  []string `json:"conns"`
	Active string   `json:"active,omitempty"`
}

// tabState is what is kept of an open tab.
type tabState struct {
	Kind  string `json:"kind"` // "table" | "query"
	Table string `json:"table,omitempty"`
	Mode  int    `json:"mode,omitempty"`
	SQL   string `json:"sql,omitempty"`
}

// wsState is a workspace's saved session: the tabs and query drafts, restored
// the next time the connection opens.
type wsState struct {
	Database string     `json:"database"`
	Tabs     []tabState `json:"tabs"`
	Active   int        `json:"active"`
	Panel    int        `json:"panel"`
}

func (w *Workspace) snapshot() wsState {
	st := wsState{Database: w.db, Active: w.active, Panel: w.panel}
	for _, tb := range w.tabs {
		switch t := tb.(type) {
		case *tableTab:
			if t.isNew {
				continue // 还没建出来的表不进会话
			}
			st.Tabs = append(st.Tabs, tabState{Kind: "table", Table: t.table, Mode: t.mode})
		case *overviewTab:
			st.Tabs = append(st.Tabs, tabState{Kind: "overview"})
		case *queryTab:
			st.Tabs = append(st.Tabs, tabState{Kind: "query", SQL: t.ed.Buffer.String()})
		}
	}
	return st
}

// restore reopens the saved tabs. Table tabs load their data when first shown.
func (w *Workspace) restore(st wsState) {
	for _, ts := range st.Tabs {
		switch ts.Kind {
		case "table":
			t := newTableTab(w, ts.Table)
			t.mode = ts.Mode
			w.tabs = append(w.tabs, t)
		case "overview":
			w.openOverview()
		case "query":
			w.newQuery(ts.SQL)
		}
	}
	w.active = min(max(st.Active, 0), max(len(w.tabs)-1, 0))
	w.panel = st.Panel
}

// saveWorkspace keeps a workspace's session under its own key, and as the
// connection's last session, which the next connect restores.
func (s *Shell) saveWorkspace(w *Workspace) {
	st := w.snapshot()
	_ = s.app.Store.Put(stateBucket, w.dbKey(), st)
	_ = s.app.Store.Put(stateBucket, w.cfg.ID, st)
}

// SaveState writes the sessions of every open workspace; the active one
// of a connection is the last, so it is what the connection reopens with.
func (s *Shell) SaveState() {
	for i, w := range s.spaces {
		if i != s.active {
			s.saveWorkspace(w)
		}
	}
	if s.active >= 0 && s.active < len(s.spaces) {
		s.saveWorkspace(s.spaces[s.active])
	}
	s.saveSession()
}

// saveSession 记下现在打开的连接。恢复还没做完时不写，免得把上次的存档用一个空列表盖掉。
func (s *Shell) saveSession() {
	if !s.sessionReady {
		return
	}
	st := sessionState{}
	for _, w := range s.spaces {
		st.Conns = append(st.Conns, w.cfg.ID)
	}
	if s.active >= 0 && s.active < len(s.spaces) {
		st.Active = s.spaces[s.active].cfg.ID
	}
	_ = s.app.Store.Put("settings", sessionKey, st)
}

// restoreSession 在连接列表第一次读出来之后，按上次的顺序把连接逐个重新打开，
// 并回到上次在前台的那个。需要现输密码的连接不自动打开，免得一启动就弹出一串密码框。
func (s *Shell) restoreSession() {
	if s.restored {
		return
	}
	s.restored = true
	var st sessionState
	if s.prefs.NoRestore || s.app.Store.Get("settings", sessionKey, &st) != nil {
		s.sessionReady = true
		return
	}
	var queue []database.ConnectionConfig
	for _, id := range st.Conns {
		for _, cfg := range s.conns {
			if cfg.ID == id && !app.NeedsPassword(cfg) {
				queue = append(queue, cfg)
			}
		}
	}
	logger.Info("[Session] 恢复上次打开的连接: %d 个", len(queue))
	s.restoreNext(queue, st.Active)
}

// restoreNext 打开队列里的下一个连接，等它完成（成功或失败）再开下一个，保证标签顺序不乱。
func (s *Shell) restoreNext(queue []database.ConnectionConfig, active string) {
	if len(queue) == 0 {
		for i, w := range s.spaces {
			if w.cfg.ID == active {
				s.active = i
			}
		}
		s.sessionReady = true
		s.redraw()
		return
	}
	s.openConnectionThen(queue[0], func() { s.restoreNext(queue[1:], active) })
}

// startAutosave saves the sessions every few seconds, on the UI thread.
func (s *Shell) startAutosave() {
	go func() {
		for range time.Tick(8 * time.Second) {
			s.up.post(s.SaveState)
		}
	}()
}

// pendingEverywhere counts uncommitted changes in all workspaces.
func (s *Shell) pendingEverywhere() int {
	n := 0
	for _, w := range s.spaces {
		n += w.pendingTotal()
	}
	return n
}

func (s *Shell) quitGuard() (cancel bool) {
	n := s.pendingEverywhere()
	if n == 0 {
		return false
	}
	s.askWarn(locales.QuitTitle(), locales.QuitMsg(n), locales.QuitConfirm(), func() {
		s.quitting = true
		s.requestQuit()
	})
	s.redraw()
	return true
}
