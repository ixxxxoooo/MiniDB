package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// 切库只在原工作区里就地切换，不会多出工作区；会话按库各存一份，切回来时恢复。
func TestDatabaseSwitchStaysInWorkspace(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	main := w.db
	w.openTable("users")

	e.sh.openDatabase(w, "other")
	if len(e.sh.spaces) != 1 || e.sh.spaces[0] != w || w.db != "other" {
		t.Fatalf("切库必须在原工作区里完成: n=%d db=%q", len(e.sh.spaces), w.db)
	}
	if len(w.tabs) != 0 {
		t.Fatalf("切库后旧库的标签页应当清掉: tabs=%d", len(w.tabs))
	}
	// 切回去：旧库的标签页恢复
	w.switchDatabase(main)
	if len(e.sh.spaces) != 1 || w.db != main {
		t.Fatalf("切回原库: n=%d db=%q", len(e.sh.spaces), w.db)
	}
	if len(w.tabs) != 1 || w.tabs[0].title() != "users" {
		t.Fatalf("切回来要恢复这个库的标签页: tabs=%d", len(w.tabs))
	}
	e.tt.Frame()
}

// 断开连接只关当前这个工作区，别的连接不受影响。
func TestDisconnectClosesOnlyCurrentWorkspace(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name = "c2", "second"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.openConnection(cfg2)
	if len(e.sh.spaces) != 2 || e.sh.active != 1 {
		t.Fatalf("新建连接要新开一个工作区: n=%d active=%d", len(e.sh.spaces), e.sh.active)
	}
	e.sh.closeSpaceGuarded(e.sh.spaces[e.sh.active])
	if len(e.sh.spaces) != 1 || e.sh.spaces[0] != w {
		t.Fatalf("只应断开当前工作区: n=%d", len(e.sh.spaces))
	}
}

func TestRailClickHoverAndClose(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name = "c2", "second"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.openConnection(cfg2)
	e.tt.Frame()
	if e.sh.active != 1 {
		t.Fatalf("active = %d", e.sh.active)
	}
	// click the first rail item: switches workspace
	e.tt.ClickAt(30, 66)
	e.tt.Frame()
	if e.sh.active != 0 {
		t.Fatalf("clicking the first rail item must activate it, active=%d", e.sh.active)
	}
	// hover the second one: shows a close button; clicking it closes the workspace
	e.tt.Move(30, 150)
	e.tt.Frame()
	e.shot("51-rail-hover")
	e.tt.ClickAt(44, 128)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 {
		t.Fatalf("the close button must close that workspace, spaces=%d", len(e.sh.spaces))
	}
	if e.sh.spaces[0].cfg.ID == "c2" {
		t.Fatal("closed the wrong workspace")
	}
}

func TestSidebarSingleClickOpensTable(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if err := e.tt.Click("orders"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.tabs) != 1 || w.tabs[0].title() != "orders" {
		t.Fatalf("one click on a table must open it, tabs=%d", len(w.tabs))
	}
}

func TestMiddleClickClosesTab(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	w.openTable("orders")
	e.tt.Frame()
	if len(w.tabs) != 2 {
		t.Fatalf("tabs = %d", len(w.tabs))
	}
	x, y := float32(290), float32(50) // the first tab, "users"（只有一个工作区时没有左栏）
	e.tt.PressButton(2, x, y)
	e.tt.Frame()
	e.tt.ReleaseButton(2, x, y)
	e.tt.Frame()
	if len(w.tabs) != 1 || w.tabs[0].title() != "orders" {
		t.Fatalf("middle click must close the tab, tabs=%d", len(w.tabs))
	}
}

func TestFilterBarEscapeCloses(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	if !tab.showFilter {
		t.Fatal("⌘F must open the filter bar")
	}
	e.tt.Type("abc")
	e.tt.Frame()
	e.shot("52-filter")
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if tab.showFilter {
		t.Fatal("Escape must close the filter bar, even with the input focused")
	}
	if tab.filters[0].val != "" {
		t.Fatalf("closing clears the filter text, got %q", tab.filters[0].val)
	}
}

func TestGlobalShortcuts(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	e.tt.Key(mui.Cmd, mui.KeyT)
	e.tt.Frame()
	if len(w.tabs) != 1 {
		t.Fatalf("⌘T must open one query tab, tabs=%d", len(w.tabs))
	}
	w.openTable("users")
	w.openTable("orders")
	e.tt.Frame()
	e.tt.Key(mui.Cmd, mui.KeyBracketLeft)
	e.tt.Frame()
	if w.active != 1 {
		t.Fatalf("⌘[ goes to the previous tab, active=%d", w.active)
	}
	e.tt.Key(mui.Cmd, mui.KeyBracketRight)
	e.tt.Frame()
	if w.active != 2 {
		t.Fatalf("⌘] goes to the next tab, active=%d", w.active)
	}
	tab := w.tabs[2].(*tableTab)
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyBracketRight)
	e.tt.Frame()
	if tab.mode != 1 {
		t.Fatalf("⌘⌥] switches the sub view, mode=%d", tab.mode)
	}
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyBracketLeft)
	e.tt.Frame()
	if tab.mode != 0 {
		t.Fatalf("⌘⌥[ switches back, mode=%d", tab.mode)
	}
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	if !tab.showFilter {
		t.Fatal("⌘F opens the filter bar exactly once")
	}
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	e.tt.Key(0, mui.KeyInsert)
	e.tt.Frame()
	if tab.pendingCount() != 1 {
		t.Fatalf("Insert adds a row, pending=%d", tab.pendingCount())
	}
	e.tt.Key(mui.Cmd, mui.KeyW)
	e.tt.Frame()
	if len(w.tabs) != 2 && len(e.sh.confirm.title) == 0 {
		t.Fatalf("⌘W closes the tab (or asks about changes), tabs=%d", len(w.tabs))
	}
}

func TestTabCloseButtonOnHoverOnly(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	w.openTable("orders")
	w.newQuery("select 1")
	e.tt.Move(1000, 400) // away from the tabs
	e.tt.Frame()
	e.shot("70-tabs")
	// hovering the first tab shows its close button; clicking it closes the tab
	e.tt.Move(310, 50)
	e.tt.Frame()
	e.shot("71-tabs-hover")
	e.tt.ClickAt(342, 50) // its close slot, at the right end of the tab
	e.tt.Frame()
	if len(w.tabs) != 2 {
		t.Fatalf("the close button of a hovered tab must close it, tabs=%d", len(w.tabs))
	}
}

func TestConnectionListKeyboardAndPill(t *testing.T) {
	e := newEnv(t)
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name = "c2", "second"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyK)
	e.tt.Frame()
	if !e.sh.dlg.open {
		t.Fatal("⌘⇧K must open the connection list")
	}
	first := e.sh.selected
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	if e.sh.selected == first {
		t.Fatalf("↓ must move the selection off %q", first)
	}
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 || e.sh.dlg.open {
		t.Fatalf("Enter must open the selected connection: spaces=%d dialogOpen=%v", len(e.sh.spaces), e.sh.dlg.open)
	}
	// the title-bar pill opens the list again
	e.tt.ClickAt(640, 18)
	e.tt.Frame()
	if !e.sh.dlg.open {
		t.Fatal("clicking the connection pill must open the list")
	}
}

func TestCommitShortcutAfterCellEdit(t *testing.T) {
	// finished edit (Enter), then ⌘S
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.beginEdit(2, "name")
	e.tt.Frame()
	e.tt.Type("zzz")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if tab.pendingCount() != 1 {
		t.Fatalf("an edit finished with Enter is pending, got %d", tab.pendingCount())
	}
	e.tt.Key(mui.Cmd, mui.KeyS)
	e.tt.Frame()
	if tab.pendingCount() != 0 {
		t.Fatal("⌘S after a finished edit must commit it at once")
	}

	// still editing the cell: ⌘S commits that edit too, then previews
	e2 := newEnv(t)
	tab2 := openUsers(e2)
	e2.tt.Frame()
	tab2.beginEdit(3, "name")
	e2.tt.Frame()
	e2.tt.Type("yyy")
	e2.tt.Key(mui.Cmd, mui.KeyS)
	e2.tt.Frame()
	if tab2.pendingCount() != 0 || tab2.es.editing != nil {
		t.Fatalf("⌘S while a cell is still being edited must take the edit and commit it (pending=%d, editing=%v)", tab2.pendingCount(), tab2.es.editing != nil)
	}
	if got := queryCount(t, e2, `SELECT name FROM users WHERE id=4`); got != "用户004yyy" {
		t.Fatalf("the typed text must be in the database, got %q", got)
	}
}

func TestClickingAwayKeepsTypedCell(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.beginEdit(1, "name")
	e.tt.Frame()
	e.tt.Type("keep")
	e.tt.Frame()
	e.tt.ClickAt(700, 400) // another cell
	e.tt.Frame()
	e.tt.Frame()
	if tab.es.editing != nil || tab.pendingCount() != 1 {
		t.Fatalf("clicking another cell must end the edit and keep the text: editing=%v pending=%d", tab.es.editing != nil, tab.pendingCount())
	}
}

func TestBackspaceReachesEditors(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("hello")
	e.tt.Frame()
	q.ed.Select(5, 5)
	e.tt.Frame()
	e.tt.Key(0, mui.KeyBackspace)
	e.tt.Frame()
	if got := q.ed.Buffer.String(); got != "hell" {
		t.Fatalf("Backspace in the SQL editor: %q", got)
	}

	// the table doc's editor
	e2 := newEnv(t)
	tab := openUsers(e2)
	tab.mode = 3
	tab.modeChanged()
	e2.tt.Frame()
	e2.tt.ClickAt(450, 200) // into the editor
	e2.tt.Frame()
	e2.tt.Type("# doc")
	e2.tt.Frame()
	e2.tt.Key(0, mui.KeyBackspace)
	e2.tt.Frame()
	if got := tab.doc.ed.Buffer.String(); got != "# do" {
		t.Fatalf("Backspace in the doc editor: %q", got)
	}
	e2.tt.Key(0, mui.KeyLeft)
	e2.tt.Key(0, mui.KeyDelete)
	e2.tt.Frame()
	if got := tab.doc.ed.Buffer.String(); got != "# d" {
		t.Fatalf("Delete in the doc editor: %q", got)
	}
}

func TestBackspaceDeletesChosenRowsInTheGrid(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.tt.ClickAt(450, 150) // a row of the grid: it takes the focus and chooses the row
	e.tt.Frame()
	if tab.g.chosen.Len() == 0 {
		t.Fatal("the click should choose a row")
	}
	e.tt.Key(0, mui.KeyBackspace)
	e.tt.Frame()
	if len(tab.edits.deleted) != 1 {
		t.Fatalf("Backspace with the grid focused deletes the chosen row, deleted=%d", len(tab.edits.deleted))
	}
}

func TestParseConnURL(t *testing.T) {
	typ, host, port, user, pass, db, ok := parseConnURL("mysql://root:s3cret@10.0.0.5:3307/shop")
	if !ok || typ != "mysql" || host != "10.0.0.5" || port != 3307 || user != "root" || pass != "s3cret" || db != "shop" {
		t.Fatalf("mysql url: %v %v %v %v %v %v %v", typ, host, port, user, pass, db, ok)
	}
	typ, host, port, _, _, db, ok = parseConnURL("postgresql://me@db.example.com/app")
	if !ok || typ != "postgres" || host != "db.example.com" || port != 0 || db != "app" {
		t.Fatalf("postgres url: %v %v %v %v %v", typ, host, port, db, ok)
	}
	if _, _, _, _, _, _, ok := parseConnURL("not a url"); ok {
		t.Fatal("garbage must be rejected")
	}
	if _, _, _, _, _, _, ok := parseConnURL("ftp://host/x"); ok {
		t.Fatal("an unknown scheme must be rejected")
	}
}

// Switching to Structure/DDL/Doc must show content at once: the views were
// loaded behind the rows, and nothing shows a loading note or spinner.
func TestSubViewsArePrefetched(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	if !tab.st.loaded || !tab.ddl.loaded || !tab.doc.loaded {
		t.Fatalf("sub-views should be prefetched: structure=%v ddl=%v doc=%v", tab.st.loaded, tab.ddl.loaded, tab.doc.loaded)
	}
	tab.mode = 1
	e.tt.Frame()
	e.hasNot("加载中")
	e.has("索引")
}

func TestCommitNeedsNoDialogExceptProduction(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "name", "direct")
	tab.commitNow()
	if e.sh.confirm.open || tab.pendingCount() != 0 {
		t.Fatalf("a local commit is immediate: confirm=%v pending=%d", e.sh.confirm.open, tab.pendingCount())
	}

	// a production connection keeps one confirmation, as the original does
	tab.w.cfg.Tag = "production"
	tab.setCell(1, "name", "guarded")
	tab.commitNow()
	if !e.sh.confirm.open || tab.pendingCount() == 0 {
		t.Fatalf("production asks before writing: confirm=%v pending=%d", e.sh.confirm.open, tab.pendingCount())
	}
}

func TestPaletteAndSwitcherArrowKeys(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.sh.openPalette()
	e.tt.Frame()
	e.tt.Frame() // the field takes the focus
	if len(e.sh.paletteItems()) < 3 {
		t.Fatal("need a few palette items")
	}
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	if e.sh.pal.sel != 2 {
		t.Fatalf("↓↓ in the palette must move the selection to 2, got %d", e.sh.pal.sel)
	}
	e.tt.Key(0, mui.KeyUp)
	e.tt.Frame()
	if e.sh.pal.sel != 1 {
		t.Fatalf("↑ must move back to 1, got %d", e.sh.pal.sel)
	}
	// the highlighted row is the one Enter runs
	e.sh.pal.open = false

	e.sh.openSwitcher()
	e.tt.Frame()
	e.tt.Frame()
	d2, d3 := e.sh.sw.w.dbs[0], e.sh.sw.w.dbs[0]
	d2.Name, d3.Name = "second", "third"
	e.sh.sw.w.dbs = append(e.sh.sw.w.dbs, d2, d3)
	e.tt.Frame()
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	if e.sh.sw.sel != 1 {
		t.Fatalf("↓ in the database switcher must move the selection, got %d", e.sh.sw.sel)
	}
}

func TestConnectionListArrowKeys(t *testing.T) {
	e := newEnv(t)
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name = "c2", "second"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.newConnectionDialog()
	e.tt.Frame()
	e.tt.Frame()
	first := e.sh.selected
	e.tt.Key(0, mui.KeyDown)
	e.tt.Frame()
	if e.sh.selected == first || e.sh.selected == "" {
		t.Fatalf("↓ with the search field focused must move the selection off %q, got %q", first, e.sh.selected)
	}

}

func TestInitialDatabase(t *testing.T) {
	my := model.ConnectionConfig{Type: "mysql", Database: "shop"}
	// the connection's default database wins over where the last session ended
	if db, restore := initialDatabase(my, wsState{Database: "other"}, true); db != "shop" || restore {
		t.Fatalf("configured default must win: %q restore=%v", db, restore)
	}
	// the same database as the last session: its tabs come back
	if db, restore := initialDatabase(my, wsState{Database: "shop"}, true); db != "shop" || !restore {
		t.Fatalf("same database restores the tabs: %q restore=%v", db, restore)
	}
	// no default configured: the last session's database
	none := model.ConnectionConfig{Type: "mysql"}
	if db, restore := initialDatabase(none, wsState{Database: "other"}, true); db != "other" || !restore {
		t.Fatalf("no default: last session's: %q restore=%v", db, restore)
	}
	// nothing at all: left empty for firstUserDatabase
	if db, _ := initialDatabase(none, wsState{}, false); db != "" {
		t.Fatalf("db = %q", db)
	}
	dbs := []model.DatabaseInfo{{Name: "information_schema"}, {Name: "mysql"}, {Name: "app"}, {Name: "sys"}}
	if got := firstUserDatabase(dbs); got != "app" {
		t.Fatalf("first user database = %q, want app (not a system schema)", got)
	}
}

// 左边的工作区栏：只有一个工作区（或没有连接）时不显示，多于一个才出现；出现时侧栏整体右移 60。
func TestRailOnlyWithMoreThanOneWorkspace(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 {
		t.Fatalf("spaces = %d", len(e.sh.spaces))
	}
	one, ok := e.tt.Find("项目")
	if !ok {
		t.Fatal("侧栏里应有「项目」分段")
	}
	e.shot("72-no-rail")
	e.sh.openConnection(e.sh.conns[0]) // 再开一个工作区：栏出现
	e.tt.Frame()
	if len(e.sh.spaces) != 2 {
		t.Fatalf("spaces = %d", len(e.sh.spaces))
	}
	two, ok := e.tt.Find("项目")
	if !ok {
		t.Fatal("侧栏里应有「项目」分段")
	}
	if d := two.X - one.X; d < 58 || d > 62 {
		t.Fatalf("两个工作区时侧栏应右移 60（栏的宽度），实际 %v → %v", one.X, two.X)
	}
	e.shot("73-rail-two")
}

func TestParseConnURLMariaDB(t *testing.T) {
	typ, host, port, user, _, db, ok := parseConnURL("mariadb://root:pw@127.0.0.1:3307/app")
	if !ok || typ != model.MariaDB || host != "127.0.0.1" || port != 3307 || user != "root" || db != "app" {
		t.Fatalf("mariadb:// 应解析为 MariaDB 类型: %v %v %v %v %v %v", typ, host, port, user, db, ok)
	}
	if typ, _, _, _, _, _, ok := parseConnURL("mysql://root@h/db"); !ok || typ != model.MySQL {
		t.Fatalf("mysql:// 仍是 MySQL: %v", typ)
	}
}

func TestParseConnURLClickHouse(t *testing.T) {
	typ, host, port, user, pass, db, ok := parseConnURL("clickhouse://default:pw@ch.local:9440/events")
	if !ok || typ != model.ClickHouse || host != "ch.local" || port != 9440 || user != "default" || pass != "pw" || db != "events" {
		t.Fatalf("%v %v %v %v %v %v %v", typ, host, port, user, pass, db, ok)
	}
}

func TestParseConnURLSQLServer(t *testing.T) {
	typ, host, port, user, pass, db, ok := parseConnURL("sqlserver://sa:Pw%401@10.0.0.5:1433?database=Sales&encrypt=disable")
	if !ok || typ != model.SQLServer || host != "10.0.0.5" || port != 1433 || user != "sa" || pass != "Pw@1" || db != "Sales" {
		t.Fatalf("%v %v %v %v %v %v %v", typ, host, port, user, pass, db, ok)
	}
	if typ, _, port, _, _, _, ok := parseConnURL("mssql://sa@h"); !ok || typ != model.SQLServer || port != 0 {
		t.Fatalf("mssql:// 也是 SQL Server: %v %v", typ, port)
	}
}
