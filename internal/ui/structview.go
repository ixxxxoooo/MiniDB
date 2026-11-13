package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// structState is the working copy of a table's columns and indexes while the
// user edits its structure. Original rows keep their UID (the original name),
// which is how the backend matches them; new rows get "new:N".
type structState struct {
	loaded  bool
	loading bool
	err     string

	orig    []database.StructureColumnEdit
	cols    []database.StructureColumnEdit
	deleted map[string]bool
	info    []database.ColumnInfo // read-only facts: primary key, auto increment

	origIdx []database.StructureIndexEdit
	idx     []database.StructureIndexEdit
	idxDel  map[string]bool

	types   []string
	sel     string // UID of the chosen column
	shownAt time.Time
	nextUID int

	obj objState // foreign keys and triggers

	newIndex struct {
		open    bool
		name    string
		columns []string
		unique  bool
	}
	preview struct {
		open  bool
		stmts []string
		err   string
		busy  bool
	}
}

func (s *structState) load(t *tableTab) {
	w := t.w
	db := w.db
	s.loading, s.err = true, ""
	var info []database.ColumnInfo
	var idx []database.IndexInfo
	var types []string
	var err error
	w.sh.up.bg(func() {
		info, err = w.sh.app.Databases.GetColumns(w.cfg.ID, db, t.table)
		if err == nil {
			idx, err = w.sh.app.Databases.GetIndexes(w.cfg.ID, db, t.table)
		}
		types = w.sh.app.Databases.GetColumnTypes(w.cfg.ID)
	}, func() {
		s.loading = false
		if err != nil {
			s.err = err.Error()
			return
		}
		s.loaded, s.info, s.types = true, info, types
		s.orig = s.orig[:0]
		for _, c := range info {
			s.orig = append(s.orig, database.StructureColumnEdit{
				UID: c.Name, Name: c.Name, Type: c.Type, Nullable: c.Nullable, DefaultValue: c.DefaultValue, Comment: c.Comment,
			})
		}
		s.origIdx = s.origIdx[:0]
		for _, ix := range idx {
			s.origIdx = append(s.origIdx, database.StructureIndexEdit{
				UID: ix.Name, Name: ix.Name, Type: ix.Type, IsUnique: ix.IsUnique, IsPrimary: ix.IsPrimary, Columns: ix.Columns,
			})
		}
		s.reset()
		s.loadObjects(t)
	})
}

func (s *structState) reset() {
	s.cols = append([]database.StructureColumnEdit(nil), s.orig...)
	for i := range s.cols {
		if s.cols[i].DefaultValue != nil {
			v := *s.cols[i].DefaultValue
			s.cols[i].DefaultValue = &v
		}
	}
	s.idx = append([]database.StructureIndexEdit(nil), s.origIdx...)
	s.deleted, s.idxDel = map[string]bool{}, map[string]bool{}
}

func (s *structState) isPK(name string) bool {
	for _, c := range s.info {
		if c.Name == name && c.IsPrimary {
			return true
		}
	}
	return false
}

func (s *structState) origCol(uid string) (database.StructureColumnEdit, bool) {
	for _, c := range s.orig {
		if c.UID == uid {
			return c, true
		}
	}
	return database.StructureColumnEdit{}, false
}

func sameDefault(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// working returns the columns with their Status derived from the originals.
func (s *structState) working() []database.StructureColumnEdit {
	out := make([]database.StructureColumnEdit, 0, len(s.cols))
	for _, c := range s.cols {
		o, existed := s.origCol(c.UID)
		switch {
		case !existed:
			c.Status = "new"
		case s.deleted[c.UID]:
			c.Status = "deleted"
		case c.Name != o.Name || c.Type != o.Type || c.Nullable != o.Nullable || c.Comment != o.Comment || !sameDefault(c.DefaultValue, o.DefaultValue):
			c.Status = "modified"
		default:
			c.Status = ""
		}
		out = append(out, c)
	}
	return out
}

func (s *structState) workingIndexes() []database.StructureIndexEdit {
	out := make([]database.StructureIndexEdit, 0, len(s.idx))
	for _, ix := range s.idx {
		switch {
		case !hasUID(s.origIdx, ix.UID):
			ix.Status = "new"
		case s.idxDel[ix.UID]:
			ix.Status = "deleted"
		default:
			ix.Status = ""
		}
		out = append(out, ix)
	}
	return out
}

func hasUID(list []database.StructureIndexEdit, uid string) bool {
	for _, x := range list {
		if x.UID == uid {
			return true
		}
	}
	return false
}

// changeCount counts edited columns and indexes.
func (s *structState) changeCount() int {
	n := 0
	for _, c := range s.working() {
		if c.Status != "" {
			n++
		}
	}
	for _, ix := range s.workingIndexes() {
		if ix.Status != "" {
			n++
		}
	}
	return n
}

func (t *tableTab) structureView(c *ui.Context) {
	th := c.Theme()
	s := &t.st
	if !s.loaded && !s.loading && s.err == "" {
		s.load(t)
	}
	switch {
	case s.err != "":
		ui.Text(c, locales.TableLoadFail(s.err)).TextColor(th.Danger).Padding(16).Selectable()
		return
	case !s.loaded:
		quietLoading(c, baseTokens(c), &s.shownAt)
		return
	}
	readOnly := t.w.cfg.ReadOnly
	p := baseTokens(c)
	// The grid's own theme: 12px mono text, as the data grid and TablePlus's
	// structure tab, with controls that fill their cell.
	gt := *th
	gt.Spacing, gt.FontSize, gt.Font = 2.5, font2XS, mono
	sc := scrollbars(c, ui.Scroll(c.Key("struct")).Grow(1).FillWidth()).Children(func() {
		ui.Themed(c, &gt).Background(ui.Transparent).FillWidth().Children(func() {
			t.structSection(c, p, locales.StructColumns(), len(s.cols), func() {
				if !readOnly {
					s.addColumn()
				}
			}, readOnly, locales.StructAddColumn())
			t.structColsHeader(c, p)
			for i := range s.cols {
				t.structColumnRow(c, p, i, readOnly)
			}
			ui.Box(c).Height(14).FillWidth()
			t.structSection(c, p, locales.StructIndexes(), len(s.idx), func() {
				s.newIndex.open, s.newIndex.name, s.newIndex.columns, s.newIndex.unique = true, "", nil, false
			}, readOnly, locales.StructAddIndex())
			t.structIdxHeader(c, p)
			for i := range s.idx {
				t.structIndexRow(c, p, i, readOnly)
			}
			t.structObjects(c, p, readOnly)
		})
	})
	// Backspace or Delete with the grid focused (not a cell's field) removes the chosen column
	if !readOnly && s.sel != "" && (sc.Shortcut(0, ui.KeyBackspace) || sc.Shortcut(0, ui.KeyDelete)) {
		for i, col := range s.cols {
			if col.UID == s.sel {
				t.removeColumn(i)
				break
			}
		}
	}
	t.structDialogs(c)
}

func itoa(n int) string { return strconv.Itoa(n) }

// Column widths of the structure grids.
const (
	stW0    = 40  // row number / key
	stWName = 190 // name
	stWType = 150 // type
	stWNull = 64  // nullable
	stWDef  = 140 // default
	stWAct  = 34  // delete
)

// structCell is one bordered cell of a structure grid: w wide, or sharing
// what is left when w is 0.
func structCell(c *ui.Context, p tokens, w float32, content func()) {
	e := ui.Row(c).Height(gridRowH).AlignItems(ui.Center).Padding(0, 8).Shrink(0).
		BorderWidth(0, 1, 1, 0).BorderColor(p.borderSubtle).Clip()
	if w > 0 {
		e.Width(w)
	} else {
		e.Grow(1).Shrink(1).MinWidth(120)
	}
	e.Children(content)
}

// structSection is the strip that heads a grid: title, count, add button.
func (t *tableTab) structSection(c *ui.Context, p tokens, title string, n int, add func(), readOnly bool, addTip string) {
	ui.Row(c.Key("sec-"+title)).Gap(6).AlignItems(ui.Center).Padding(0, 8, 0, 12).Height(26).FillWidth().Background(p.toolbar).Children(func() {
		ui.Text(c, title).FontWeight(600).FontSize(font2XS)
		ui.Text(c, "("+itoa(n)+")").FontSize(font2XS).TextColor(p.fgMuted)
		ui.Spacer(c)
		if tipBtn(c, icPlus, addTip).Size(sizeBtnSm, sizeBtnSm).Disabled(readOnly).Clicked() {
			add()
		}
	})
	hairline(c, p.border)
}

func structHead(c *ui.Context, p tokens, w float32, label string) {
	structCell(c, p, w, func() {
		ui.Text(c, label).FontWeight(600).SingleLine()
	})
}

func (t *tableTab) structColsHeader(c *ui.Context, p tokens) {
	ui.Row(c).FillWidth().Background(p.surfaceSecondary).Children(func() {
		structHead(c, p, stW0, "#")
		structHead(c, p, stWName, locales.StructName())
		structHead(c, p, stWType, locales.StructType())
		structHead(c, p, stWNull, locales.StructNullable())
		structHead(c, p, stWDef, locales.StructDefault())
		structHead(c, p, 0, locales.StructComment())
		structHead(c, p, stWAct, "")
	})
}

func (t *tableTab) structIdxHeader(c *ui.Context, p tokens) {
	ui.Row(c).FillWidth().Background(p.surfaceSecondary).Children(func() {
		structHead(c, p, stW0, "#")
		structHead(c, p, stWName, locales.StructIndexName())
		structHead(c, p, 0, locales.StructIndexColumns())
		structHead(c, p, stWType, locales.StructType())
		structHead(c, p, stWNull+20, locales.StructUnique())
		structHead(c, p, stWAct, "")
	})
}

// cellInput is a text field that looks like a grid cell until it is used.
func cellInput(c *ui.Context, key string, v *string, ro bool) ui.Element {
	return ui.TextInput(c.Key(key), v).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false).ReadOnly(ro)
}

func (t *tableTab) removeColumn(i int) {
	s := &t.st
	col := s.cols[i]
	_, existed := s.origCol(col.UID)
	switch {
	case !existed:
		s.cols = append(s.cols[:i], s.cols[i+1:]...)
	case s.isPK(col.UID):
		t.w.sh.toast(locales.StructPKLocked())
	default:
		s.deleted[col.UID] = !s.deleted[col.UID]
	}
}

func (t *tableTab) structColumnRow(c *ui.Context, p tokens, i int, readOnly bool) {
	th := c.Theme()
	s := &t.st
	col := &s.cols[i]
	_, existed := s.origCol(col.UID)
	deleted := s.deleted[col.UID]
	ro := readOnly || deleted
	row := ui.Row(c.Key(col.UID)).FillWidth()
	hovered := row.Hovered()
	switch {
	case deleted:
		row.Background(p.danger.Alpha(0.12))
	case !existed:
		row.Background(p.success.Alpha(0.14))
	case s.sel == col.UID:
		row.Background(p.accent.Alpha(0.10))
	case i%2 == 1:
		row.Background(p.surfaceSecondary)
	}
	row.Children(func() {
		structCell(c, p, stW0, func() {
			if s.isPK(col.UID) && existed {
				ui.Icon(c, icKey).FontSize(11).TextColor(p.warning).Tooltip(locales.StructPrimary())
			} else {
				ui.Text(c, itoa(i+1)).TextColor(p.fgMuted).FontSize(font2XS)
			}
		})
		structCell(c, p, stWName, func() { cellInput(c, "n", &col.Name, ro) })
		structCell(c, p, stWType, func() {
			ui.Combobox(c.Key("t"), &col.Type, s.types).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).Disabled(ro)
		})
		structCell(c, p, stWNull, func() {
			ui.Checkbox(c.Key("nl"), &col.Nullable, "").Disabled(ro)
		})
		structCell(c, p, stWDef, func() {
			def := ""
			if col.DefaultValue != nil {
				def = *col.DefaultValue
			}
			in := cellInput(c, "d", &def, ro).Placeholder("NULL")
			if in.Changed() {
				if def == "" {
					col.DefaultValue = nil
				} else {
					col.DefaultValue = &def
				}
			}
		})
		structCell(c, p, 0, func() { cellInput(c, "c", &col.Comment, ro) })
		structCell(c, p, stWAct, func() {
			if (hovered || s.sel == col.UID) && !readOnly {
				if tipBtn(c.Key("del"), icTrash, locales.CommonDelete()).Size(20, 20).Clicked() {
					defer t.removeColumn(i)
				}
			}
		})
	})
	if row.Clicked() {
		s.sel = col.UID
	}
	_ = th
}

func (t *tableTab) structIndexRow(c *ui.Context, p tokens, i int, readOnly bool) {
	s := &t.st
	ix := s.idx[i]
	existed := hasUID(s.origIdx, ix.UID)
	row := ui.Row(c.Key("ix:" + ix.UID)).FillWidth()
	hovered := row.Hovered()
	switch {
	case s.idxDel[ix.UID]:
		row.Background(p.danger.Alpha(0.12))
	case !existed:
		row.Background(p.success.Alpha(0.14))
	case i%2 == 1:
		row.Background(p.surfaceSecondary)
	}
	row.Children(func() {
		structCell(c, p, stW0, func() {
			if ix.IsPrimary {
				ui.Icon(c, icKey).FontSize(11).TextColor(p.warning)
			} else {
				ui.Text(c, itoa(i+1)).TextColor(p.fgMuted).FontSize(font2XS)
			}
		})
		structCell(c, p, stWName, func() { ui.Text(c, ix.Name).SingleLine() })
		structCell(c, p, 0, func() { ui.Text(c, strings.Join(ix.Columns, ", ")).SingleLine() })
		structCell(c, p, stWType, func() { ui.Text(c, ix.Type).SingleLine().TextColor(p.fgSecondary) })
		structCell(c, p, stWNull+20, func() {
			txt := "NO"
			if ix.IsUnique || ix.IsPrimary {
				txt = "YES"
			}
			ui.Text(c, txt).TextColor(p.fgSecondary)
		})
		structCell(c, p, stWAct, func() {
			if hovered && !readOnly && !ix.IsPrimary {
				if tipBtn(c.Key("idel"), icTrash, locales.CommonDelete()).Size(20, 20).Clicked() {
					defer func() {
						if existed {
							s.idxDel[ix.UID] = !s.idxDel[ix.UID]
						} else {
							s.idx = append(s.idx[:i], s.idx[i+1:]...)
						}
					}()
				}
			}
		})
	})
}

// buildStructurePreview fills in the DDL a commit would run (for tests and logs).
func (t *tableTab) buildStructurePreview() {
	s := &t.st
	var stmts []string
	var err error
	w := t.w
	w.sh.up.bg(func() {
		stmts, err = w.sh.app.Databases.PreviewTableStructureChanges(w.cfg.ID, t.table, s.working(), s.orig, s.workingIndexes())
	}, func() {
		s.preview.stmts, s.preview.err = stmts, ""
		if err != nil {
			s.preview.err = err.Error()
		}
	})
}

func (t *tableTab) applyStructure() {
	s := &t.st
	w := t.w
	db := w.db
	s.preview.busy, s.preview.err = true, ""
	working, idx := s.working(), s.workingIndexes()
	stmts := append([]string(nil), s.preview.stmts...)
	started := time.Now()
	var err error
	w.sh.up.bg(func() {
		err = w.sh.app.Databases.ApplyTableStructureChanges(w.cfg.ID, db, t.table, working, s.orig, idx)
	}, func() {
		s.preview.busy = false
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		w.sh.logCommit(db, stmts, 0, time.Since(started), errText)
		if err != nil {
			s.preview.err = locales.StructApplyFail(err.Error())
			w.sh.toast(s.preview.err)
			return
		}
		w.sh.toast(locales.StructApplied())
		t.ddl = ddlState{}
		t.colSigReset()
		s.load(t)
		t.loadStructure()
		t.load()
		w.loadTables()
	})
}

// commitStructure applies the structure changes at once, without a dialog;
// the connection's Safe Mode may ask first.
func (t *tableTab) commitStructure() {
	s := &t.st
	if s.preview.busy || s.changeCount() == 0 || t.w.cfg.ReadOnly {
		return
	}
	stmts := s.preview.stmts
	if len(stmts) == 0 {
		stmts = []string{"ALTER TABLE " + t.table}
	}
	t.w.sh.safeGate(t.w, stmts, t.applyStructure)
}

// colSigReset forces the data grid to rebuild its columns after a structure change.
func (t *tableTab) colSigReset() { t.g.sig = "" }

func (t *tableTab) structDialogs(c *ui.Context) {
	t.objectDialogs(c)
	s := &t.st
	sh := t.w.sh
	p := sh.tokens(c)
	if s.newIndex.open {
		var names []string
		for _, col := range s.cols {
			names = append(names, col.Name)
		}
		sh.dialog(c, &s.newIndex.open, dlgOpts{title: locales.StructAddIndex(), sub: t.table, width: 460}, func() {
			dlgField(c, p, locales.StructIndexName(), func() {
				ui.TextInput(c.Key("ixname"), &s.newIndex.name).AutoFocus().FillWidth().Height(sizeBtn)
			})
			dlgField(c, p, locales.StructIndexColumns(), func() {
				ui.TokenField(c, &s.newIndex.columns, names).FillWidth()
			})
			ui.Checkbox(c, &s.newIndex.unique, locales.StructUnique())
		}, func() {
			if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
				s.newIndex.open = false
			}
			ok := strings.TrimSpace(s.newIndex.name) != "" && len(s.newIndex.columns) > 0
			if ui.PrimaryButton(c, locales.CommonConfirm()).Height(sizeBtn).Disabled(!ok).Clicked() {
				s.nextUID++
				s.idx = append(s.idx, database.StructureIndexEdit{
					UID: "new:" + itoa(s.nextUID), Name: strings.TrimSpace(s.newIndex.name),
					Columns: append([]string(nil), s.newIndex.columns...), IsUnique: s.newIndex.unique,
				})
				s.newIndex.open = false
			}
		})
	}
}

func (s *structState) addColumn() {
	s.nextUID++
	s.cols = append(s.cols, database.StructureColumnEdit{UID: "new:" + itoa(s.nextUID), Name: "column_" + itoa(s.nextUID), Type: "varchar(255)", Nullable: true})
}
