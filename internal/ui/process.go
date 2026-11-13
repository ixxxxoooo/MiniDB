package ui

import (
	"fmt"

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
}

func (s *Shell) openProcesses(w *Workspace) {
	s.proc = processDialog{open: true, w: w}
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
			d.err = err.Error()
			return
		}
		d.err, d.items = "", items
	})
}

func (s *Shell) killProcess(id string, queryOnly bool) {
	w := s.proc.w
	var err error
	s.up.bg(func() { err = s.app.Databases.KillProcess(w.cfg.ID, id, queryOnly) }, func() {
		if err != nil {
			s.toast(err.Error())
			return
		}
		s.toast(locales.ProcessKilled())
		s.reloadProcesses()
	})
}

func (s *Shell) processView(c *ui.Context) {
	d := &s.proc
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.ProcessTitle(), width: 860, height: 480, extra: func() {
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
		dlgBox(c, p).Grow(1).FillWidth().Children(func() {
			ui.Scroll(c.Key("proc")).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for i, it := range d.items {
						it, i := it, i
						row := ui.Row(c.Key(it.ID)).Gap(10).Padding(0, 10).Height(30).AlignItems(ui.Center).FillWidth()
						if i%2 == 1 {
							row.Background(p.surface)
						}
						row.Children(func() {
							ui.Text(c, it.ID).Width(60).FontSize(font2XS).Font(mono)
							ui.Text(c, it.User).Width(90).FontSize(font2XS).SingleLine()
							ui.Text(c, it.Database).Width(90).FontSize(font2XS).SingleLine()
							ui.Text(c, fmt.Sprint(it.TimeSec)).Width(50).FontSize(font2XS)
							ui.Text(c, oneLine(it.Info)).Grow(1).FontSize(font2XS).Font(mono).SingleLine().TextColor(p.fgSecondary)
							if ui.Button(c, locales.ProcessKillQuery()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
								s.killProcess(it.ID, true)
							}
							if ui.Button(c, locales.ProcessKill()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
								s.killProcess(it.ID, false)
							}
						})
					}
				})
			})
		})
	}, nil)
}
