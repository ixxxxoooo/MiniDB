package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/dataimport"
	"minidb/locales"
)

func (s *Shell) importDialogView(c *ui.Context) {
	d := &s.imp
	if !d.open {
		return
	}
	p := s.tokens(c)
	title := locales.ImportTitleGeneric()
	if d.table != "" && !d.newTable {
		title = locales.ImportTitle(d.table)
	}
	s.dialog(c, &d.open, dlgOpts{title: title, width: 680, height: 620}, func() {
		scrollbars(c, ui.Scroll(c.Key("imp-body")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(12).FillWidth().Children(func() {
				s.importFileRow(c, p)
				if d.err != "" {
					ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
				}
				if d.sql != nil {
					s.importSQLView(c, p)
				} else if d.preview != nil {
					s.importDataView(c, p)
				}
			})
		})
	}, func() {
		if d.busy {
			ui.Spinner(c)
		}
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		label, ready := locales.ImportStart(), d.preview != nil
		if d.sql != nil {
			label, ready = locales.ImportSQLRun(), true
		}
		if footPrimary(c, label).Disabled(!ready || d.busy).Clicked() {
			s.confirmImport()
		}
	})
}

func (s *Shell) importFileRow(c *ui.Context, p tokens) {
	d := &s.imp
	dlgField(c, p, locales.ImportFile(), func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, d.path).SingleLine().TruncateMiddle().Grow(1).TextColor(p.fgSecondary).Padding(0, 8).Height(sizeBtn).Background(p.surfaceSecondary).Radius(radiusBtn)
			if fieldBtn(c, icFolder, locales.ImportChoose()).Clicked() {
				s.pickImportFile()
			}
		})
	})
}

func (s *Shell) importSQLView(c *ui.Context, p tokens) {
	pv := s.imp.sql
	ui.Text(c, locales.ImportSQLCount(pv.Count)).FontWeight(500)
	dlgBox(c, p).FillWidth().Children(func() {
		ui.Column(c).Padding(8).Gap(4).FillWidth().Children(func() {
			for i, st := range pv.Head {
				ui.Text(c.Key(fmt.Sprint(i)), oneLine(st)).SingleLine().FontSize(font2XS).Font(mono).TextColor(p.fgSecondary)
			}
		})
	})
	if len(pv.Risky) > 0 {
		ui.Text(c, locales.ImportSQLRisky(len(pv.Risky))).TextColor(p.danger).FontSize(font2XS)
	}
	ui.Text(c, locales.ImportSQLNote()).TextColor(p.fgMuted).FontSize(font2XS)
}

func (s *Shell) importDataView(c *ui.Context, p tokens) {
	d := &s.imp
	// 解析方式
	ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
		if ui.Checkbox(c, &d.opts.HasHeader, locales.ImportHasHeader()).Changed() {
			s.refreshImportPreview()
		}
		ui.Text(c, locales.ImportDelimiter()).TextColor(p.fgSecondary).FontSize(font2XS)
		if ui.Select(c, &d.delimiter, delimiterChoices).MinWidth(0).Width(80).Height(sizeBtnSm).Changed() {
			s.refreshImportPreview()
		}
		ui.Text(c, locales.ImportEncoding()).TextColor(p.fgSecondary).FontSize(font2XS)
		if ui.Select(c, &d.encoding, dataimport.Encodings).MinWidth(0).Width(110).Height(sizeBtnSm).Changed() {
			s.refreshImportPreview()
		}
		ui.Text(c, locales.ImportQuote()).TextColor(p.fgSecondary).FontSize(font2XS)
		if labelSelect(c, &d.quote, importQuotes, []string{`"`, `'`, locales.ExportQuoteNone()}, 90) {
			s.refreshImportPreview()
		}
		ui.Spacer(c)
		ui.Text(c, locales.ImportRows(int(d.preview.TotalRows))).TextColor(p.fgMuted).FontSize(font2XS)
	})

	// 预览
	ui.Text(c, locales.ImportPreview(len(d.preview.Rows))).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
	dlgBox(c, p).FillWidth().Children(func() {
		scrollbars(c, ui.ScrollBoth(c.Key("imp-pv")).Height(150).FillWidth()).Children(func() {
			ui.Column(c).Padding(6).Gap(2).Children(func() {
				ui.Row(c.Key("h")).Gap(8).Children(func() {
					for i, name := range d.preview.Columns {
						ui.Text(c.Key(fmt.Sprint(i)), name).Width(120).SingleLine().FontSize(font2XS).FontWeight(600)
					}
				})
				for r, row := range d.preview.Rows {
					ui.Row(c.Key("r" + fmt.Sprint(r))).Gap(8).Children(func() {
						for i, cell := range row {
							ui.Text(c.Key(fmt.Sprint(i)), oneLine(cell)).Width(120).SingleLine().FontSize(font2XS).TextColor(p.fgSecondary)
						}
					})
				}
			})
		})
	})

	// 导入到哪
	seg := 0
	if d.newTable {
		seg = 1
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.ImportTarget()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
		if ui.Segmented(c, &seg, locales.ImportTargetExisting(), locales.ImportTargetNew()).Changed() {
			d.newTable = seg == 1
		}
		if d.newTable {
			ui.TextInput(c.Key("imp-new"), &d.newName).Placeholder(locales.ImportNewName()).Grow(1).Height(sizeBtnSm)
		} else {
			var names []string
			for _, t := range d.w.tables {
				names = append(names, t.Name)
			}
			if ui.Combobox(c, &d.table, names).Grow(1).Height(sizeBtnSm).Changed() {
				s.loadImportCols(d.table)
			}
		}
	})

	// 列映射
	ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.ImportMapping()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
		ui.Spacer(c)
		if !d.newTable {
			ui.Text(c, locales.ImportMapBy()).FontSize(font2XS).TextColor(p.fgSecondary)
			if labelSelect(c, &d.mapBy, []string{"name", "order"}, []string{locales.ImportMapByName(), locales.ImportMapByOrder()}, 90) {
				s.autoMapImport()
			}
		}
	})
	dlgBox(c, p).FillWidth().Children(func() {
		scrollbars(c, ui.Scroll(c.Key("map")).Height(200).FillWidth()).Children(func() {
			ui.Column(c).Padding(8).Gap(4).FillWidth().Children(func() {
				opts := append([]string{locales.ImportSkip()}, d.tableCols...)
				for i, name := range d.preview.Columns {
					i := i
					sample := ""
					if len(d.preview.Rows) > 0 && i < len(d.preview.Rows[0]) {
						sample = d.preview.Rows[0][i]
					}
					ui.Row(c.Key(fmt.Sprint(i))).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
						ui.Text(c, name).Width(150).SingleLine().FontWeight(500)
						ui.Text(c, oneLine(sample)).Width(130).SingleLine().TextColor(p.fgMuted).FontSize(font2XS)
						if d.newTable {
							if i < len(d.newCols) {
								ui.TextInput(c.Key("nc"+fmt.Sprint(i)), &d.newCols[i].name).Placeholder(locales.ImportSkip()).Grow(1).Height(sizeBtnSm)
								labelSelect(c, &d.newCols[i].kind, importKinds, importKindLabels(), 90)
							}
							return
						}
						cur := d.target[i]
						if cur == "" {
							cur = locales.ImportSkip()
						}
						if ui.Select(c, &cur, opts).Grow(1).Height(sizeBtnSm).Changed() {
							if cur == locales.ImportSkip() {
								cur = ""
							}
							d.target[i] = cur
						}
					})
				}
			})
		})
	})

	if !d.newTable {
		ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, locales.ImportConflict()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
			labelSelect(c, &d.conflict, importConflictValues(), importConflictLabels(), 200)
		})
	}
	ui.Checkbox(c, &d.atomic, locales.ImportAtomic())
	ui.Checkbox(c, &d.emptyAsNull, locales.ImportEmptyAsNull())
}

// confirmImport 过安全模式的关口，再开始。
func (s *Shell) confirmImport() {
	d := &s.imp
	if d.sql != nil {
		stmts := append(append([]string{}, d.sql.Head...), d.sql.Risky...)
		s.safeGate(d.w, stmts, s.startSQLImport)
		return
	}
	table := d.table
	if d.newTable {
		table = strings.TrimSpace(d.newName)
	}
	if table == "" {
		d.err = locales.ImportNeedTable()
		return
	}
	stmts := []string{"INSERT INTO " + table}
	switch {
	case d.newTable:
		stmts = []string{"CREATE TABLE " + table}
	case d.conflict == database.ConflictReplace:
		stmts = []string{"REPLACE INTO " + table}
	case d.conflict == database.ConflictUpdate:
		stmts = []string{"UPDATE " + table}
	}
	s.safeGate(d.w, stmts, s.startImport)
}

func (s *Shell) startSQLImport() {
	d := &s.imp
	req := app.SQLImportRequest{ConnID: d.w.cfg.ID, Database: d.w.db, FilePath: d.path}
	d.busy = true
	var err error
	s.up.bg(func() { _, err = s.app.Imports.StartSQLImport(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.open = false
		s.toastOK(locales.ImportStarted())
	})
}

func (s *Shell) startImport() {
	d := &s.imp
	req := app.ImportRequest{
		ConnID: d.w.cfg.ID, Database: d.w.db, Table: d.table, FilePath: d.path, Parse: d.opts,
		Import: database.ImportOptions{Atomic: d.atomic, EmptyAsNull: d.emptyAsNull, Conflict: d.conflict},
	}
	if d.newTable {
		req.Table = strings.TrimSpace(d.newName)
		req.Import.Conflict = database.ConflictStop
		for i, nc := range d.newCols {
			if name := strings.TrimSpace(nc.name); name != "" {
				req.NewColumns = append(req.NewColumns, app.ImportNewColumn{Source: i, Name: name, Kind: nc.kind})
			}
		}
		if len(req.NewColumns) == 0 {
			d.err = locales.ImportNeedMap()
			return
		}
	} else {
		for i, tgt := range d.target {
			if tgt != "" {
				req.Columns = append(req.Columns, database.ImportColumn{Source: i, Target: tgt})
			}
		}
		if len(req.Columns) == 0 {
			d.err = locales.ImportNeedMap()
			return
		}
	}
	d.busy = true
	var err error
	s.up.bg(func() { _, err = s.app.Imports.StartImport(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.open = false
		s.toastOK(locales.ImportStarted())
	})
}
