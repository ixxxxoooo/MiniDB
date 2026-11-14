package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

var pageSizes = []string{"100", "200", "500", "1000"}

// tableTab browses one table: a paged grid with server-side sort and filter.
// 状态按职责分给几个内嵌的小结构（字段名保持不变，t.page、t.filters 照旧可用）：
// pageState 取数与分页，sortState 排序，filterState 过滤条，tablePanes 各个浮层与侧栏。
type tableTab struct {
	w       *Workspace
	table   string
	preview bool // a single-clicked table: the next click replaces it
	isNew   bool // 新建表：还没有真正建出来，只有结构页（tablenew.go）

	pageState
	sortState
	filterState
	tablePanes

	g     *resultGrid
	edits tableEdits
	es    tableEditState
	st    structState
	ddl   ddlState
	doc   docState
	mode  int // 0 data, 1 structure, 2 DDL, 3 docs
}

// keep promotes a preview tab to a normal one, once the user does something in it.
func (t *tableTab) keep() { t.preview = false }

func newTableTab(w *Workspace, table string) *tableTab {
	g := newResultGrid(true)
	g.dbType, g.table = w.cfg.Type, table
	t := &tableTab{w: w, table: table, g: g}
	t.page, t.pageSize = 1, w.sh.prefs.pageSize()
	t.filters = []filterRow{newFilterRow()}
	g.cell = t.drawCell
	g.rowTint0 = t.rowTint
	g.onKeys = t.gridKeys
	g.extraMenu = t.cellMenuExtras
	g.onManage = func() { t.colsOpen = true }
	t.bindLayout()
	if table != "" {
		t.loadStructure()
	}
	return t
}

func (t *tableTab) title() string {
	if t.isNew {
		if n := strings.TrimSpace(t.st.name); n != "" {
			return n
		}
		return locales.TableNewTitle()
	}
	return t.table
}
func (t *tableTab) icon() *ui.SVG { return objectIcon(t.objKind()) }

// objKind 是 view 还是 table：看侧栏已加载的对象列表里这个名字是不是视图。
func (t *tableTab) objKind() ObjectKind {
	if t.w != nil && !t.isNew {
		for _, info := range t.w.tables {
			if info.Name == t.table && isView(info) {
				return KindView
			}
		}
	}
	return KindTable
}
func (t *tableTab) pending() int { return t.pendingCount() + t.st.changeCount() }

func (t *tableTab) gridView(c *ui.Context) {
	more := c.Modifiers()&ui.Shift != 0
	if t.g.view(c, t.table, t.w.sh.toast) {
		sorted := t.g.sort
		t.g.sort = t.sortOrder // keep the header as it was if the user declines
		t.reload(func() {
			if sorted.Column == "" { // 第三次点击：取消排序
				t.sorted, t.sortOrder, t.g.sort, t.sorts = false, ui.SortOrder{}, ui.SortOrder{}, nil
				t.g.marks = nil
				t.g.rebuild()
				t.page = 1
				return
			}
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
