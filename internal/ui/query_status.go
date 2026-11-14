package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo

// queryStatus 是状态栏左边属于查询页的部分：当前结果的「耗时 · 行数」（设计稿 A），
// 还没运行、出错或没有结果集时什么也不画。
func (q *queryTab) queryStatus(c *ui.Context, p tokens) {
	if len(q.results) == 0 || q.failed != "" {
		return
	}
	r := q.results[min(q.shown, len(q.results)-1)]
	if r.res == nil || r.res.Error != "" || len(r.res.Columns) == 0 {
		return
	}
	ui.Text(c, fmtDur(resultDur(r.res))+" · "+locales.TableRows(len(r.res.Rows))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
}
