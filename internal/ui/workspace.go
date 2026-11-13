package ui

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// tab is one page of a workspace: a table, a query, ...
type tab interface {
	title() string
	icon() *ui.SVG
	view(c *ui.Context, p tokens)
	// pending reports unsaved edits, for the close guard and the status bar.
	pending() int
}

// Workspace is the window content of one open connection.
type Workspace struct {
	sh  *Shell
	cfg database.ConnectionConfig

	dbs           []database.DatabaseInfo
	db            string
	dbSel         string
	tables        []database.TableInfo
	routines      []database.RoutineInfo
	loading       bool
	loadErr       string
	filter        string
	tableSel      ui.ListState
	side          sideState
	loadStart     time.Time
	midKey        string // the tab a held middle button already closed
	sidebarHidden bool
	back, fwd     []int // tabs visited, for ⌘⌃← and ⌘⌃→
	lastActive    int
	navigating    bool

	tabs     []tab
	active   int
	queryN   int
	sidebarW float32
	version  string
	schema   []sqltext.Table
	ai       *assistant
	panelPct float32
	panel    int // 0 none, 1 inspector, 2 assistant
	panelW   float32
}

func newWorkspace(s *Shell, cfg database.ConnectionConfig) *Workspace {
	w := &Workspace{sh: s, cfg: cfg, db: cfg.Database, sidebarW: 260, panelW: 340, panelPct: 66}
	w.ai = newAssistant(w)
	return w
}

func (w *Workspace) loadDatabases() {
	w.loading = true
	w.loadStart = time.Now()
	var dbs []database.DatabaseInfo
	var err error
	var ver string
	w.sh.up.bg(func() {
		dbs, err = w.sh.app.Databases.GetDatabases(w.cfg.ID)
		ver, _ = w.sh.app.Databases.GetServerVersion(w.cfg.ID)
	}, func() {
		w.loading = false
		w.version = ver
		if err != nil {
			w.loadErr = err.Error()
			return
		}
		w.dbs = dbs
		if w.db == "" {
			w.db = firstUserDatabase(dbs)
		}
		w.dbSel = w.db
		w.loadTables()
	})
}

// id names the workspace: its connection and database, as the original does.
func (w *Workspace) id() string { return w.cfg.ID + ":" + w.db }

// switchDatabase opens the database in its own workspace.
func (w *Workspace) switchDatabase(name string) { w.sh.openDatabase(w, name) }

func (w *Workspace) loadTables() {
	w.loading, w.loadErr = true, ""
	w.loadStart = time.Now()
	db := w.db
	var tables []database.TableInfo
	var err error
	w.sh.up.bg(func() { tables, err = w.sh.app.Databases.GetTables(w.cfg.ID, db) }, func() {
		if db != w.db {
			return // switched meanwhile
		}
		w.loading = false
		if err != nil {
			w.loadErr = err.Error()
			return
		}
		w.tables = tables
		w.loadPins()
		w.loadRoutines()
		if w.sh.win == nil {
			w.loadSchema()
			return
		}
		// completion data is not needed yet; keep the first moments free for
		// what the user opened
		go func() {
			time.Sleep(1500 * time.Millisecond)
			w.sh.up.post(w.loadSchema)
		}()
	})
}

// loadSchema fetches columns of every table for SQL completion.
func (w *Workspace) loadSchema() {
	db := w.db
	var out []sqltext.Table
	w.sh.up.bg(func() {
		sc, err := w.sh.app.Schema.GetSchema(context.Background(), w.cfg.ID, db)
		if err != nil || sc == nil {
			return
		}
		for _, t := range sc.Tables {
			tb := sqltext.Table{Name: t.Name}
			for _, c := range t.Columns {
				tb.Columns = append(tb.Columns, sqltext.Column{Name: c.Name, Type: c.Type})
			}
			out = append(out, tb)
		}
	}, func() {
		if db == w.db && out != nil {
			w.schema = out
		}
	})
}

// visibleTables applies the sidebar filter.
func (w *Workspace) visibleTables() []database.TableInfo {
	q := strings.ToLower(strings.TrimSpace(w.filter))
	if q == "" {
		return w.tables
	}
	var out []database.TableInfo
	for _, t := range w.tables {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	return out
}

// openTable shows a table in a tab of its own; a preview tab on it is kept.
func (w *Workspace) openTable(name string) {
	for i, t := range w.tabs {
		if tt, ok := t.(*tableTab); ok && tt.table == name {
			w.active = i
			tt.preview = false
			return
		}
	}
	tt := newTableTab(w, name)
	w.tabs = append(w.tabs, tt)
	w.active = len(w.tabs) - 1
	tt.load()
}

// openTableFiltered shows a table with one condition (col = value) applied: a
// foreign key's target. A tab on that table is reused.
func (w *Workspace) openTableFiltered(table, col, value string) {
	apply := func(tt *tableTab) {
		tt.keep()
		tt.showFilter = true
		tt.filters = []filterRow{{on: true, col: col, op: "=", val: value}}
		tt.filterOr = false
		tt.page = 1
		tt.setFilter()
		tt.load()
	}
	for i, t := range w.tabs {
		if tt, ok := t.(*tableTab); ok && tt.table == table {
			w.active = i
			tt.w.sh.discardGuard(tt.pending(), func() { apply(tt) })
			return
		}
	}
	tt := newTableTab(w, table)
	w.tabs = append(w.tabs, tt)
	w.active = len(w.tabs) - 1
	apply(tt)
}

// openTableNewTab opens the table in one more tab, even when it is open already.
func (w *Workspace) openTableNewTab(name string) {
	tt := newTableTab(w, name)
	w.tabs = append(w.tabs, tt)
	w.active = len(w.tabs) - 1
	tt.load()
}

// previewTable is a single click on a table: it shows the table in the preview
// tab (italic), which the next click replaces; a double click, an edit or a
// change of view keeps it.
func (w *Workspace) previewTable(name string) {
	for i, t := range w.tabs {
		if tt, ok := t.(*tableTab); ok && tt.table == name {
			w.active = i
			return
		}
	}
	tt := newTableTab(w, name)
	tt.preview = true
	for i, t := range w.tabs {
		if old, ok := t.(*tableTab); ok && old.preview && old.pending() == 0 {
			w.tabs[i] = tt
			w.active = i
			tt.load()
			return
		}
	}
	w.tabs = append(w.tabs, tt)
	w.active = len(w.tabs) - 1
	tt.load()
}

// isPreviewTab reports whether tb is a preview tab.
func isPreviewTab(tb tab) bool {
	t, ok := tb.(*tableTab)
	return ok && t.preview
}

func (w *Workspace) closeTab(i int) {
	if i < 0 || i >= len(w.tabs) {
		return
	}
	tb := w.tabs[i]
	w.sh.discardGuard(tb.pending(), func() {
		for j, t := range w.tabs { // the index may have moved meanwhile
			if t == tb {
				w.tabs = append(w.tabs[:j], w.tabs[j+1:]...)
				break
			}
		}
		if w.active >= len(w.tabs) {
			w.active = max(0, len(w.tabs)-1)
		}
	})
}

func (w *Workspace) pendingTotal() int {
	n := 0
	for _, t := range w.tabs {
		n += t.pending()
	}
	return n
}

func (w *Workspace) closeOthers(keep int) {
	if keep < 0 || keep >= len(w.tabs) {
		return
	}
	w.tabs = []tab{w.tabs[keep]}
	w.active = 0
}

// tableMenu is the context menu of a table in the sidebar.
func (w *Workspace) tableMenu(m *ui.Menu, name string) {
	view := w.isViewName(name)
	if view && m.Item(locales.ObjEditView()).Chosen() {
		defer w.openDefinition("view", name)
	}
	if m.Item(locales.TableOpen()).Chosen() {
		defer w.openTable(name)
	}
	if m.Item(locales.TableOpenNewTab()).Chosen() {
		defer w.openTableNewTab(name)
	}
	if m.Item(locales.TableStructure()).Chosen() {
		defer w.openTableMode(name, 1)
	}
	if m.Item(locales.TableDDL()).Chosen() {
		defer w.openTableMode(name, 2)
	}
	if m.Item(locales.TableOpenQuery()).Chosen() {
		defer func() {
			sql, _ := w.sh.app.Queries.DefaultSelectTableSQL(w.cfg.ID, name, 100)
			w.newQuery(sql)
		}()
	}
	m.Separator()
	pin := locales.TablePin()
	if w.isPinned(name) {
		pin = locales.TableUnpin()
	}
	if m.Item(pin).Chosen() {
		defer w.togglePin(name)
	}
	if m.Item(locales.TableCopyName()).Chosen() {
		defer func() { w.sh.copyText(name) }()
	}
	if m.Item(locales.TableCopyScript()).Chosen() {
		defer func() { w.copyCreateScript(name) }()
	}
	if !view && m.Item(locales.TableClone()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.cloneTable(name)
	}
	m.Separator()
	if m.Item(locales.TableExport()).Chosen() {
		defer w.sh.exportTable(w, name)
	}
	if view {
		m.Separator()
		if m.Item(locales.ObjDropView()).Disabled(w.cfg.ReadOnly).Chosen() {
			defer w.dropObject("view", name)
		}
		return
	}
	if m.Item(locales.TableImport()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.sh.importInto(w, name)
	}
	m.Separator()
	if m.Item(locales.TableTruncate()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.truncateTable(name)
	}
	if m.Item(locales.TableDrop()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.dropTable(name)
	}
}

// copyCreateScript puts a table's CREATE statement on the clipboard.
func (w *Workspace) copyCreateScript(name string) {
	db := w.db
	var ddl string
	var err error
	w.sh.up.bg(func() { ddl, err = w.sh.app.Databases.GetDDL(w.cfg.ID, db, name) }, func() {
		if err != nil {
			w.sh.toast(err.Error())
			return
		}
		w.sh.copyText(ddl)
	})
}

func (w *Workspace) openTableMode(name string, mode int) {
	w.openTable(name)
	if tt, ok := w.tabs[w.active].(*tableTab); ok {
		tt.mode = mode
		tt.modeChanged()
	}
}

func (w *Workspace) truncateTable(name string) {
	w.sh.ask(locales.TableTruncateTitle(), locales.TableTruncateMsg(name), locales.TableTruncate(), func() {
		w.sh.safeGatePassword(w, []string{"TRUNCATE TABLE " + name}, func() {
			db := w.db
			var err error
			w.sh.up.bg(func() { err = w.sh.app.Databases.TruncateTable(w.cfg.ID, db, name) }, func() {
				if err != nil {
					w.sh.toast(err.Error())
					return
				}
				w.sh.toast(locales.TableDone())
				for _, tb := range w.tabs {
					if tt, ok := tb.(*tableTab); ok && tt.table == name {
						tt.load()
					}
				}
			})
		})
	})
}

func (w *Workspace) dropTable(name string) {
	w.sh.ask(locales.TableDropTitle(), locales.TableDropMsg(name), locales.TableDrop(), func() {
		w.sh.safeGatePassword(w, []string{"DROP TABLE " + name}, func() {
			db := w.db
			var err error
			w.sh.up.bg(func() { err = w.sh.app.Databases.DropTable(w.cfg.ID, db, name) }, func() {
				if err != nil {
					w.sh.toast(err.Error())
					return
				}
				for i := len(w.tabs) - 1; i >= 0; i-- {
					if tt, ok := w.tabs[i].(*tableTab); ok && tt.table == name {
						tt.revert()
						w.closeTab(i)
					}
				}
				w.sh.toast(locales.TableDone())
				w.loadTables()
			})
		})
	})
}

func (w *Workspace) togglePanel(p int) {
	if w.panel == p {
		w.panel = 0
	} else {
		w.panel = p
	}
}

// dbLabel is the database's name for display; SQLite's is its file's.
func (w *Workspace) dbLabel() string {
	if w.cfg.Type == "sqlite" {
		return filepath.Base(w.cfg.Database)
	}
	return w.db
}
