package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"

	"minidb/locales"
)

// queryResult is the outcome of one statement of a run.
type queryResult struct {
	stmt string
	res  *app.QueryResult
	g    *resultGrid

	name   string // from a "-- name: …" comment above the statement
	pinned bool   // kept when the query runs again
	chart  chartState
	find   string           // narrows the rows shown to those containing it
	all    []map[string]any // every row, while find narrows what g shows

	sortApplied ui.SortOrder // 分页结果当前已经按它排好（由数据库排的），翻页时沿用

	// 翻页：没写 LIMIT 的 SELECT 一次只取 pageSize 行，其余的按页取（见 gotoPage）
	page     int  // 当前页，从 1 起
	pageSize int  // 每页行数
	paging   bool // 正在取另一页

	showPreview bool    // 右侧的行预览栏（空格开关）
	previewW    float32 // 预览栏宽度
	pv          rowPane // 预览栏的状态（和表数据页共用同一个组件）
}

// label is the result's tab title.
func (r *queryResult) label(i int) string {
	t := r.name
	if t == "" {
		t = locales.QueryResultN(i + 1)
	}
	if r.pinned {
		t = "◆ " + t
	}
	return t
}

// narrow shows only the rows containing r.find (any cell, case-insensitive).
func (r *queryResult) narrow() {
	rows := r.all
	if q := strings.ToLower(strings.TrimSpace(r.find)); q != "" {
		rows = nil
		for _, row := range r.all {
			for _, v := range row {
				if v != nil && strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
					rows = append(rows, row)
					break
				}
			}
		}
	}
	r.g.set(r.res.Columns, rows)
}
