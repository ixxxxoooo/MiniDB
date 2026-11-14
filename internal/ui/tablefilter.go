package ui

import (
	"strings"

	"minidb/internal/app"
	"minidb/locales"
)

var filterOps = []string{"=", "!=", ">", "<", ">=", "<=", "LIKE", "NOT LIKE", "IN", "IS NULL", "IS NOT NULL"}

// filterRow is one condition of the filter bar: ticked ones apply together.
type filterRow struct {
	on           bool
	col, op, val string
}

func newFilterRow() filterRow {
	return filterRow{on: true, col: locales.TableFilterAny(), op: "="}
}

// compile 把过滤条里勾选的行整理成发给后端的条件：applied 是列条件，appliedRaw 是手写的 SQL 片段。
// 纯逻辑，只读写 filterState，不依赖 tableTab，可以单独测试。
func (f *filterState) compile() {
	f.applied, f.appliedRaw = nil, ""
	for _, r := range f.filters {
		if !r.on {
			continue
		}
		if r.col == locales.DatagridRawSQL() {
			if txt := strings.TrimSpace(r.val); txt != "" && f.appliedRaw == "" {
				f.appliedRaw = txt
			}
			continue
		}
		op := r.op
		if strings.TrimSpace(r.val) == "" && op != "IS NULL" && op != "IS NOT NULL" {
			continue
		}
		col := r.col
		if col == locales.TableFilterAny() {
			col = "__any"
			if op != "=" && op != "LIKE" {
				op = "LIKE"
			}
		}
		f.applied = append(f.applied, app.Filter{Column: col, Operator: op, Value: r.val, Or: f.filterOr && len(f.applied) > 0})
	}
}

// hasFilter reports whether anything is applied or typed in the filter bar.
func (f *filterState) hasFilter() bool {
	if len(f.applied) > 0 || f.appliedRaw != "" {
		return true
	}
	for _, r := range f.filters {
		if strings.TrimSpace(r.val) != "" {
			return true
		}
	}
	return false
}

// reset 清空过滤条和已应用的条件。
func (f *filterState) reset() {
	f.filters = []filterRow{newFilterRow()}
	f.applied, f.appliedRaw = nil, ""
}

func (t *tableTab) applyFilter() { t.reload(t.setFilter) }

func (t *tableTab) setFilter() {
	t.compile()
	t.page = 1
}

func (t *tableTab) clearFilter() {
	t.reload(func() {
		t.reset()
		t.page = 1
	})
}
