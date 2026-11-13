package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

var filterOps = []string{"=", "!=", ">", "<", ">=", "<=", "LIKE", "NOT LIKE", "IN", "IS NULL", "IS NOT NULL"}
var pageSizes = []string{"100", "200", "500", "1000"}

// tableTab browses one table: a paged grid with server-side sort and filter.
type tableTab struct {
	w       *Workspace
	table   string
	preview bool // a single-clicked table: the next click replaces it

	loading   bool
	started   bool // load has been called at least once
	loadStart time.Time
	err       string
	seq       int // discards responses of superseded loads

	g           *resultGrid
	edits       tableEdits
	es          tableEditState
	st          structState
	ddl         ddlState
	doc         docState
	mode        int // 0 data, 1 structure, 2 DDL, 3 docs
	showPreview bool
	showFilter  bool
	focusFilter bool
	previewW    float32 // width of the row pane
	total       int64
	dur         int64

	page     int // 1-based
	pageSize string

	sortOrder ui.SortOrder
	sorted    bool
	sorts     []database.Sort // every sort column, in order: ⇧ click adds one

	filterOr      bool // the conditions join with OR instead of AND
	filterFocused bool // a filter field has the keyboard (⌘I, ⌘⇧I and ⌘B are the bar's then)

	filters    []filterRow // the rows of the filter bar
	applied    []database.Filter
	appliedRaw string
	colsOpen   bool // the "columns shown" dialog
	colsQ      string
	quick      *quickCell // the Set Value menu
	sqlOpen    bool       // the SQL preview
	pv         struct {   // the row pane's field buffers
		q        string
		buf, sig map[string]string
		menu     map[string]*bool // 各字段的 ⋯ 菜单是否展开（展开时按钮不能因鼠标移开而消失）
	}
	qlOpen bool // Quick Look
	ql     quickLook
}

// filterRow is one condition of the filter bar: ticked ones apply together.
type filterRow struct {
	on           bool
	col, op, val string
}

// keep promotes a preview tab to a normal one, once the user does something in it.
func (t *tableTab) keep() { t.preview = false }

func newFilterRow() filterRow {
	return filterRow{on: true, col: locales.TableFilterAny(), op: "="}
}

func newTableTab(w *Workspace, table string) *tableTab {
	g := newResultGrid(true)
	g.dbType, g.table = w.cfg.Type, table
	t := &tableTab{w: w, table: table, g: g, page: 1, pageSize: w.sh.prefs.pageSize(), filters: []filterRow{newFilterRow()}}
	g.cell = t.drawCell
	g.rowTint0 = t.rowTint
	g.onKeys = t.gridKeys
	g.extraMenu = t.cellMenuExtras
	t.loadStructure()
	return t
}

func (t *tableTab) title() string { return t.table }
func (t *tableTab) icon() *ui.SVG { return icTable }
func (t *tableTab) pending() int  { return t.pendingCount() + t.st.changeCount() }

func (t *tableTab) size() int {
	var n int
	fmt.Sscan(t.pageSize, &n)
	if n <= 0 {
		return 200
	}
	return n
}

// load fetches the current page; pending edits are dropped, so callers that
// can lose them go through reload.
func (t *tableTab) load() {
	t.started = true
	t.loadStart = time.Now()
	t.seq++
	seq := t.seq
	t.loading, t.err = true, ""
	w := t.w
	sorts := append([]database.Sort(nil), t.sortList()...)
	filters, raw, page, size := append([]database.Filter(nil), t.applied...), t.appliedRaw, t.page, t.size()
	db := w.db
	logged := t.tableSelectSQL() // 控制台日志里记下这一页对应的 SELECT
	var res *database.QueryResult
	var err error
	w.sh.up.bg(func() {
		if strings.TrimSpace(raw) == "" {
			// rows first: COUNT(*) can take long on big tables, so it follows
			res, err = w.sh.app.Queries.QueryTableRows(w.cfg.ID, db, t.table, page, size, filters, sorts)
			return
		}
		res, err = w.sh.app.Queries.QueryTableDataWithRawInput(w.cfg.ID, db, t.table, page, size, filters, sorts, raw)
	}, func() {
		if seq != t.seq {
			return
		}
		t.loading = false
		switch {
		case err != nil:
			t.err = err.Error()
			w.sh.logSQL(db, logged, 0, -1, t.err)
		case res != nil && res.Error != "":
			t.err = res.Error
			w.sh.logSQL(db, logged, time.Duration(res.Duration)*time.Millisecond, -1, t.err)
		case res != nil:
			w.sh.logSQL(db, logged, time.Duration(res.Duration)*time.Millisecond, len(res.Rows), "")
			t.total, t.dur = res.Total, res.Duration
			t.g.offset = (page - 1) * size
			t.setRows(res.Columns, res.Rows)
			if res.Total < 0 {
				t.countRows(seq, filters)
			}
			t.prefetch()
		}
	})
}

// prefetch loads what the other sub-views show (structure, DDL, doc) once the
// rows are on screen, so switching to them is instant, with nothing to wait for.
func (t *tableTab) prefetch() {
	if !t.st.loaded && !t.st.loading && t.st.err == "" {
		t.st.load(t)
	}
	if !t.ddl.loaded && !t.ddl.loading {
		t.ddl.load(t)
	}
	if !t.doc.loaded && !t.doc.loading {
		t.doc.load(t)
	}
}

// countRows fills in the total after the rows are on screen.
func (t *tableTab) countRows(seq int, filters []database.Filter) {
	w, db := t.w, t.w.db
	var n int64
	var err error
	w.sh.up.bg(func() { n, err = w.sh.app.Queries.CountTableData(w.cfg.ID, db, t.table, filters) }, func() {
		if seq != t.seq {
			return
		}
		if err != nil {
			n = int64(len(t.g.rows)) // no total: show what the page holds
			if t.page > 1 {
				n += int64(t.g.offset)
			}
		}
		t.total = n
	})
}

// reload loads again after asking, when edits would be lost.
func (t *tableTab) reload(change func()) {
	t.w.sh.discardGuard(t.pending(), func() {
		if change != nil {
			change()
		}
		t.load()
	})
}

func (t *tableTab) applyFilter() { t.reload(t.setFilter) }

func (t *tableTab) setFilter() {
	t.applied, t.appliedRaw = nil, ""
	for _, f := range t.filters {
		if !f.on {
			continue
		}
		if f.col == locales.DatagridRawSQL() {
			if txt := strings.TrimSpace(f.val); txt != "" && t.appliedRaw == "" {
				t.appliedRaw = txt
			}
			continue
		}
		op := f.op
		if strings.TrimSpace(f.val) == "" && op != "IS NULL" && op != "IS NOT NULL" {
			continue
		}
		col := f.col
		if col == locales.TableFilterAny() {
			col = "__any"
			if op != "=" && op != "LIKE" {
				op = "LIKE"
			}
		}
		t.applied = append(t.applied, database.Filter{Column: col, Operator: op, Value: f.val, Or: t.filterOr && len(t.applied) > 0})
	}
	t.page = 1
}

func (t *tableTab) clearFilter() {
	t.reload(func() {
		t.filters = []filterRow{newFilterRow()}
		t.applied, t.appliedRaw = nil, ""
		t.page = 1
	})
}

// hasFilter reports whether anything is applied or typed in the filter bar.
func (t *tableTab) hasFilter() bool {
	if len(t.applied) > 0 || t.appliedRaw != "" {
		return true
	}
	for _, f := range t.filters {
		if strings.TrimSpace(f.val) != "" {
			return true
		}
	}
	return false
}

// sortList is the sort the table is loaded with.
func (t *tableTab) sortList() []database.Sort {
	if len(t.sorts) > 0 {
		return t.sorts
	}
	if t.sorted && t.sortOrder.Column != "" {
		return []database.Sort{sortOf(t.sortOrder)}
	}
	return nil
}

func sortOf(o ui.SortOrder) database.Sort {
	dir := "ASC"
	if o.Descending {
		dir = "DESC"
	}
	return database.Sort{Column: o.Column, Direction: dir}
}

// addSort puts a column's sort in: alone, or after the others when more is true
// (⇧ click on a header). Clicking a column already sorted flips its direction.
func (t *tableTab) addSort(o ui.SortOrder, more bool) {
	s := sortOf(o)
	if !more {
		t.sorts = []database.Sort{s}
		return
	}
	for i := range t.sorts {
		if t.sorts[i].Column == s.Column {
			t.sorts[i] = s
			return
		}
	}
	if len(t.sorts) == 0 && t.sorted && t.sortOrder.Column != "" && t.sortOrder.Column != s.Column {
		t.sorts = []database.Sort{sortOf(t.sortOrder)} // what the header showed so far comes first
	}
	t.sorts = append(t.sorts, s)
}

// sortMarks numbers the sort columns in their headers when there are several.
func (t *tableTab) sortMarks() map[string]string {
	if len(t.sorts) < 2 {
		return nil
	}
	marks := map[string]string{}
	for i, s := range t.sorts {
		arrow := "↑"
		if s.Direction == "DESC" {
			arrow = "↓"
		}
		marks[s.Column] = fmt.Sprintf(" %s%d", arrow, i+1)
	}
	return marks
}

func (t *tableTab) gridView(c *ui.Context) {
	more := c.Modifiers()&ui.Shift != 0
	if t.g.view(c, t.table, t.w.sh.toast) {
		sorted := t.g.sort
		t.g.sort = t.sortOrder // keep the header as it was if the user declines
		t.reload(func() {
			t.sorted, t.sortOrder, t.g.sort = true, sorted, sorted
			t.addSort(sorted, more)
			t.g.marks = t.sortMarks()
			t.g.rebuild()
			t.page = 1
		})
	}
}

// rowTint is the colour of a row's state: deleted red, new green.
func (t *tableTab) rowTint(c *ui.Context, row int) ui.Color {
	p := baseTokens(c)
	switch {
	case t.edits.deleted[row]:
		return p.rowDelete
	case row >= len(t.es.orig):
		return p.rowNew
	}
	return ui.Color{}
}
