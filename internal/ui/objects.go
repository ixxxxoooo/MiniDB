package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/database"
	"minidb/locales"
)

// Objects other than tables: views, functions and procedures edited as SQL in
// a tab of their own.

// ---- 定义标签：视图、函数、存储过程 ----

// defTab edits the definition of a view, function or procedure. Applying it
// replaces the object (the app layer restores the old one if the new SQL fails).
type defTab struct {
	w       *Workspace
	kind    string // view, function, procedure
	name    string
	ed      *editor.State
	orig    string
	loading bool
	busy    bool
	err     string
}

func (d *defTab) title() string { return d.name }
func (d *defTab) icon() *ui.SVG {
	if d.kind == "view" {
		return icView
	}
	return icCode
}

// pending is 1 while the text differs from what the database holds.
func (d *defTab) pending() int {
	if d.ed != nil && !d.loading && d.ed.Buffer.String() != d.orig {
		return 1
	}
	return 0
}

// openDefinition shows the definition of an object in a tab, reusing an open one.
func (w *Workspace) openDefinition(kind, name string) {
	for i, t := range w.tabs {
		if d, ok := t.(*defTab); ok && d.kind == kind && d.name == name {
			w.active = i
			return
		}
	}
	buf := editor.NewBuffer("")
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	d := &defTab{w: w, kind: kind, name: name, loading: true}
	d.ed = &editor.State{Buffer: buf, SoftWrap: true}
	w.tabs = append(w.tabs, d)
	w.active = len(w.tabs) - 1
	d.load()
}

func (d *defTab) load() {
	w, db := d.w, d.w.db
	d.loading, d.err = true, ""
	var ddl string
	var err error
	w.sh.up.bg(func() { ddl, err = w.sh.app.Databases.GetRoutineDDL(w.cfg.ID, db, d.kind, d.name) }, func() {
		d.loading = false
		if err != nil {
			d.err = err.Error()
			return
		}
		d.orig = ddl
		d.ed.Buffer.Set(ddl)
	})
}

func (d *defTab) apply() {
	ddl := strings.TrimSpace(d.ed.Buffer.String())
	if d.busy || d.loading || ddl == "" || ddl == strings.TrimSpace(d.orig) || d.w.cfg.ReadOnly {
		return
	}
	w := d.w
	w.sh.safeGate(w, []string{ddl}, func() {
		d.busy = true
		db := w.db
		started := time.Now()
		var err error
		w.sh.up.bg(func() { err = w.sh.app.Databases.ApplyRoutineDDL(w.cfg.ID, db, d.kind, d.name, ddl) }, func() {
			d.busy = false
			errText := ""
			if err != nil {
				errText = err.Error()
			}
			w.sh.logCommit(db, []string{ddl}, 0, time.Since(started), errText)
			if err != nil {
				d.err = locales.ObjApplyFail(err.Error())
				w.sh.toastErr(d.err)
				return
			}
			d.err, d.orig = "", ddl
			w.sh.toastOK(locales.ObjApplied())
			w.loadTables()
		})
	})
}

func (d *defTab) view(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(6).Padding(0, 8).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
			ui.Icon(c, d.icon()).FontSize(13).TextColor(iconColor(p, "view"))
			ui.Text(c, d.name).FontWeight(600).FontSize(font2XS)
			ui.Spacer(c)
			if d.busy || d.loading {
				ui.Spinner(c)
			}
			if actPrimary(c, icCheck, locales.ObjApply()+" (⌘S)").
				Disabled(d.busy || d.loading || d.pending() == 0 || d.w.cfg.ReadOnly).Clicked() {
				d.apply()
			}
		})
	}).Padding(0)
	hairline(c, p.border)
	if d.err != "" {
		ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Padding(8, 12).Selectable()
	}
	if sz := d.w.sh.prefs.EditorFont; sz > 0 {
		d.ed.Font.Size = sz
	}
	editorBars(c, d.ed)
	editor.View(c, d.ed).Grow(1).FillWidth().AutoFocus()
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		d.apply()
	}
}

// ---- 侧栏里的函数与存储过程 ----

// loadRoutines lists the functions and procedures of the database.
func (w *Workspace) loadRoutines() {
	if w.cfg.Type == "sqlite" || w.cfg.Type == "starrocks" {
		w.routines = nil
		return
	}
	db := w.db
	var rs []database.RoutineInfo
	w.sh.up.bg(func() { rs, _ = w.sh.app.Databases.GetRoutines(w.cfg.ID, db) }, func() {
		if db == w.db {
			w.routines = rs
		}
	})
}

func (w *Workspace) routineRow(c *ui.Context, p tokens, r sideRow) {
	name := r.routineName
	key := r.section + "/" + name
	picked := w.side.picked == key
	row := tipOn(c, ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).Padding(0, 12).Height(24).Radius(6).Margin(0, 4).FillWidth(), name)
	if picked {
		row.Background(p.accent.Alpha(0.12))
	}
	row.Children(func() {
		col, txt := p.iconQuery, p.fg
		if picked {
			col, txt = p.accent, p.accent
		}
		ui.Icon(c, icCode).FontSize(12).TextColor(col)
		ui.Text(c, name).FontSize(font2XS).TextColor(txt).SingleLine().Grow(1)
	})
	kind := r.kind
	switch {
	case row.DoubleClicked(), row.Clicked():
		w.side.picked = key
		if row.DoubleClicked() {
			w.openDefinition(kind, name)
		}
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.ObjEditDefinition()).Chosen() {
			defer w.openDefinition(kind, name)
		}
		if m.Item(locales.ObjCopyDefinition()).Chosen() {
			defer w.copyDefinition(kind, name)
		}
		m.Separator()
		if m.Item(locales.ObjDropRoutine()).Disabled(w.cfg.ReadOnly).Chosen() {
			defer w.dropObject(kind, name)
		}
	})
}

// copyDefinition puts an object's definition on the clipboard.
func (w *Workspace) copyDefinition(kind, name string) {
	db := w.db
	var ddl string
	var err error
	w.sh.up.bg(func() { ddl, err = w.sh.app.Databases.GetRoutineDDL(w.cfg.ID, db, kind, name) }, func() {
		if err != nil {
			w.sh.toastErr(err.Error())
			return
		}
		w.sh.copyText(ddl)
	})
}

// dropObject drops a view, function or procedure after asking.
func (w *Workspace) dropObject(kind, name string) {
	title, msg := locales.ObjDropRoutineTitle(), locales.ObjDropRoutineMsg(name)
	stmt := "DROP " + strings.ToUpper(kind) + " " + name
	if kind == "view" {
		title, msg = locales.ObjDropViewTitle(), locales.ObjDropViewMsg(name)
	}
	w.sh.askDanger(title, msg, locales.CommonDelete(), func() {
		w.sh.safeGatePassword(w, []string{stmt}, func() {
			db := w.db
			var err error
			w.sh.up.bg(func() {
				if kind == "view" {
					err = w.sh.app.Databases.DropView(w.cfg.ID, db, name)
				} else {
					err = w.sh.app.Databases.DropRoutine(w.cfg.ID, db, kind, name)
				}
			}, func() {
				if err != nil {
					w.sh.toastErr(err.Error())
					return
				}
				for i := len(w.tabs) - 1; i >= 0; i-- {
					switch t := w.tabs[i].(type) {
					case *defTab:
						if t.kind == kind && t.name == name {
							t.orig = t.ed.Buffer.String()
							w.closeTab(i)
						}
					case *tableTab:
						if kind == "view" && t.table == name {
							t.revert()
							w.closeTab(i)
						}
					}
				}
				w.sh.toastOK(locales.ObjDone())
				w.loadTables()
			})
		})
	})
}

// isViewName reports whether a table of the sidebar is a view.
func (w *Workspace) isViewName(name string) bool {
	for _, t := range w.tables {
		if t.Name == name {
			return isView(t)
		}
	}
	return false
}

// cloneTable asks for a name and copies the table, with its rows if asked.
func (w *Workspace) cloneTable(name string) {
	w.sh.prompt(promptDialog{title: locales.TableCloneTitle(), sub: name, label: locales.TableCloneName(), value: name + "_copy",
		ok: locales.TableCloneOk(), checkLabel: locales.TableCloneData(),
		action: func(dst string, withData bool, _ string) {
			stmts := []string{"CREATE TABLE " + dst}
			w.sh.safeGate(w, stmts, func() {
				db := w.db
				var err error
				w.sh.up.bg(func() { err = w.sh.app.Databases.CloneTable(w.cfg.ID, db, name, dst, withData) }, func() {
					if err != nil {
						w.sh.toastErr(err.Error())
						return
					}
					w.sh.toastOK(locales.TableCloneDone(dst))
					w.loadTables()
					w.openTable(dst)
				})
			})
		}})
}
