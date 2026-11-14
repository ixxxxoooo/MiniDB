package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/locales"
)

// promptDialog asks for one line of text, with an optional checkbox and an
// optional choice. It serves "new database", "rename", "clone table" and the
// like, so each of them is a few lines at the call site.
type promptDialog struct {
	open       bool
	title, sub string
	label      string
	value      string
	ok         string
	checkLabel string // shows a checkbox when set
	check      bool
	selLabel   string // shows a choice when options is set
	options    []string
	sel        string
	action     func(value string, check bool, sel string)
}

// prompt opens the dialog; action runs with the trimmed, non-empty text.
func (s *Shell) prompt(p promptDialog) {
	p.open = true
	s.prm = p
}

func (s *Shell) promptView(c *ui.Context) {
	d := &s.prm
	if !d.open {
		return
	}
	p := s.tokens(c)
	done := func() {
		v := strings.TrimSpace(d.value)
		if v == "" {
			return
		}
		act, chk, sel := d.action, d.check, d.sel
		d.open = false
		if act != nil {
			act(v, chk, sel)
		}
	}
	s.dialog(c, &d.open, dlgOpts{title: d.title, sub: d.sub, width: 420}, func() {
		dlgField(c, p, d.label, func() {
			if ui.TextInput(c.Key("prm-v"), &d.value).AutoFocus().FillWidth().Height(sizeBtn).Submitted() {
				defer done()
			}
		})
		if len(d.options) > 0 {
			dlgField(c, p, d.selLabel, func() {
				ui.Select(c.Key("prm-s"), &d.sel, d.options).FillWidth().Height(sizeBtn)
			})
		}
		if d.checkLabel != "" {
			ui.Checkbox(c.Key("prm-c"), &d.check, d.checkLabel)
		}
	}, func() {
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		if footPrimary(c, d.ok).Disabled(strings.TrimSpace(d.value) == "").Clicked() {
			done()
		}
	})
}

// ---- 库内搜索 ----

// dbSearch is "Search in Database…": a term, then the tables that hold it.
type dbSearch struct {
	open    bool
	w       *Workspace
	term    string
	busy    bool
	ran     bool // a search finished, so "no hits" can be told from "not run yet"
	hits    []app.SearchHit
	err     string
	queried string
}

func (s *Shell) openDatabaseSearch() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.srch = dbSearch{open: true, w: w}
}

func (s *Shell) runDatabaseSearch() {
	d := &s.srch
	term := strings.TrimSpace(d.term)
	if term == "" || d.busy || d.w == nil {
		return
	}
	w := d.w
	d.busy, d.err, d.hits, d.ran, d.queried = true, "", nil, false, term
	db := w.db
	var hits []app.SearchHit
	var err error
	s.up.bg(func() { hits, err = s.app.Databases.SearchDatabase(w.cfg.ID, db, term) }, func() {
		d.busy, d.ran = false, true
		if err != nil {
			d.err = err.Error()
			return
		}
		d.hits = hits
	})
}

func (s *Shell) searchView(c *ui.Context) {
	d := &s.srch
	if !d.open || d.w == nil {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.SearchDbTitle(), sub: d.w.dbLabel(), width: 520}, func() {
		ui.Row(c).Gap(8).FillWidth().Children(func() {
			in := ui.SearchField(c.Key("srch-q"), &d.term).Placeholder(locales.SearchDbPlaceholder()).AutoFocus().Grow(1).Height(sizeBtn)
			if in.Submitted() {
				defer s.runDatabaseSearch()
			}
			if fieldPrimary(c.Key("srch-go"), icSearch, locales.SearchDbRun()).Disabled(d.busy || strings.TrimSpace(d.term) == "").Clicked() {
				s.runDatabaseSearch()
			}
		})
		switch {
		case d.busy:
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, locales.SearchDbSearching()).FontSize(font2XS).TextColor(p.fgMuted)
			})
		case d.err != "":
			ui.Text(c, d.err).TextColor(p.danger).Selectable()
		case d.ran && len(d.hits) == 0:
			ui.Text(c, locales.SearchDbNone(d.queried)).TextColor(p.fgMuted).FontSize(font2XS)
		case len(d.hits) > 0:
			dlgBox(c, p).FillWidth().Children(func() {
				scrollbars(c, ui.Scroll(c.Key("srch-list")).MaxHeight(280).FillWidth()).Children(func() {
					ui.Column(c).Gap(1).Padding(4).FillWidth().Children(func() {
						for _, h := range d.hits {
							h := h
							row := ui.Row(c.Key("hit:"+h.Table)).Gap(8).AlignItems(ui.Center).Padding(0, 10).Height(26).Radius(6).FillWidth()
							if row.Hovered() {
								row.Background(p.hover)
							}
							row.Children(func() {
								ui.Icon(c, icTable).FontSize(12).TextColor(iconColor(p, KindTable))
								ui.Text(c, h.Table).FontSize(font2XS).SingleLine().Grow(1)
								ui.Text(c, locales.SearchDbRows(int(h.Count))).FontSize(font2XS).TextColor(p.fgMuted)
							})
							if row.Clicked() {
								d.open = false
								d.w.openTableFiltered(h.Table, locales.TableFilterAny(), d.queried)
							}
						}
					})
				})
			})
		}
	}, nil)
}

// ---- 数据库的新建、重命名与删除 ----

// reloadDatabases refreshes the database list of w and the switcher.
func (s *Shell) reloadDatabases(w *Workspace) {
	var dbs []model.DatabaseInfo
	s.up.bg(func() { dbs, _ = s.app.Databases.GetAllDatabases(w.cfg.ID) }, func() {
		if len(dbs) > 0 {
			for _, o := range s.spaces {
				if o.cfg.ID == w.cfg.ID {
					o.dbs = dbs
				}
			}
		}
	})
}

// dbOpen reports whether a workspace of the connection shows the database.
func (s *Shell) dbOpen(w *Workspace, name string) bool {
	for _, o := range s.spaces {
		if o.cfg.ID == w.cfg.ID && o.db == name {
			return true
		}
	}
	return false
}

// newDatabase asks for a name (and, on MySQL, a character set) and creates it.
func (s *Shell) newDatabase(w *Workspace) {
	if w.cfg.ReadOnly || !database.CapsOf(w.cfg.Type).CreateDatabase {
		return
	}
	pr := promptDialog{title: locales.DbNewTitle(), sub: w.cfg.Name, label: locales.DbName(), ok: locales.DbCreate()}
	if database.CapsOf(w.cfg.Type).Charset {
		pr.selLabel, pr.options, pr.sel = locales.DbCharset(), []string{"utf8mb4", "utf8", "latin1", "gbk"}, "utf8mb4"
	}
	pr.action = func(name string, _ bool, charset string) {
		stmt, err := database.BuildCreateDatabaseSQL(w.cfg.Type, name, charset)
		if err != nil {
			s.toastErr(err.Error())
			return
		}
		s.safeGate(w, []string{stmt}, func() {
			var err error
			s.up.bg(func() { err = s.app.Databases.CreateDatabase(w.cfg.ID, name, charset) }, func() {
				if err != nil {
					s.toastErr(err.Error())
					return
				}
				s.toastOK(locales.ObjDone())
				s.reloadDatabases(w)
			})
		})
	}
	s.prompt(pr)
}

// renameDatabase renames a database that no workspace shows.
func (s *Shell) renameDatabase(w *Workspace, name string) {
	switch {
	case w.cfg.ReadOnly:
		return
	case s.dbOpen(w, name):
		s.toastWarn(locales.DbInUse(name))
		return
	}
	s.prompt(promptDialog{title: locales.DbRenameTitle(), sub: name, label: locales.DbName(), value: name, ok: locales.DbRename(),
		action: func(to string, _ bool, _ string) {
			if to == name {
				return
			}
			stmt, _ := database.BuildRenameDatabaseSQL(w.cfg.Type, name, to)
			if stmt == "" {
				stmt = "RENAME DATABASE " + name + " TO " + to
			}
			s.safeGate(w, []string{stmt}, func() {
				var err error
				s.up.bg(func() { err = s.app.Databases.RenameDatabase(w.cfg.ID, name, to) }, func() {
					if err != nil {
						s.toastErr(err.Error())
						return
					}
					s.toastOK(locales.ObjDone())
					s.reloadDatabases(w)
				})
			})
		}})
}

// dropDatabase deletes a database that no workspace shows, after asking.
func (s *Shell) dropDatabase(w *Workspace, name string) {
	switch {
	case w.cfg.ReadOnly:
		return
	case s.dbOpen(w, name):
		s.toastWarn(locales.DbInUse(name))
		return
	}
	s.askDanger(locales.DbDropTitle(), locales.DbDropMsg(name), locales.DbDrop(), func() {
		s.safeGatePassword(w, []string{"DROP DATABASE " + name}, func() {
			var err error
			s.up.bg(func() { err = s.app.Databases.DropDatabase(w.cfg.ID, name) }, func() {
				if err != nil {
					s.toastErr(err.Error())
					return
				}
				s.toastOK(locales.ObjDone())
				s.reloadDatabases(w)
			})
		})
	})
}
