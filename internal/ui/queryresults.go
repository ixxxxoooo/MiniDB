package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

func (q *queryTab) resultsView(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		pal := q.w.sh.tokens(c)
		if q.failed != "" {
			q.banner(c, pal, kindError, q.failed, nil)
			return
		}
		if len(q.results) == 0 {
			// 还没运行过：给个提示，而不是一片空白
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icPlay).FontSize(22).TextColor(pal.fgMuted)
				ui.Text(c, locales.QueryEmptyResults()).FontSize(font2XS).TextColor(pal.fgMuted)
				ui.Text(c, strings.Trim(q.w.sh.keyHint("run"), " ()")).Font(mono).FontSize(font2XS).TextColor(pal.fgMuted)
			})
			return
		}
		q.resultHead(c, pal)
		if q.shown >= len(q.results) {
			q.shown = len(q.results) - 1
		}
		r := q.results[q.shown]
		switch {
		case q.page == pageMessages:
			q.messagesView(c)
		default:
			switch {
			case r.res.ErrorCode == "cancelled":
				q.banner(c, pal, kindWarning, locales.QueryCancelled(), nil)
			case r.res.ErrorCode == "timeout":
				q.banner(c, pal, kindError, locales.QueryTimedOut(q.w.sh.prefs.QueryTimeout), nil)
			case r.res.ErrorCode == "read_only":
				q.banner(c, pal, kindError, locales.QueryReadOnly(), nil)
			case r.res.Error != "":
				q.banner(c, pal, kindError, r.res.Error, func() {
					if actBtn(c, icSparkle, locales.QueryAIDiagnose()).Clicked() {
						q.askAI(locales.AiPromptDiagnose(r.stmt, r.res.Error))
					}
				})
			case len(r.res.Columns) == 0:
				q.banner(c, pal, kindSuccess, locales.QueryOkAffected(int(r.res.Total)), nil)
			default:
				// 网格始终在树里同一个位置，预览栏只是加在右边，所以打开它不会让网格滚动位置丢失
				ui.Row(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
					ui.Column(c.Key("rgrid")).Grow(1).MinWidth(0).FillHeight().Children(func() {
						// 分页的结果只有当前一页在内存里：点表头排序要交给数据库对整个结果排，
						// 内存里排只会得到"这一页内部有序"的假象；没分页的结果仍在内存里排
						r.g.local = !r.paged()
						if r.g.view(c, locales.QueryResultN(q.shown+1), q.w.sh.toast) {
							q.sortPaged(r)
						}
						// 点列头排序 / 翻页要等数据库时，结果区顶上一条流光（和数据页加载同一个效果，行不动）
						if r.paging && delayed(c, r.pagingAt, 500*time.Millisecond) {
							ui.Progress(c, -1).Absolute().Top(0).Left(0).Right(0).Height(2)
						}
					})
					if r.showPreview {
						r.previewHandle(c, q.w.sh.tokens(c))
						ui.Column(c.Key("rpreview")).Width(r.previewW).FillHeight().Shrink(0).Children(func() { r.resultPreview(c, q.w.sh.tokens(c), q.w.sh) })
					}
				})
			}
		}
	})
}

// messagesView lists what each statement of the run did.
func (q *queryTab) messagesView(c *ui.Context) {
	p := q.w.sh.tokens(c)
	scrollbars(c, ui.Scroll(c.Key("msgs-list")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).Gap(4).Padding(10, 12).FillWidth().Children(func() {
			for i, r := range q.results {
				col, text := p.success, locales.QueryOkAffected(int(r.res.Total))
				if len(r.res.Columns) > 0 {
					text = locales.TableRows(len(r.res.Rows))
				}
				if r.res.Error != "" {
					col, text = p.danger, r.res.Error
				}
				ui.Row(c.Key(fmt.Sprint("m", i))).Gap(8).AlignItems(ui.Start).FillWidth().Children(func() {
					ui.Text(c, "●").FontSize(font2XS).TextColor(col)
					ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, firstLines(r.stmt, 2)).Font(mono).FontSize(font2XS).Selectable()
						ui.Text(c, text+" · "+fmtDur(resultDur(r.res))).FontSize(font2XS).TextColor(col).Selectable()
					})
				})
			}
		})
	})
}

// banner 在结果区画一条提示：左侧一枚按种类配色的图标，错误信息等宽显示、可选中，
// 右边可以放一个操作（如"让 AI 诊断"）。
func (q *queryTab) banner(c *ui.Context, p tokens, kind, text string, action func()) {
	col := kindColor(p, kind)
	// 外层撑满剩余高度，让底部状态栏始终贴在最下面
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(10, 12).Children(func() {
		ui.Row(c).Gap(10).Padding(10, 12).AlignItems(ui.Start).FillWidth().Radius(radiusBtn+1).
			Background(col.Alpha(0.07)).Border(1, col.Alpha(0.28)).Shrink(0).Children(func() {
			ui.Icon(c, kindIcon(kind)).FontSize(15).TextColor(col).Shrink(0).Margin(1, 0, 0, 0)
			t := ui.Text(c, text).FontSize(font2XS + 1).TextColor(p.fg).Selectable().Grow(1).MinWidth(0)
			if kind == kindError {
				t.Font(mono)
			}
			if action != nil {
				action()
			}
		})
	})
}

// paged 报告这个结果是不是被分页了：自动加了 LIMIT，而且总行数比一页多。
func (r *queryResult) paged() bool {
	return r.res.AutoLimited && r.pageSize > 0 && r.res.Total > int64(r.pageSize)
}

// pages 是总页数。
func (r *queryResult) pages() int {
	return int((r.res.Total + int64(r.pageSize) - 1) / int64(r.pageSize))
}

// gotoPage 取结果的第 page 页（pageSize 不为 0 时同时改每页行数）：只重新执行这一条语句，
// 成功后换上这一页的行，行号接着上一页数；失败时留在原来那一页并提示。
func (q *queryTab) gotoPage(r *queryResult, page, pageSize int) {
	q.loadPage(r, page, pageSize, r.sortApplied)
}

// sortPaged 响应分页结果的表头点击：回到第 1 页，按新的排序让数据库重查。
// 表头先保持原样，查询成功后才换成新的箭头；失败时用户看到的仍是原来的排序。
func (q *queryTab) sortPaged(r *queryResult) {
	sorted := r.g.sort
	r.g.sort = r.sortApplied
	q.loadPage(r, 1, r.pageSize, sorted)
}

// loadPage 取结果的第 page 页，sorted 不为空时对整个结果按该列排序后再分页。
func (q *queryTab) loadPage(r *queryResult, page, pageSize int, sorted ui.SortOrder) {
	if r.paging || q.running {
		return
	}
	if pageSize <= 0 {
		pageSize = r.pageSize
	}
	w := q.w
	db := w.db
	r.paging, r.pagingAt = true, time.Now()
	started := time.Now()
	stmt := app.SortedSQL(w.cfg.Type, r.stmt, sorted.Column, sorted.Descending)
	var res *app.QueryResult
	var err error
	w.sh.up.bg(func() {
		res, err = w.sh.app.Queries.ExecuteSQLPaged(w.cfg.ID, db, stmt, page, pageSize)
	}, func() {
		r.paging = false
		sql := fmt.Sprintf("%s LIMIT %d OFFSET %d", strings.TrimRight(strings.TrimSpace(stmt), "; \n"), pageSize, (page-1)*pageSize)
		switch {
		case err != nil:
			w.sh.toastErr(errMessage(err))
			w.sh.logSQL(db, sql, time.Since(started), -1, errMessage(err))
		case res == nil || res.Error != "":
			msg := locales.QueryNoResult()
			if res != nil {
				msg = res.Error
			}
			w.sh.toastErr(msg)
			w.sh.logSQL(db, sql, time.Since(started), -1, msg)
		default:
			r.res, r.all, r.page, r.pageSize = res, res.Rows, page, pageSize
			r.sortApplied, r.g.sort = sorted, sorted
			r.g.rebuild() // 表头的排序箭头跟着变
			r.g.offset = (page - 1) * pageSize
			r.g.chosen.Clear()
			r.narrow()
			w.sh.logSQL(db, sql, resultDur(res), len(res.Rows), "")
		}
	})
}
