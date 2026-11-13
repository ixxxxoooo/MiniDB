package ui

import (
	"time"

	"minidb/locales"
)

const stateBucket = "workspace_state"

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
			st.Tabs = append(st.Tabs, tabState{Kind: "table", Table: t.table, Mode: t.mode})
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
	_ = s.app.Store.Put(stateBucket, w.id(), st)
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
	s.ask(locales.QuitTitle(), locales.QuitMsg(n), locales.QuitConfirm(), func() {
		s.quitting = true
		s.requestQuit()
	})
	s.redraw()
	return true
}
