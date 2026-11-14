package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
)

// 排序的纯逻辑挂在 sortState 上，不依赖 tableTab，可以单独测试。

// sortList is the sort the table is loaded with.
func (t *sortState) sortList() []app.Sort {
	if len(t.sorts) > 0 {
		return t.sorts
	}
	if t.sorted && t.sortOrder.Column != "" {
		return []app.Sort{sortOf(t.sortOrder)}
	}
	return nil
}

func sortOf(o ui.SortOrder) app.Sort {
	dir := "ASC"
	if o.Descending {
		dir = "DESC"
	}
	return app.Sort{Column: o.Column, Direction: dir}
}

// addSort puts a column's sort in: alone, or after the others when more is true
// (⇧ click on a header). Clicking a column already sorted flips its direction.
func (t *sortState) addSort(o ui.SortOrder, more bool) {
	s := sortOf(o)
	if !more {
		t.sorts = []app.Sort{s}
		return
	}
	for i := range t.sorts {
		if t.sorts[i].Column == s.Column {
			t.sorts[i] = s
			return
		}
	}
	if len(t.sorts) == 0 && t.sorted && t.sortOrder.Column != "" && t.sortOrder.Column != s.Column {
		t.sorts = []app.Sort{sortOf(t.sortOrder)} // what the header showed so far comes first
	}
	t.sorts = append(t.sorts, s)
}

// sortMarks numbers the sort columns in their headers when there are several.
func (t *sortState) sortMarks() map[string]sortMark {
	if len(t.sorts) < 2 {
		return nil
	}
	marks := map[string]sortMark{}
	for i, s := range t.sorts {
		marks[s.Column] = sortMark{desc: s.Direction == "DESC", n: i + 1}
	}
	return marks
}
