package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func TestPaletteFindsTablesAndRuns(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openPalette()
	e.has("新建连接…") // the commands, each with its icon
	e.sh.pal.query = ""
	e.shot("13-palette-empty")
	e.sh.pal.query = "ord"
	items := e.sh.paletteItems()
	if len(items) == 0 || items[0].label != "orders" {
		t.Fatalf("items = %+v", items)
	}
	e.tt.Frame()
	e.shot("13-palette")
	e.sh.pal.sel = 0
	items[0].run()
	if len(w.tabs) != 1 || w.tabs[0].title() != "orders" {
		t.Fatalf("palette should open the table, tabs=%d", len(w.tabs))
	}
}

func TestPaletteKeyboard(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openPalette()
	e.tt.Frame()
	e.tt.Type("users")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if e.sh.pal.open || len(w.tabs) != 1 || w.tabs[0].title() != "users" {
		t.Fatalf("typing a name and Enter should open it: open=%v tabs=%d", e.sh.pal.open, len(w.tabs))
	}
}

func TestCommandsActOnActiveTab(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.cmdNewQuery()
	if len(w.tabs) != 1 {
		t.Fatal("cmdNewQuery")
	}
	q := e.sh.activeQuery()
	q.ed.Buffer.Set("select 1 as x")
	e.sh.cmdFormat()
	if got := q.ed.Buffer.String(); got != "SELECT 1 AS x" {
		t.Fatalf("format = %q", got)
	}
	e.sh.cmdRun(true)
	if len(q.results) != 1 {
		t.Fatal("cmdRun")
	}
	e.sh.cmdToggleAI()
	if w.panel != 2 {
		t.Fatal("cmdToggleAI")
	}
	e.sh.cmdCycleTab(1)
	e.sh.cmdCloseTab()
	if len(w.tabs) != 0 {
		t.Fatalf("cmdCloseTab, tabs=%d", len(w.tabs))
	}
}

func TestCloseTabAsksBeforeDiscarding(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "name", "x")
	w := e.sh.spaces[0]
	e.sh.cmdCloseTab()
	e.has("丢弃未提交的修改")
	if len(w.tabs) != 1 {
		t.Fatal("tab must stay until the user agrees")
	}
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if len(w.tabs) != 0 {
		t.Fatalf("tabs after discarding = %d", len(w.tabs))
	}
}

func TestProcessListUnsupportedOnSQLite(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openProcesses(w)
	e.has("不支持查看进程")
}

func TestConnectionDialogValidationAndSave(t *testing.T) {
	e := newEnv(t)
	e.sh.newConnection()
	e.tt.Frame()
	e.shot("14-conn-dialog")
	// empty name is rejected with a message
	e.sh.saveConnection(false)
	if e.sh.dlg.msg == "" || !e.sh.dlg.open {
		t.Fatalf("missing name must be reported: msg=%q open=%v", e.sh.dlg.msg, e.sh.dlg.open)
	}
	d := &e.sh.dlg
	d.cfg.Name, d.typeSel = "mydb", "SQLite"
	e.sh.saveConnection(false)
	if d.msg == "" {
		t.Fatal("sqlite without a file must be rejected")
	}
	d.cfg.Database = e.dbPath
	e.sh.saveConnection(false)
	if !d.open || d.msg != "" {
		t.Fatalf("saving keeps the manager open: open=%v msg=%q", d.open, d.msg)
	}
	found := false
	for _, c := range e.sh.conns {
		if c.Name == "mydb" && c.Type == "sqlite" {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved connection missing: %+v", e.sh.conns)
	}
	// MySQL form: port must be numeric and in range
	e.sh.newConnection()
	d = &e.sh.dlg
	d.cfg.Name, d.port = "m", "99999"
	if _, problem := d.build(); problem == "" {
		t.Fatal("port 99999 must be rejected")
	}
	d.port = "3307"
	cfg, problem := d.build()
	if problem != "" || cfg.Port != 3307 || cfg.Type != "mysql" {
		t.Fatalf("cfg=%+v problem=%q", cfg, problem)
	}
}

func TestSecretsNeverLeakToUI(t *testing.T) {
	e := newEnv(t)
	cfg := e.cfg
	cfg.ID, cfg.Name, cfg.Type, cfg.Host, cfg.Port, cfg.User, cfg.Password = "c2", "pw", "mysql", "h", 3306, "u", "s3cret"
	if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	for _, c := range e.sh.conns {
		if c.Password != "" {
			t.Fatalf("connection %q carries a password into the UI", c.Name)
		}
	}
	e.sh.editConnection(e.sh.conns[len(e.sh.conns)-1])
	if e.sh.dlg.password != "" {
		t.Fatal("edit dialog must not prefill the stored password")
	}
}

func TestSessionRestoresTabsAndDrafts(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	w.newQuery("SELECT 1 -- draft")
	w.tabs[0].(*tableTab).mode = 1
	w.active = 1
	e.sh.closeSpace(0) // saves the session as it closes
	if len(e.sh.spaces) != 0 {
		t.Fatal("space should be closed")
	}

	e.sh.openConnection(e.sh.conns[0])
	w2 := e.sh.spaces[0]
	if len(w2.tabs) != 2 {
		t.Fatalf("restored tabs = %d", len(w2.tabs))
	}
	tt, ok := w2.tabs[0].(*tableTab)
	if !ok || tt.table != "users" || tt.mode != 1 {
		t.Fatalf("table tab = %+v", w2.tabs[0])
	}
	q, ok := w2.tabs[1].(*queryTab)
	if !ok || q.ed.Buffer.String() != "SELECT 1 -- draft" {
		t.Fatalf("query draft lost: %+v", w2.tabs[1])
	}
	if w2.active != 1 {
		t.Fatalf("active = %d", w2.active)
	}
	// a restored table tab loads its data only when shown
	if tt.started {
		t.Fatal("background tab must not load before it is shown")
	}
	w2.active, tt.mode = 0, 0 // back to the data view
	e.tt.Frame()
	if !tt.started {
		t.Fatal("tab should load once shown")
	}
}

func TestQuitGuardAsksAboutPendingEdits(t *testing.T) {
	e := newEnv(t)
	quit := false
	e.sh.requestQuit = func() { quit = true }
	if e.sh.quitGuard() {
		t.Fatal("nothing pending: quitting must not be blocked")
	}
	tab := openUsers(e)
	tab.setCell(0, "name", "x")
	if !e.sh.quitGuard() {
		t.Fatal("pending edits must block the quit")
	}
	e.has("退出 MiniDB？")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if !quit || !e.sh.quitting {
		t.Fatal("confirming must quit")
	}
}
