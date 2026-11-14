package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

func (q *queryTab) resultsView(c *ui.Context) {
	th := c.Theme()
	if q.page == pagePlan && q.plan != nil {
		q.planView(c, q.w.sh.tokens(c))
		return
	}
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
		if len(q.results) > 1 {
			labels := make([]string, len(q.results))
			for i, r := range q.results {
				labels[i] = r.label(i)
			}
			if i := tabStrip(c, pal, "results", labels, q.shown, connColor(c, cfgColor(q.w.cfg)), false, nil); i >= 0 {
				q.shown = i
			}
		}
		if q.shown >= len(q.results) {
			q.shown = len(q.results) - 1
		}
		r := q.results[q.shown]
		charting := r.res.Error == "" && len(r.res.Columns) > 0
		switch {
		case q.page == pageMessages:
			q.messagesView(c)
		case q.page == pageChart && charting:
			q.chartView(c, r, q.w.sh.tokens(c))
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
					})
					if r.showPreview {
						r.previewHandle(c, q.w.sh.tokens(c))
						ui.Column(c.Key("rpreview")).Width(r.previewW).FillHeight().Shrink(0).Children(func() { r.resultPreview(c, q.w.sh.tokens(c), q.w.sh) })
					}
				})
			}
		}
		ui.Box(c).Height(1).Background(th.Border).FillWidth()
		ui.Row(c).Gap(12).Padding(4, 12).AlignItems(ui.Center).FillWidth().Background(pal.surfaceSecondary).Children(func() {
			// 状态点：成功绿、失败红
			dot := pal.success
			if r.res.Error != "" {
				dot = pal.danger
			}
			ui.Box(c).Size(7, 7).Radius(3.5).Background(dot).Shrink(0)
			// 耗时在前，行数（或翻页栏）在后
			ui.Text(c, fmtDur(resultDur(r.res))).FontSize(12).TextColor(th.TextMuted)
			if r.res.Error == "" && len(r.res.Columns) > 0 {
				switch {
				case r.find != "" && r.paged():
					// 分页结果里只搜得到当前页：文案要明说，不能让人以为"没找到"就是整个结果里没有
					ui.Text(c, locales.QueryFoundPage(len(r.g.rows), len(r.all))).FontSize(12).TextColor(th.TextMuted)
				case r.find != "":
					ui.Text(c, locales.QueryFound(len(r.g.rows), len(r.all))).FontSize(12).TextColor(th.TextMuted)
				case r.paged():
					q.pager(c, pal, r)
				default:
					ui.Text(c, locales.TableRows(len(r.res.Rows))).FontSize(12).TextColor(th.TextMuted)
				}
			}
			ui.Spacer(c)
			if r.res.Error == "" && len(r.res.Columns) > 0 && q.page == pageData {
				findHint := locales.QueryFindInResult()
				if r.paged() {
					findHint = locales.QueryFindInPage()
				}
				if ui.SearchField(c.Key("rfind"), &r.find).Placeholder(findHint).FontSize(font2XS).Width(180).Height(sizeBtnSm).Changed() {
					r.narrow()
				}
			}
			pin := locales.QueryPin()
			if r.pinned {
				pin = locales.QueryUnpin()
			}
			if tipBtn(c.Key("pin"), icPin, pin).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				r.pinned = !r.pinned
			}
			q.pageTabs(c, pal)
			if r.res.Error == "" && len(r.res.Columns) > 0 && r.stmt != "" {
				if actBtn(c, icDownload, locales.QueryExportResult()).Clicked() {
					q.w.sh.exportQuery(q.w, r.stmt)
				}
			}
		})
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
	r.paging = true
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

// pager 是结果底栏里的翻页：当前显示第几行到第几行、上一页 / 下一页、第几页 / 共几页，以及每页行数。
func (q *queryTab) pager(c *ui.Context, p tokens, r *queryResult) {
	from := (r.page-1)*r.pageSize + 1
	to := from + len(r.res.Rows) - 1
	ui.Text(c, locales.QueryRange(from, to, int(r.res.Total))).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
	pages := r.pages()
	if tipBtn(c.Key("rprev"), icChevronR, locales.QueryPagePrev()).Size(18, 18).Disabled(r.page <= 1 || r.paging).Rotate(180).Clicked() {
		q.gotoPage(r, r.page-1, 0)
	}
	ui.Text(c, fmt.Sprintf("%d/%d", r.page, pages)).FontSize(font2XS).TextColor(p.fgSecondary)
	if tipBtn(c.Key("rnext"), icChevronR, locales.QueryPageNext()).Size(18, 18).Disabled(r.page >= pages || r.paging).Clicked() {
		q.gotoPage(r, r.page+1, 0)
	}
	size := strconv.Itoa(r.pageSize)
	labels := []string{"100", "500", "1000", "5000", "10000"}
	if !slices.Contains(labels, size) { // 设置里的上限不在列表里：照样列出来
		labels = append(labels, size)
	}
	if ui.Select(c.Key("rsize"), &size, labels).MinWidth(0).Width(72).Height(sizeBtnSm).FontSize(font2XS).Changed() {
		n, _ := strconv.Atoi(size)
		q.gotoPage(r, 1, n)
	}
}
