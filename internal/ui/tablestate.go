package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
)

// pageState 是一页数据的取数与分页状态（取数见 tableload.go）。
type pageState struct {
	loading   bool
	started   bool // load has been called at least once
	loadStart time.Time
	err       string
	seq       int // discards responses of superseded loads

	total int64
	dur   time.Duration // 这一页的查询耗时

	page     int // 1-based
	pageSize string
}

// sortState 是表格的排序状态（逻辑见 tablesort.go）。
type sortState struct {
	sortOrder ui.SortOrder
	sorted    bool
	sorts     []app.Sort // every sort column, in order: ⇧ click adds one
}

// filterState 是过滤条的状态（逻辑见 tablefilter.go）。
type filterState struct {
	showFilter    bool
	focusFilter   bool
	filterOr      bool // the conditions join with OR instead of AND
	filterFocused bool // a filter field has the keyboard (⌘I, ⌘⇧I and ⌘B are the bar's then)

	filters    []filterRow // the rows of the filter bar
	applied    []app.Filter
	appliedRaw string
}

// tablePanes 是数据页上的侧栏与浮层：行详情、显示列、快速取值菜单、SQL 预览、Quick Look。
type tablePanes struct {
	showPreview bool
	previewW    float32 // width of the row pane
	pv          rowPane // the row pane

	jumpState      // 跳到列（⌘L）
	colsOpen  bool // the "columns shown" dialog
	colsQ     string
	quick     *quickCell // the Set Value menu
	sqlOpen   bool       // the SQL preview
	qlOpen    bool       // Quick Look
	ql        quickLook
}
