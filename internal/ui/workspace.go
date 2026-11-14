package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/internal/sqltext"
)

// tab is one page of a workspace: a table, a query, ...
type tab interface {
	title() string
	icon() *ui.SVG
	// objKind 是标签里东西的类型（table、view、query、function、procedure），图标和颜色都由它定。
	objKind() ObjectKind
	view(c *ui.Context, p tokens)
	// pending reports unsaved edits, for the close guard and the status bar.
	pending() int
}

// Workspace is the window content of one open connection. One connection can have
// several of them: every "open" opens a fresh one, and switching database happens
// inside a workspace (shell.go switchInPlace).
type Workspace struct {
	sh  *Shell
	cfg model.ConnectionConfig
	uid int // 序号，让同一个连接、同一个库开出的几个工作区在界面里有各自的键

	dbs           []model.DatabaseInfo
	db            string
	dbSel         string
	tables        []model.TableInfo
	routines      []model.RoutineInfo
	keywords      map[string]string          // 收藏查询的关键字到语句，供补全
	favs          map[string]app.HistoryItem // 收藏的语句（去掉首尾空白）到条目，供编辑器判断当前语句是否已收藏
	loading       bool
	loadErr       string
	filter        string
	tableSel      ui.ListState
	side          sideState
	loadStart     time.Time
	midKey        string // the tab a held middle button already closed
	sidebarHidden bool
	sideHover     bool  // 鼠标这一帧在侧栏上（⌘F 在侧栏上是搜索侧栏的列表）
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
	idx      indexView
	panelPct float32
	panel    int // 0 none, 1 inspector, 2 assistant
	panelW   float32

	// 异步加载：带序号保护，快速切库、反复刷新时旧响应不会盖住新结果。
	tablesLd   loader[[]model.TableInfo]
	routinesLd loader[[]model.RoutineInfo]
	schemaLd   loader[[]sqltext.Table]
	visCache   visibleCache // 侧栏过滤结果的缓存
}

func newWorkspace(s *Shell, cfg model.ConnectionConfig) *Workspace {
	s.spaceSeq++
	w := &Workspace{sh: s, cfg: cfg, uid: s.spaceSeq, db: cfg.Database, sidebarW: 260, panelW: 340, panelPct: 66}
	s.layout.applyTo(w)
	w.ai = newAssistant(w)
	return w
}

func (w *Workspace) loadDatabases() {
	w.loading = true
	w.loadStart = time.Now()
	var dbs []model.DatabaseInfo
	var err error
	var ver string
	w.sh.up.bg(func() {
		dbs, err = w.sh.app.Databases.GetDatabases(w.cfg.ID)
		ver, _ = w.sh.app.Databases.GetServerVersion(w.cfg.ID)
	}, func() {
		w.loading = false
		w.version = ver
		if err != nil {
			w.loadErr = errMessage(err)
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

// id 是工作区在界面里的唯一键：连接、当前库，再加序号——同一个连接可以开出好几个工作区，
// 甚至停在同一个库上，所以不能只靠连接和库。切库时它会变（异步回调靠它判断"是不是已经切走了"）。
func (w *Workspace) id() string { return fmt.Sprintf("%s:%s#%d", w.cfg.ID, w.db, w.uid) }

// dbKey 是当前库的存档键（连接:库）：置顶的表、上次的会话都按它存，
// 同一个连接同一个库的几个工作区共用一份。
func (w *Workspace) dbKey() string { return w.cfg.ID + ":" + w.db }

// ordinal 是 w 在同连接、同库的工作区里的第几个（从 1 起），用来在多个同名工作区之间区分。
func (w *Workspace) ordinal() int {
	n := 0
	for _, o := range w.sh.spaces {
		if o.cfg.ID == w.cfg.ID && o.db == w.db {
			n++
		}
		if o == w {
			return n
		}
	}
	return 1
}

// switchDatabase 在当前工作区里切到另一个库。
func (w *Workspace) switchDatabase(name string) { w.sh.openDatabase(w, name) }

func (w *Workspace) loadTables() {
	w.loading, w.loadErr = true, ""
	w.loadStart = time.Now()
	db := w.db
	w.tablesLd.load(w.sh.up, func() ([]model.TableInfo, error) { return w.sh.app.Databases.GetTables(w.cfg.ID, db) }, func(tables []model.TableInfo, err error) {
		if db != w.db {
			return // switched meanwhile
		}
		w.loading = false
		if err != nil {
			w.loadErr = errMessage(err)
			return
		}
		w.tables = tables
		for _, tb := range w.tabs { // 表增删改名之后，开着的概览跟着刷新
			if o, ok := tb.(*overviewTab); ok {
				o.load()
			}
		}
		w.loadPins()
		w.loadRoutines()
		w.loadKeywords()
		// 补全数据不急着要：真窗口里先让用户刚打开的内容画出来，晚一会儿再取
		delay := schemaLoadDelay
		if w.sh.win == nil {
			delay = 0
		}
		w.loadSchema(delay)
	})
}

// schemaLoadDelay 打开库之后多久才去取 SQL 补全用的列信息。
const schemaLoadDelay = 1500 * time.Millisecond

// loadSchema fetches columns of every table for SQL completion, after delay.
// 切库、关闭工作区时（schemaLd.cancel）等待中的这次加载会被取消，不会白取一遍。
func (w *Workspace) loadSchema(delay time.Duration) {
	db := w.db
	w.schemaLd.loadAfter(w.sh.up, delay, func(ctx context.Context) ([]sqltext.Table, error) {
		sc, err := w.sh.app.SchemaIndex.GetSchema(ctx, w.cfg.ID, db)
		if err != nil || sc == nil {
			return nil, err
		}
		var out []sqltext.Table
		for _, t := range sc.Tables {
			tb := sqltext.Table{Name: t.Name}
			for _, c := range t.Columns {
				tb.Columns = append(tb.Columns, sqltext.Column{Name: c.Name, Type: c.Type})
			}
			out = append(out, tb)
		}
		return out, nil
	}, func(out []sqltext.Table, err error) {
		if db == w.db && err == nil && out != nil {
			w.schema = out
		}
	})
}

// visibleCache 记着上一次侧栏过滤的结果。表清单只会整体替换、不会原地改动，
// 所以"筛选词 + 清单的长度与首元素地址"都没变，就说明结果还是有效的。
type visibleCache struct {
	q     string
	n     int
	first *model.TableInfo
	out   []model.TableInfo
	valid bool
}

// visibleTables applies the sidebar filter. 侧栏每帧都要问，几千张表时逐个转小写再匹配不能每帧重做，
// 所以按筛选词缓存。
func (w *Workspace) visibleTables() []model.TableInfo {
	q := strings.ToLower(strings.TrimSpace(w.filter))
	if q == "" {
		return w.tables
	}
	var first *model.TableInfo
	if len(w.tables) > 0 {
		first = &w.tables[0]
	}
	if c := &w.visCache; c.valid && c.q == q && c.n == len(w.tables) && c.first == first {
		return c.out
	}
	var out []model.TableInfo
	for _, t := range w.tables {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	w.visCache = visibleCache{q: q, n: len(w.tables), first: first, out: out, valid: true}
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

func (w *Workspace) togglePanel(p int) {
	if w.panel == p {
		w.panel = 0
	} else {
		w.panel = p
	}
}

// dbLabel is the database's name for display; SQLite's is its file's.
func (w *Workspace) dbLabel() string {
	if !app.CapsOf(w.cfg.Type).MultiDatabase {
		return filepath.Base(w.cfg.Database)
	}
	return w.db
}
