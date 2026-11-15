package ui

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 查询页并入底部状态栏的部分（设计稿：结果区自己的底栏取消，两条合成一条）：
// 左边是「状态点 耗时 · 行范围（或查找命中数）· 翻页 · 每页行数」，右边是「在当前页中查找 · 固定 · 导出结果」。

// shownResult 是当前显示的结果，没有结果或这一轮执行失败时为 nil。
func (q *queryTab) shownResult() *queryResult {
	if len(q.results) == 0 || q.failed != "" {
		return nil
	}
	r := q.results[min(q.shown, len(q.results)-1)]
	if r.res == nil {
		return nil
	}
	return r
}

// hasRows 报告结果是不是有结果集的成功结果（可查找、可导出）。
func (r *queryResult) hasRows() bool { return r.res.Error == "" && len(r.res.Columns) > 0 }

// queryStatus 画状态栏左边属于查询页的部分。参数：p 是令牌。还没运行或整轮失败时什么也不画。
func (q *queryTab) queryStatus(c *ui.Context, p tokens) {
	r := q.shownResult()
	if r == nil {
		return
	}
	dot := p.success // 状态点：成功绿、失败红
	if r.res.Error != "" {
		dot = p.danger
	}
	ui.Box(c.Key("q-dot")).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(dot).Shrink(0)
	ui.Text(c, fmtDur(resultDur(r.res))).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine()
	if !r.hasRows() {
		return
	}
	muted := func(s string) { ui.Text(c, s).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine() }
	switch {
	case r.find != "" && r.paged():
		// 分页结果里只搜得到当前页：文案要明说，不能让人以为「没找到」就是整个结果里没有
		muted(locales.QueryFoundPage(len(r.g.rows), len(r.all)))
	case r.find != "":
		muted(locales.QueryFound(len(r.g.rows), len(r.all)))
	case r.paged():
		q.pager(c, p, r)
	default:
		muted(locales.TableRows(len(r.res.Rows)))
	}
}

// tabStatusRight 画状态栏右边属于当前标签的部分（目前只有查询页：查找、固定、导出结果）。
func (w *Workspace) tabStatusRight(c *ui.Context, p tokens) {
	if q, ok := w.activeTab().(*queryTab); ok && q.page != pagePlan {
		q.queryStatusRight(c, p)
	}
}

// queryStatusRight 画「在当前页中查找 · 固定 · 导出结果」。
func (q *queryTab) queryStatusRight(c *ui.Context, p tokens) {
	r := q.shownResult()
	if r == nil {
		return
	}
	if r.hasRows() && q.page == pageData {
		hint := locales.QueryFindInResult()
		if r.paged() {
			hint = locales.QueryFindInPage()
		}
		if ui.SearchField(c.Key("rfind"), &r.find).Placeholder(hint).FontSize(fontCaption).Width(sizeStatusFind).Height(sizeBtnSm).Changed() {
			r.narrow()
		}
	}
	pin := locales.QueryPin()
	if r.pinned {
		pin = locales.QueryUnpin()
	}
	if toggleBtn(c.Key("pin"), icPin, pin, r.pinned).Size(sizeBtnSm, sizeBtnSm).Clicked() {
		r.pinned = !r.pinned
	}
	if r.hasRows() && r.stmt != "" {
		if ghostBtn(c, icDownload, locales.QueryExportResult()).Clicked() {
			q.w.sh.exportQuery(q.w, r.stmt)
		}
	}
}

// pager 是状态栏里的翻页：第几行到第几行、上一页 / 下一页、第几页 / 共几页，以及每页行数。
func (q *queryTab) pager(c *ui.Context, p tokens, r *queryResult) {
	from := (r.page-1)*r.pageSize + 1
	to := from + len(r.res.Rows) - 1
	ui.Text(c, locales.QueryRange(from, to, int(r.res.Total))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	pages := r.pages()
	if tipBtn(c.Key("rprev"), icChevronR, locales.QueryPagePrev()).Size(sizeBtnSm, sizeBtnSm).Disabled(r.page <= 1 || r.paging).Rotate(180).Clicked() {
		q.gotoPage(r, r.page-1, 0)
	}
	ui.Text(c, fmt.Sprintf("%d/%d", r.page, pages)).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine()
	if tipBtn(c.Key("rnext"), icChevronR, locales.QueryPageNext()).Size(sizeBtnSm, sizeBtnSm).Disabled(r.page >= pages || r.paging).Clicked() {
		q.gotoPage(r, r.page+1, 0)
	}
	size := strconv.Itoa(r.pageSize)
	labels := []string{"100", "500", "1000", "5000", "10000"}
	if !slices.Contains(labels, size) { // 设置里的上限不在列表里：照样列出来
		labels = append(labels, size)
	}
	if ui.Select(c.Key("rsize"), &size, labels).MinWidth(0).Width(sizeStatusSel).Height(sizeBtnSm).FontSize(fontCaption).Changed() {
		n, _ := strconv.Atoi(size)
		q.gotoPage(r, 1, n)
	}
}
