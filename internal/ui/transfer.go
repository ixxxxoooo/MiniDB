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

var exportFormats = []string{"csv", "json", "sql"}

// exportDialog picks the format of a table or query-result export.
type exportDialog struct {
	open   bool
	w      *Workspace
	table  string
	query  string // when set, exports this statement's result instead of a table
	format int
}

func (s *Shell) exportTable(w *Workspace, table string) {
	s.exp = exportDialog{open: true, w: w, table: table}
}

func (s *Shell) exportQuery(w *Workspace, sql string) {
	s.exp = exportDialog{open: true, w: w, table: "query", query: sql}
}

func (s *Shell) exportDialogView(c *ui.Context) {
	d := &s.exp
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.TableExport(), sub: d.table, width: 400}, func() {
		dlgField(c, p, locales.ExportFormat(), func() {
			ui.Segmented(c, &d.format, "CSV", "JSON", "SQL")
		})
	}, func() {
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			d.open = false
		}
		if ui.PrimaryButton(c, locales.ExportStart()).Height(sizeBtn).Clicked() {
			d.open = false
			s.startExport(*d)
		}
	})
}

func (s *Shell) startExport(d exportDialog) {
	w := d.w
	format, db := exportFormats[d.format], w.db
	var err error
	s.up.bg(func() { // the save dialog blocks, so this runs off the UI thread
		if d.query != "" {
			_, err = s.app.Exports.ExportSQLResultStream(w.cfg.ID, db, d.query, format)
		} else {
			_, err = s.app.Exports.ExportTableStream(w.cfg.ID, db, d.table, format)
		}
	}, func() {
		if err != nil {
			s.toast(err.Error())
		}
	})
}

// importDialog maps a data file's columns onto a table's and starts the import.
type importDialog struct {
	open        bool
	w           *Workspace
	table       string
	path        string
	opts        dataimport.Options
	delimiter   string
	preview     *dataimport.Preview
	err         string
	target      []string // per source column: table column or ""
	tableCols   []string
	atomic      bool
	emptyAsNull bool
	busy        bool
}

var delimiterChoices = []string{"auto", ",", ";", "tab", "|"}

func (s *Shell) importInto(w *Workspace, table string) {
	d := importDialog{open: true, w: w, table: table, atomic: true, emptyAsNull: true, delimiter: "auto"}
	d.opts.HasHeader = true
	s.imp = d
	var cols []database.ColumnInfo
	s.up.bg(func() { cols, _ = s.app.Databases.GetColumns(w.cfg.ID, w.db, table) }, func() {
		for _, c := range cols {
			s.imp.tableCols = append(s.imp.tableCols, c.Name)
		}
	})
}

func (s *Shell) pickImportFile() {
	d := &s.imp
	var path string
	var err error
	s.up.bg(func() { path, err = s.app.Imports.PickImportFile() }, func() {
		if err != nil {
			d.err = err.Error()
			return
		}
		if path != "" {
			d.path = path
			s.refreshImportPreview()
		}
	})
}

func (s *Shell) refreshImportPreview() {
	d := &s.imp
	if d.path == "" {
		return
	}
	opts := d.opts
	switch d.delimiter {
	case "auto":
		opts.Delimiter = ""
	case "tab":
		opts.Delimiter = "\t"
	default:
		opts.Delimiter = d.delimiter
	}
	d.opts = opts
	var pv *dataimport.Preview
	var err error
	s.up.bg(func() { pv, err = s.app.Imports.PreviewImportFile(d.path, opts) }, func() {
		if err != nil {
			d.err, d.preview = err.Error(), nil
			return
		}
		d.err, d.preview = "", pv
		d.target = make([]string, len(pv.Columns))
		for i, name := range pv.Columns {
			for _, tc := range d.tableCols {
				if strings.EqualFold(tc, name) {
					d.target[i] = tc
				}
			}
		}
	})
}

func (s *Shell) importDialogView(c *ui.Context) {
	d := &s.imp
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.ImportTitle(d.table), width: 640}, func() {
		dlgField(c, p, locales.ImportFile(), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, d.path).SingleLine().TruncateMiddle().Grow(1).TextColor(p.fgSecondary).Padding(0, 8).Height(sizeBtn).Background(p.surfaceSecondary).Radius(radiusBtn)
				if ui.Button(c, locales.ImportChoose()).Height(sizeBtn).Clicked() {
					s.pickImportFile()
				}
			})
		})
		if d.err != "" {
			ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
		}
		if d.preview != nil {
			ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
				if ui.Checkbox(c, &d.opts.HasHeader, locales.ImportHasHeader()).Changed() {
					s.refreshImportPreview()
				}
				ui.Text(c, locales.ImportDelimiter()).TextColor(p.fgSecondary).FontSize(font2XS)
				if ui.Select(c, &d.delimiter, delimiterChoices).MinWidth(0).Width(90).Height(sizeBtnSm).Changed() {
					s.refreshImportPreview()
				}
				ui.Spacer(c)
				ui.Text(c, locales.ImportRows(int(d.preview.TotalRows))).TextColor(p.fgMuted).FontSize(font2XS)
			})
			ui.Text(c, locales.ImportMapping()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
			dlgBox(c, p).FillWidth().Children(func() {
				ui.Scroll(c.Key("map")).Height(220).FillWidth().Children(func() {
					ui.Column(c).Padding(8).Gap(4).FillWidth().Children(func() {
						opts := append([]string{locales.ImportSkip()}, d.tableCols...)
						for i, name := range d.preview.Columns {
							i := i
							sample := ""
							if len(d.preview.Rows) > 0 && i < len(d.preview.Rows[0]) {
								sample = d.preview.Rows[0][i]
							}
							ui.Row(c.Key(fmt.Sprint(i))).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
								ui.Text(c, name).Width(170).SingleLine().FontWeight(500)
								ui.Text(c, oneLine(sample)).Width(150).SingleLine().TextColor(p.fgMuted).FontSize(font2XS)
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
			ui.Checkbox(c, &d.atomic, locales.ImportAtomic())
			ui.Checkbox(c, &d.emptyAsNull, locales.ImportEmptyAsNull())
		}
	}, func() {
		if d.busy {
			ui.Spinner(c)
		}
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			d.open = false
		}
		if ui.PrimaryButton(c, locales.ImportStart()).Height(sizeBtn).Disabled(d.preview == nil || d.busy).Clicked() {
			s.safeGate(d.w, []string{"INSERT INTO " + d.table}, s.startImport)
		}
	})
}

func (s *Shell) startImport() {
	d := &s.imp
	var cols []database.ImportColumn
	for i, tgt := range d.target {
		if tgt != "" {
			cols = append(cols, database.ImportColumn{Source: i, Target: tgt})
		}
	}
	if len(cols) == 0 {
		d.err = locales.ImportNeedMap()
		return
	}
	req := app.ImportRequest{
		ConnID: d.w.cfg.ID, Database: d.w.db, Table: d.table, FilePath: d.path, Parse: d.opts, Columns: cols,
		Import: database.ImportOptions{Atomic: d.atomic, EmptyAsNull: d.emptyAsNull},
	}
	d.busy = true
	var err error
	s.up.bg(func() { _, err = s.app.Imports.StartImport(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = err.Error()
			return
		}
		d.open = false
		s.toast(locales.ImportStarted())
	})
}
