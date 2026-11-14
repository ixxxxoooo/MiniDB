package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

type processDialog struct {
	open    bool
	w       *Workspace
	items   []database.ProcessInfo
	err     string
	loading bool
	sel     map[string]bool // 勾选的进程 ID
	dbSel   string          // 按数据库过滤，空为全部
}

// visible 返回通过数据库过滤的进程。
func (d *processDialog) visible() []database.ProcessInfo {
	if d.dbSel == "" {
		return d.items
	}
	var out []database.ProcessInfo
	for _, it := range d.items {
		if it.Database == d.dbSel {
			out = append(out, it)
		}
	}
	return out
}

// databases 列出进程里出现过的数据库名。
func (d *processDialog) databases() []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range d.items {
		if it.Database != "" && !seen[it.Database] {
			seen[it.Database] = true
			out = append(out, it.Database)
		}
	}
	sort.Strings(out)
	return out
}

// selected 返回勾选且可见的进程，按列表顺序。
func (d *processDialog) selected() []database.ProcessInfo {
	var out []database.ProcessInfo
	for _, it := range d.visible() {
		if d.sel[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

// processTSV 把进程写成带表头的制表符分隔文本，方便贴进表格或工单。
func processTSV(items []database.ProcessInfo) string {
	var b strings.Builder
	b.WriteString("ID\tUser\tHost\tDatabase\tCommand\tTime\tState\tInfo\n")
	clean := func(s string) string { return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s) }
	for _, it := range items {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n", it.ID, clean(it.User), clean(it.Host), clean(it.Database), clean(it.Command), it.TimeSec, clean(it.State), clean(it.Info))
	}
	return b.String()
}

func (s *Shell) openProcesses(w *Workspace) {
	s.proc = processDialog{open: true, w: w, sel: map[string]bool{}}
	if !database.SupportsProcessList(w.cfg.Type) {
		s.proc.err = locales.ProcessUnsupported()
		return
	}
	s.reloadProcesses()
}

func (s *Shell) reloadProcesses() {
	d := &s.proc
	w := d.w
	d.loading = true
	var items []database.ProcessInfo
	var err error
	s.up.bg(func() { items, err = s.app.Databases.GetProcessList(w.cfg.ID) }, func() {
		d.loading = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.err, d.items = "", items
		// 已经不存在的进程不再保持勾选
		alive := map[string]bool{}
		for _, it := range items {
			alive[it.ID] = true
		}
		for id := range d.sel {
			if !alive[id] {
				delete(d.sel, id)
			}
		}
		if d.dbSel != "" {
			found := false
			for _, n := range d.databases() {
				found = found || n == d.dbSel
			}
			if !found {
				d.dbSel = ""
			}
		}
	})
}

// killSelected 终止所有勾选的进程（queryOnly 时只终止它们的查询）。
func (s *Shell) killSelected(queryOnly bool) {
	d := &s.proc
	w := d.w
	picked := d.selected()
	if len(picked) == 0 {
		return
	}
	var failed int
	var firstErr error
	s.up.bg(func() {
		for _, it := range picked {
			if err := s.app.Databases.KillProcess(w.cfg.ID, it.ID, queryOnly); err != nil {
				failed++
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}, func() {
		if failed > 0 {
			s.toastErr(locales.ProcessKillFailed(failed, firstErr.Error()))
		} else {
			s.toastOK(locales.ProcessKilledMany(len(picked)))
		}
		d.sel = map[string]bool{}
		s.reloadProcesses()
	})
}

func (s *Shell) killProcess(id string, queryOnly bool) {
	w := s.proc.w
	var err error
	s.up.bg(func() { err = s.app.Databases.KillProcess(w.cfg.ID, id, queryOnly) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.toastOK(locales.ProcessKilled())
		s.reloadProcesses()
	})
}

func (s *Shell) processView(c *ui.Context) {
	d := &s.proc
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.ProcessTitle(), width: 900, height: 520, extra: func() {
		if d.loading {
			ui.Spinner(c)
		}
		if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			s.reloadProcesses()
		}
	}}, func() {
		if d.err != "" {
			ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
		}
		visible := d.visible()
		ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
			all := len(visible) > 0 && len(d.selected()) == len(visible)
			if ui.Checkbox(c, &all, locales.ProcessSelectAll()).Changed() {
				for _, it := range visible {
					d.sel[it.ID] = all
				}
			}
			ui.Spacer(c)
			dbs := append([]string{locales.ProcessAllDb()}, d.databases()...)
			cur := d.dbSel
			if cur == "" {
				cur = locales.ProcessAllDb()
			}
			if ui.Select(c, &cur, dbs).MinWidth(0).Width(180).Height(sizeBtnSm).Changed() {
				d.dbSel = cur
				if cur == locales.ProcessAllDb() {
					d.dbSel = ""
				}
			}
		})
		dlgBox(c, p).Grow(1).FillWidth().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("proc")).Grow(1).FillWidth()).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for i, it := range visible {
						it, i := it, i
						row := ui.Row(c.Key(it.ID)).Gap(10).Padding(0, 10).Height(30).AlignItems(ui.Center).FillWidth()
						if i%2 == 1 {
							row.Background(p.surface)
						}
						row.Children(func() {
							on := d.sel[it.ID]
							if ui.Checkbox(c.Key("sel"), &on, "").Changed() {
								d.sel[it.ID] = on
							}
							ui.Text(c, it.ID).Width(60).FontSize(font2XS).Font(mono)
							ui.Text(c, it.User).Width(90).FontSize(font2XS).SingleLine()
							ui.Text(c, it.Database).Width(90).FontSize(font2XS).SingleLine()
							ui.Text(c, fmt.Sprint(it.TimeSec)).Width(50).FontSize(font2XS)
							ui.Text(c, oneLine(it.Info)).Grow(1).FontSize(font2XS).Font(mono).SingleLine().TextColor(p.fgSecondary)
							if actBtn(c, icStop, locales.ProcessKillQuery()).Clicked() {
								s.killProcess(it.ID, true)
							}
							if actBtn(c, icUnplug, locales.ProcessKill()).Clicked() {
								s.killProcess(it.ID, false)
							}
						})
					}
				})
			})
		})
	}, func() {
		picked := d.selected()
		// 复制：有勾选就复制勾选的，没有就复制当前可见的全部
		if footBtn(c, locales.ProcessCopy()).Clicked() {
			items := picked
			if len(items) == 0 {
				items = d.visible()
			}
			s.copyText(processTSV(items))
		}
		if footBtn(c, locales.ProcessKillQuery()).Disabled(len(picked) == 0).Clicked() {
			s.killSelected(true)
		}
		if footPrimary(c, locales.ProcessKillSelected(len(picked))).Disabled(len(picked) == 0).Clicked() {
			s.killSelected(false)
		}
	})
}
