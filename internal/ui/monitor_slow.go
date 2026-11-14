package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// 监控窗口的「慢查询」页签：按平均耗时排序，选中一行可以复制、在新查询里打开、看执行计划、终止同类的运行中语句。

type slowState struct {
	items   []app.SlowQuery
	err     string
	loading bool
	loaded  bool
	sel     int             // 选中的行；-1 没选
	orig    []app.SlowQuery // 数据来源给的顺序，点表头排序时从它出发，第三次点击就能回到这里
	sort    sortSpec
}

// slowCmp 返回按 col 列比较两条慢查询的函数。
func slowCmp(col string) func(a, b app.SlowQuery) int {
	switch col {
	case "sql":
		return func(a, b app.SlowQuery) int { return strings.Compare(strings.ToLower(a.SQL), strings.ToLower(b.SQL)) }
	case "db":
		return func(a, b app.SlowQuery) int {
			return strings.Compare(strings.ToLower(a.Database), strings.ToLower(b.Database))
		}
	case "calls":
		return func(a, b app.SlowQuery) int { return cmp.Compare(a.Calls, b.Calls) }
	case "avg":
		return func(a, b app.SlowQuery) int { return cmp.Compare(a.AvgMs, b.AvgMs) }
	case "max":
		return func(a, b app.SlowQuery) int { return cmp.Compare(a.MaxMs, b.MaxMs) }
	case "rows":
		return func(a, b app.SlowQuery) int { return cmp.Compare(a.Rows, b.Rows) }
	case "last":
		return func(a, b app.SlowQuery) int { return strings.Compare(a.LastSeen, b.LastSeen) }
	}
	return nil
}

// resort 按当前排序重排 items，并让选中的那一行跟着它的语句走（而不是停在原来的行号上）。
func (st *slowState) resort() {
	if st.orig == nil {
		st.orig = slices.Clone(st.items)
	}
	picked, had := st.selectedSlow()
	items := slices.Clone(st.orig)
	if f := slowCmp(st.sort.col); f != nil {
		desc := st.sort.desc
		slices.SortStableFunc(items, func(a, b app.SlowQuery) int {
			if desc {
				return f(b, a)
			}
			return f(a, b)
		})
	}
	st.items = items
	st.sel = -1
	if had {
		for i, it := range items {
			if it.SQL == picked.SQL && it.Database == picked.Database {
				st.sel = i
				break
			}
		}
	}
}

func (s *Shell) loadSlow() {
	st := &s.proc.mon.slow
	w := s.proc.w
	st.loading = true
	var items []app.SlowQuery
	var err error
	s.up.bg(func() { items, err = s.app.Monitor.SlowQueries(w.cfg.ID, 50) }, func() {
		st.loading, st.loaded = false, true
		st.sel = -1
		if err != nil {
			st.err, st.items = errMessage(err), nil
			return
		}
		st.err, st.orig = "", slices.Clone(items)
		st.items = items
		st.resort() // 保持用户选的排序
	})
}

// slowMs 把毫秒写成短文本：不到 1 秒保留毫秒，之后用秒。
func slowMs(ms float64) string {
	if ms < 1000 {
		return locales.MonitorMs(formatNumber(ms))
	}
	return fmt.Sprintf("%.2f s", ms/1000)
}

func (s *Shell) slowBody(c *ui.Context, p tokens) {
	st := &s.proc.mon.slow
	if st.err != "" {
		ui.Row(c).Gap(6).AlignItems(ui.Start).FillWidth().Shrink(0).Children(func() {
			ui.Icon(c, icInfo).FontSize(13).TextColor(p.warning)
			ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, locales.MonitorSlowUnavailable()).FontWeight(600).FontSize(font2XS)
				ui.Text(c, st.err).FontSize(font2XS).TextColor(p.fgSecondary).Selectable()
			})
		})
		return
	}
	if st.loaded && len(st.items) == 0 {
		ui.Text(c, locales.MonitorSlowEmpty()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		return
	}
	if len(st.items) > 0 {
		ui.Text(c, locales.MonitorSlowSource(st.items[0].Source)).FontSize(font2XS - 1).TextColor(p.fgMuted)
	}
	dlgBox(c, p).Grow(1).FillWidth().Children(func() {
		ui.Row(c).Gap(10).Padding(0, 10).Height(26).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
			// 点表头排序：文字列先升序，数值列先看最大的；再点翻转，第三次点回到原顺序
			sorted := false
			head := func(key, label string, w float32, descFirst bool) {
				if sortHead(c, p, key, label, w, &st.sort) {
					st.sort.click(key, descFirst)
					sorted = true
				}
			}
			head("sql", locales.MonitorSlowStatement(), 0, false)
			head("db", locales.MonitorSlowDb(), 80, false)
			head("calls", locales.MonitorSlowCalls(), 56, true)
			head("avg", locales.MonitorSlowAvg(), 72, true)
			head("max", locales.MonitorSlowMax(), 72, true)
			head("rows", locales.MonitorSlowRows(), 64, true)
			head("last", locales.MonitorSlowLast(), 128, true)
			if sorted {
				defer st.resort()
			}
		})
		hairline(c, p.borderSubtle)
		scrollbars(c, ui.Scroll(c.Key("slow")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for i, q := range st.items {
					i, q := i, q
					row := ui.Row(c.Key(fmt.Sprint("slow", i))).Gap(10).Padding(0, 10).Height(28).AlignItems(ui.Center).FillWidth()
					switch {
					case i == st.sel:
						row.Background(p.accent.Alpha(0.12))
					case row.Hovered():
						row.Background(p.hover)
					case i%2 == 1:
						row.Background(p.surface)
					}
					row.Children(func() {
						ui.RichText(c, sqlLineSpans(p, q.SQL, p.fg)...).Grow(1).MinWidth(0).FontSize(font2XS).Font(mono).SingleLine() // 列表里也做语法高亮
						ui.Text(c, q.Database).Width(80).FontSize(font2XS).SingleLine().TextColor(p.fgSecondary)
						ui.Text(c, fmt.Sprint(q.Calls)).Width(56).FontSize(font2XS)
						ui.Text(c, slowMs(q.AvgMs)).Width(72).FontSize(font2XS)
						ui.Text(c, slowMs(q.MaxMs)).Width(72).FontSize(font2XS)
						rows := "—"
						if q.Rows >= 0 {
							rows = formatNumber(float64(q.Rows))
						}
						ui.Text(c, rows).Width(64).FontSize(font2XS)
						ui.Text(c, q.LastSeen).Width(128).FontSize(font2XS).SingleLine().TextColor(p.fgMuted)
					})
					if row.Clicked() {
						st.sel = i
					}
				}
			})
		})
	})
	if st.sel >= 0 && st.sel < len(st.items) {
		dlgBox(c, p).FillWidth().Shrink(0).Height(86).Children(func() {
			scrollbars(c, ui.Scroll(c.Key("slow-sql")).Grow(1).FillWidth()).Children(func() {
				ui.RichText(c, sqlSpans(p, st.items[st.sel].SQL, p.fg)...).Font(mono).FontSize(font2XS).Selectable().Padding(6, 10).FillWidth()
			})
		})
	}
}

// selectedSlow 返回选中的慢查询。
func (st *slowState) selectedSlow() (app.SlowQuery, bool) {
	if st.sel < 0 || st.sel >= len(st.items) {
		return app.SlowQuery{}, false
	}
	return st.items[st.sel], true
}

func (s *Shell) slowFoot(c *ui.Context) {
	st := &s.proc.mon.slow
	w := s.proc.w
	q, ok := st.selectedSlow()
	if footBtn(c, locales.MonitorSlowCopy()).Disabled(!ok).Clicked() {
		s.copyText(q.SQL)
	}
	if footBtn(c, locales.MonitorSlowOpen()).Disabled(!ok).Clicked() {
		s.openSlowInQuery(q, false)
	}
	if app.CanExplain(w.cfg.Type) && footBtn(c, locales.MonitorSlowExplain()).Disabled(!ok).Clicked() {
		s.openSlowInQuery(q, true)
	}
	if footPrimary(c, locales.MonitorSlowKill()).Disabled(!ok || w.cfg.ReadOnly).Clicked() {
		s.killMatching(q)
	}
}

// openSlowInQuery 在新的查询标签里放入这条语句，并可以直接看它的执行计划。
func (s *Shell) openSlowInQuery(q app.SlowQuery, explain bool) {
	w := s.proc.w
	s.proc.open = false
	tab := w.newQuery(q.SQL)
	if explain && tab != nil {
		tab.explainStmt(q.SQL, false)
	}
	s.redraw()
}

// sameStatement 判断运行中的语句 info 是不是慢查询 sql 的同一条：忽略大小写和空白差异，
// 进程列表里的语句可能被截断，所以只要一方是另一方的前缀（至少 20 个字符）就算同一条。
func sameStatement(info, sql string) bool {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	a, b := norm(info), norm(sql)
	if len([]rune(a)) < 20 || len([]rune(b)) < 20 {
		return a != "" && a == b
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// killMatching 终止正在运行的、与 q 相同的语句（只中断语句，保留连接）。
func (s *Shell) killMatching(q app.SlowQuery) {
	w := s.proc.w
	var killed int
	var err error
	s.up.bg(func() {
		var procs []app.ProcessInfo
		if procs, err = s.app.Databases.GetProcessList(w.cfg.ID); err != nil {
			return
		}
		for _, pr := range procs {
			if !sameStatement(pr.Info, q.SQL) {
				continue
			}
			if err = s.app.Databases.KillProcess(w.cfg.ID, pr.ID, true); err != nil {
				return
			}
			killed++
		}
	}, func() {
		switch {
		case err != nil:
			s.toastErr(errMessage(err))
		case killed == 0:
			s.toastWarn(locales.MonitorSlowNoRunning())
		default:
			s.toastOK(locales.MonitorSlowKilled(killed))
		}
	})
}
