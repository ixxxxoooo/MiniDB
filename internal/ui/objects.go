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
// a tab of their own, and the foreign keys and triggers of a table, listed in
// its structure view.

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
				w.sh.toast(d.err)
				return
			}
			d.err, d.orig = "", ddl
			w.sh.toast(locales.ObjApplied())
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
			if ui.PrimaryButton(c, locales.ObjApply()+" (⌘S)").Height(sizeBtnSm).Padding(0, 10).
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
	row := ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).Padding(0, 12).Height(24).Radius(6).Margin(0, 4).FillWidth()
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
			w.sh.toast(err.Error())
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
	w.sh.ask(title, msg, locales.CommonDelete(), func() {
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
					w.sh.toast(err.Error())
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
				w.sh.toast(locales.ObjDone())
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
						w.sh.toast(err.Error())
						return
					}
					w.sh.toast(locales.TableCloneDone(dst))
					w.loadTables()
					w.openTable(dst)
				})
			})
		}})
}

// ---- 结构视图里的外键与触发器 ----

// objState holds a table's foreign keys and triggers, and the two "add" forms.
type objState struct {
	fks   []database.ForeignKeyInfo
	trigs []database.TriggerInfo

	newFK   fkForm
	newTrig trigForm
}

type fkForm struct {
	open       bool
	name       string
	columns    []string
	refTable   string
	refColumns []string
	refOptions []string
	onDelete   string
	onUpdate   string
}

type trigForm struct {
	open   bool
	name   string
	timing string
	event  string
	body   string
}

// canEditFK reports whether the database can add or drop a foreign key on an existing table.
func canEditFK(dbType string) bool { return dbType != "sqlite" && dbType != "starrocks" }

// canTriggers reports whether the database has triggers this app can list.
func canTriggers(dbType string) bool { return dbType != "starrocks" }

func (s *structState) loadObjects(t *tableTab) {
	w, db := t.w, t.w.db
	var fks []database.ForeignKeyInfo
	var trigs []database.TriggerInfo
	w.sh.up.bg(func() {
		fks, _ = w.sh.app.Databases.GetForeignKeys(w.cfg.ID, db, t.table)
		trigs, _ = w.sh.app.Databases.GetTriggers(w.cfg.ID, db, t.table)
	}, func() {
		s.obj.fks, s.obj.trigs = fks, trigs
	})
}

// defaultFKName names a foreign key the way most schemas do.
func defaultFKName(table string, cols []string) string {
	return "fk_" + table + "_" + strings.Join(cols, "_")
}

func (t *tableTab) fkFromForm() database.ForeignKeyInfo {
	f := &t.st.obj.newFK
	name := strings.TrimSpace(f.name)
	if name == "" {
		name = defaultFKName(t.table, f.columns)
	}
	return database.ForeignKeyInfo{Name: name, Columns: f.columns, RefTable: f.refTable, RefColumns: f.refColumns, OnDelete: f.onDelete, OnUpdate: f.onUpdate}
}

func (t *tableTab) triggerFromForm() database.TriggerInfo {
	f := &t.st.obj.newTrig
	return database.TriggerInfo{Name: strings.TrimSpace(f.name), Timing: f.timing, Event: f.event, Definition: strings.TrimSpace(f.body)}
}

// runObjectSQL sends one statement for a foreign key or trigger, behind Safe
// Mode, then reloads the lists. A drop has asked already, so only a password
// level adds a step.
func (t *tableTab) runObjectSQL(stmt string, asked bool) {
	w := t.w
	run := func() {
		db := w.db
		started := time.Now()
		var err error
		w.sh.up.bg(func() { err = w.sh.app.Databases.ExecuteRawSQL(w.cfg.ID, db, stmt) }, func() {
			errText := ""
			if err != nil {
				errText = err.Error()
			}
			w.sh.logCommit(db, []string{stmt}, 0, time.Since(started), errText)
			if err != nil {
				w.sh.toast(locales.StructApplyFail(err.Error()))
				return
			}
			w.sh.toast(locales.ObjDone())
			t.st.loadObjects(t)
		})
	}
	if asked {
		w.sh.safeGatePassword(w, []string{stmt}, run)
	} else {
		w.sh.safeGate(w, []string{stmt}, run)
	}
}

func (t *tableTab) structObjects(c *ui.Context, p tokens, readOnly bool) {
	s := &t.st
	dbType := t.w.cfg.Type
	canFK := canEditFK(dbType)
	ui.Box(c).Height(14).FillWidth()
	tip := locales.StructAddForeignKey()
	if !canFK {
		tip = locales.StructFKUnsupported()
	}
	t.structSection(c, p, locales.StructForeignKeys(), len(s.obj.fks), func() {
		s.obj.newFK = fkForm{open: true, onDelete: "NO ACTION", onUpdate: "NO ACTION"}
	}, readOnly || !canFK, tip)
	if len(s.obj.fks) > 0 {
		ui.Row(c).FillWidth().Background(p.surfaceSecondary).Children(func() {
			structHead(c, p, stW0, "#")
			structHead(c, p, stWName, locales.StructName())
			structHead(c, p, stWType, locales.StructIndexColumns())
			structHead(c, p, 0, locales.StructFKReferences())
			structHead(c, p, stWDef-30, locales.StructFKOnDelete())
			structHead(c, p, stWDef-30, locales.StructFKOnUpdate())
			structHead(c, p, stWAct, "")
		})
	}
	for i, fk := range s.obj.fks {
		fk := fk
		row := ui.Row(c.Key("fk:" + fk.Name)).FillWidth()
		hovered := row.Hovered()
		if i%2 == 1 {
			row.Background(p.surfaceSecondary)
		}
		row.Children(func() {
			structCell(c, p, stW0, func() { ui.Text(c, itoa(i+1)).TextColor(p.fgMuted).FontSize(font2XS) })
			structCell(c, p, stWName, func() { ui.Text(c, fk.Name).SingleLine() })
			structCell(c, p, stWType, func() { ui.Text(c, strings.Join(fk.Columns, ", ")).SingleLine() })
			structCell(c, p, 0, func() {
				ui.Text(c, fk.RefTable+" ("+strings.Join(fk.RefColumns, ", ")+")").SingleLine().TextColor(p.fgSecondary)
			})
			structCell(c, p, stWDef-30, func() { ui.Text(c, fk.OnDelete).SingleLine().TextColor(p.fgSecondary) })
			structCell(c, p, stWDef-30, func() { ui.Text(c, fk.OnUpdate).SingleLine().TextColor(p.fgSecondary) })
			structCell(c, p, stWAct, func() {
				if hovered && !readOnly && canFK {
					if tipBtn(c.Key("fkdel"), icTrash, locales.CommonDelete()).Size(20, 20).Clicked() {
						defer t.dropForeignKey(fk.Name)
					}
				}
			})
		})
	}

	if !canTriggers(dbType) {
		return
	}
	ui.Box(c).Height(14).FillWidth()
	t.structSection(c, p, locales.StructTriggers(), len(s.obj.trigs), func() {
		s.obj.newTrig = trigForm{open: true, timing: "BEFORE", event: "INSERT"}
	}, readOnly, locales.StructAddTrigger())
	if len(s.obj.trigs) > 0 {
		ui.Row(c).FillWidth().Background(p.surfaceSecondary).Children(func() {
			structHead(c, p, stW0, "#")
			structHead(c, p, stWName, locales.StructName())
			structHead(c, p, stWNull+30, locales.StructTriggerTiming())
			structHead(c, p, stWNull+30, locales.StructTriggerEvent())
			structHead(c, p, 0, locales.StructTriggerBody())
			structHead(c, p, stWAct, "")
		})
	}
	for i, tr := range s.obj.trigs {
		tr := tr
		row := ui.Row(c.Key("trg:" + tr.Name)).FillWidth()
		hovered := row.Hovered()
		if i%2 == 1 {
			row.Background(p.surfaceSecondary)
		}
		row.Children(func() {
			structCell(c, p, stW0, func() { ui.Text(c, itoa(i+1)).TextColor(p.fgMuted).FontSize(font2XS) })
			structCell(c, p, stWName, func() { ui.Text(c, tr.Name).SingleLine() })
			structCell(c, p, stWNull+30, func() { ui.Text(c, tr.Timing).SingleLine().TextColor(p.fgSecondary) })
			structCell(c, p, stWNull+30, func() { ui.Text(c, tr.Event).SingleLine().TextColor(p.fgSecondary) })
			structCell(c, p, 0, func() {
				ui.Text(c, firstLines(tr.Definition, 1)).SingleLine().TextColor(p.fgSecondary).Tooltip(tr.Definition)
			})
			structCell(c, p, stWAct, func() {
				if hovered && !readOnly {
					if tipBtn(c.Key("trgdel"), icTrash, locales.CommonDelete()).Size(20, 20).Clicked() {
						defer t.dropTrigger(tr.Name)
					}
				}
			})
		})
	}
}

func (t *tableTab) dropForeignKey(name string) {
	stmt, err := database.BuildDropForeignKeySQL(t.w.cfg.Type, t.table, name)
	if err != nil {
		t.w.sh.toast(err.Error())
		return
	}
	t.w.sh.ask(locales.StructDropFKTitle(), locales.StructDropFKMsg(name), locales.CommonDelete(), func() { t.runObjectSQL(stmt, true) })
}

func (t *tableTab) dropTrigger(name string) {
	stmt := database.BuildDropTriggerSQL(t.w.cfg.Type, t.table, name)
	t.w.sh.ask(locales.StructDropTriggerTitle(), locales.StructDropTriggerMsg(name), locales.CommonDelete(), func() { t.runObjectSQL(stmt, true) })
}

// loadRefColumns reads the columns of the table a new foreign key points at.
func (t *tableTab) loadRefColumns() {
	f := &t.st.obj.newFK
	ref := f.refTable
	w, db := t.w, t.w.db
	var cols []database.ColumnInfo
	w.sh.up.bg(func() { cols, _ = w.sh.app.Databases.GetColumns(w.cfg.ID, db, ref) }, func() {
		if f.refTable != ref {
			return
		}
		f.refOptions = f.refOptions[:0]
		for _, c := range cols {
			f.refOptions = append(f.refOptions, c.Name)
		}
		f.refColumns = nil
	})
}

// objectDialogs draws the "add foreign key" and "add trigger" forms, each with
// the SQL it will send.
func (t *tableTab) objectDialogs(c *ui.Context) {
	s := &t.st
	sh := t.w.sh
	p := sh.tokens(c)
	preview := func(stmt string, err error) {
		text := stmt
		col := p.fg
		if err != nil {
			text, col = err.Error(), p.danger
		}
		dlgField(c, p, locales.StructSQLPreview(), func() {
			dlgBox(c, p).FillWidth().Padding(8, 10).Children(func() {
				ui.Text(c, text).Font(mono).FontSize(font2XS).TextColor(col).Selectable()
			})
		})
	}
	if f := &s.obj.newFK; f.open {
		var names, tables []string
		for _, col := range s.cols {
			names = append(names, col.Name)
		}
		for _, tb := range t.w.tables {
			if !isView(tb) {
				tables = append(tables, tb.Name)
			}
		}
		fk := t.fkFromForm()
		stmt, err := database.BuildAddForeignKeySQL(t.w.cfg.Type, t.table, fk)
		ready := f.refTable != "" && len(f.columns) > 0 && len(f.refColumns) > 0
		sh.dialog(c, &f.open, dlgOpts{title: locales.StructAddForeignKey(), sub: t.table, width: 520}, func() {
			dlgField(c, p, locales.StructFKName(), func() {
				ui.TextInput(c.Key("fkname"), &f.name).Placeholder(defaultFKName(t.table, f.columns)).AutoFocus().FillWidth().Height(sizeBtn)
			})
			dlgField(c, p, locales.StructIndexColumns(), func() { ui.TokenField(c.Key("fkcols"), &f.columns, names).FillWidth() })
			dlgField(c, p, locales.StructFKRefTable(), func() {
				if ui.Combobox(c.Key("fkref"), &f.refTable, tables).FillWidth().Height(sizeBtn).Changed() {
					t.loadRefColumns()
				}
			})
			dlgField(c, p, locales.StructFKRefColumns(), func() { ui.TokenField(c.Key("fkrefcols"), &f.refColumns, f.refOptions).FillWidth() })
			ui.Row(c).Gap(10).FillWidth().Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Text(c, locales.StructFKOnDelete()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
					ui.Select(c.Key("fkdelact"), &f.onDelete, database.FKActions).FillWidth().Height(sizeBtn)
				})
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Text(c, locales.StructFKOnUpdate()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
					ui.Select(c.Key("fkupdact"), &f.onUpdate, database.FKActions).FillWidth().Height(sizeBtn)
				})
			})
			if ready {
				preview(stmt, err)
			}
		}, func() {
			if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
				f.open = false
			}
			if ui.PrimaryButton(c, locales.StructCreate()).Height(sizeBtn).Disabled(!ready || err != nil).Clicked() {
				f.open = false
				t.runObjectSQL(stmt, false)
			}
		})
	}
	if f := &s.obj.newTrig; f.open {
		tr := t.triggerFromForm()
		stmt, err := database.BuildCreateTriggerSQL(t.w.cfg.Type, t.table, tr)
		ready := tr.Name != "" && tr.Definition != ""
		sh.dialog(c, &f.open, dlgOpts{title: locales.StructAddTrigger(), sub: t.table, width: 560}, func() {
			dlgField(c, p, locales.StructName(), func() {
				ui.TextInput(c.Key("trgname"), &f.name).AutoFocus().FillWidth().Height(sizeBtn)
			})
			ui.Row(c).Gap(10).FillWidth().Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Text(c, locales.StructTriggerTiming()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
					ui.Select(c.Key("trgtiming"), &f.timing, database.TriggerTimings).FillWidth().Height(sizeBtn)
				})
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Text(c, locales.StructTriggerEvent()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
					ui.Select(c.Key("trgevent"), &f.event, database.TriggerEvents).FillWidth().Height(sizeBtn)
				})
			})
			dlgField(c, p, locales.StructTriggerBody(), func() {
				ui.TextArea(c.Key("trgbody"), &f.body).Lines(4, 10).Font(mono).FontSize(font2XS + 1).FillWidth()
			})
			if ready {
				preview(stmt, err)
			}
		}, func() {
			if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
				f.open = false
			}
			if ui.PrimaryButton(c, locales.StructCreate()).Height(sizeBtn).Disabled(!ready || err != nil).Clicked() {
				f.open = false
				t.runObjectSQL(stmt, false)
			}
		})
	}
}
