package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/dataimport"
	"minidb/internal/export"
	"minidb/internal/model"
	"minidb/locales"
)

var exportFormats = []string{"csv", "json", "sql", "xlsx"}

var (
	exportDelims = []string{",", ";", "tab", "|"}
)

// exportPrefs 记住上次的导出选项。
type exportPrefs struct {
	Format       int    `json:"format,omitempty"`
	Gzip         bool   `json:"gzip,omitempty"`
	Delim        string `json:"delim,omitempty"`
	Quote        string `json:"quote,omitempty"`
	CRLF         bool   `json:"crlf,omitempty"`
	DecimalComma bool   `json:"decimalComma,omitempty"`
	NullKeep     bool   `json:"nullKeep,omitempty"` // NULL 写成 NULL 而不是空
	NoHeader     bool   `json:"noHeader,omitempty"`
	NoBOM        bool   `json:"noBom,omitempty"`
	SQLStructure bool   `json:"sqlStructure,omitempty"`
	SQLDrop      bool   `json:"sqlDrop,omitempty"`
	SQLNoData    bool   `json:"sqlNoData,omitempty"`
	SQLBatch     int    `json:"sqlBatch,omitempty"`  // 每条 INSERT 合并的行数，0/1 为一行一条
	JSONLines    bool   `json:"jsonLines,omitempty"` // JSON Lines
	Pattern      string `json:"pattern,omitempty"`
}

// exportDialog picks what and how to export: tables or a query result.
type exportDialog struct {
	open    bool
	w       *Workspace
	table   string
	query   string // when set, exports this statement's result instead of tables
	format  int
	sel     map[string]bool // 选中的表
	colsFor string          // cols 属于哪张表
	cols    []string
	colSel  map[string]bool
	prefs   exportPrefs
	search  string // 表清单的筛选词
	limit   string // 行数上限，空为全部
}

// limitRows 是行数上限；空、非数字或非正数都按"全部"。
func (d *exportDialog) limitRows() int64 {
	var n int64
	if _, err := fmt.Sscan(strings.TrimSpace(d.limit), &n); err != nil || n <= 0 {
		return 0
	}
	return n
}

func (s *Shell) exportTable(w *Workspace, table string) {
	pf := s.prefs.Export
	s.exp = exportDialog{open: true, w: w, table: table, sel: map[string]bool{table: true}, prefs: pf, format: pf.Format}
}

// exportTables 打开导出对话框，不预选表，由用户勾选多张。
func (s *Shell) exportTables(w *Workspace) {
	pf := s.prefs.Export
	s.exp = exportDialog{open: true, w: w, sel: map[string]bool{}, prefs: pf, format: pf.Format}
}

func (s *Shell) exportQuery(w *Workspace, sql string) {
	pf := s.prefs.Export
	s.exp = exportDialog{open: true, w: w, table: "query", query: sql, prefs: pf, format: pf.Format}
}

// chosenTables 按侧栏顺序返回选中的表。
func (d *exportDialog) chosenTables() []string {
	var out []string
	for _, t := range d.w.tables {
		if d.sel[t.Name] {
			out = append(out, t.Name)
		}
	}
	return out
}

// syncExportCols 在只选一张表时读出它的列，供挑选。
func (s *Shell) syncExportCols() {
	d := &s.exp
	if d.query != "" {
		return
	}
	chosen := d.chosenTables()
	if len(chosen) != 1 {
		d.colsFor, d.cols, d.colSel = "", nil, nil
		return
	}
	name := chosen[0]
	if d.colsFor == name {
		return
	}
	d.colsFor, d.cols, d.colSel = name, nil, map[string]bool{}
	w := d.w
	var cols []model.ColumnInfo
	s.up.bg(func() { cols, _ = s.app.Databases.GetColumns(w.cfg.ID, w.db, name) }, func() {
		if d.colsFor != name {
			return
		}
		for _, c := range cols {
			d.cols = append(d.cols, c.Name)
			d.colSel[c.Name] = true
		}
	})
}

// csvOptions 是 CSV 的格式选项；导出对话框和设置页共用。
func csvOptions(c *ui.Context, p tokens, pf *exportPrefs) {
	if pf.Delim == "" {
		pf.Delim = ","
	}
	if pf.Quote == "" {
		pf.Quote = `"`
	}
	ui.Row(c).Gap(12).FillWidth().Children(func() {
		dlgField(c, p, locales.ExportDelim(), func() {
			ui.Select(c, &pf.Delim, exportDelims).Width(120).Height(sizeBtn)
		})
		dlgField(c, p, locales.ExportQuote(), func() {
			label := pf.Quote
			if label == "none" {
				label = locales.ExportQuoteNone()
			}
			opts := []string{`"`, `'`, locales.ExportQuoteNone()}
			if ui.Select(c, &label, opts).Width(140).Height(sizeBtn).Changed() {
				pf.Quote = label
				if label == locales.ExportQuoteNone() {
					pf.Quote = "none"
				}
			}
		})
	})
	ui.Column(c).Gap(6).FillWidth().Children(func() {
		head := !pf.NoHeader
		if ui.Checkbox(c, &head, locales.ExportHeader()).Changed() {
			pf.NoHeader = !head
		}
		bom := !pf.NoBOM
		if ui.Checkbox(c, &bom, locales.ExportBOM()).Changed() {
			pf.NoBOM = !bom
		}
		ui.Checkbox(c, &pf.CRLF, locales.ExportCRLF())
		ui.Checkbox(c, &pf.DecimalComma, locales.ExportDecimalComma())
		nullEmpty := !pf.NullKeep
		if ui.Checkbox(c, &nullEmpty, locales.ExportNullEmpty()).Changed() {
			pf.NullKeep = !nullEmpty
		}
	})
}

// exportOptions 把对话框的选择变成导出选项。
func (d exportDialog) exportOptions() export.Options {
	pf := d.prefs
	o := export.Options{Format: exportFormats[d.format], Gzip: pf.Gzip && d.format != 3}
	o.CSV = export.CSVOptions{
		Delimiter: pf.Delim, Quote: pf.Quote, CRLF: pf.CRLF, DecimalComma: pf.DecimalComma,
		NoHeader: pf.NoHeader, NoBOM: pf.NoBOM,
	}
	if pf.NullKeep {
		o.CSV.NullText = "NULL"
	}
	if o.CSV.Quote == `"` {
		o.CSV.Quote = ""
	}
	if o.CSV.Delimiter == "," {
		o.CSV.Delimiter = ""
	}
	o.SQL = export.SQLOptions{Structure: pf.SQLStructure, DropTable: pf.SQLDrop, NoData: pf.SQLNoData, BatchSize: pf.SQLBatch}
	o.JSON = export.JSONOptions{Lines: pf.JSONLines}
	// 只选一张表时，没勾满的列才需要限定
	if len(d.cols) > 0 {
		var picked []string
		for _, n := range d.cols {
			if d.colSel[n] {
				picked = append(picked, n)
			}
		}
		if len(picked) > 0 && len(picked) < len(d.cols) {
			o.Columns = picked
		}
	}
	return o
}

func (s *Shell) startExport(d exportDialog) {
	w := d.w
	d.prefs.Format = d.format
	s.prefs.Export = d.prefs
	s.savePrefs()
	req := app.ExportRequest{ConnID: w.cfg.ID, Database: w.db, Query: d.query, Options: d.exportOptions(), Pattern: d.prefs.Pattern, Limit: d.limitRows()}
	if d.query == "" {
		req.Tables = d.chosenTables()
	}
	var err error
	s.up.bg(func() { // the save dialog blocks, so this runs off the UI thread
		_, err = s.app.Exports.StartExport(req)
	}, func() {
		if err != nil {
			s.toastErr(err.Error())
		}
	})
}

// importDialog 选文件、设解析方式、把文件的列对到表的列（或新建一张表），然后开始导入。
type importDialog struct {
	open  bool
	w     *Workspace
	table string // 导入到的现有表
	path  string

	newTable bool   // 导入到新建的表
	newName  string // 新表名
	newCols  []importNewCol

	opts      dataimport.Options
	delimiter string
	encoding  string
	quote     string // "\"" "'" "none"
	preview   *dataimport.Preview
	sql       *app.SQLFilePreview // 选了 .sql 文件时
	err       string

	target    []string // 每个源列对应的表列，空为跳过
	tableCols []string
	mapBy     string // "name" | "order"
	conflict  string // database.Conflict*

	atomic      bool
	emptyAsNull bool
	busy        bool
}

// importNewCol 是新表的一列。
type importNewCol struct {
	name string // 空表示不导入这一列
	kind string // int float text
}

var (
	delimiterChoices = []string{"auto", ",", ";", "tab", "|"}
	importQuotes     = []string{`"`, `'`, "none"}
	importKinds      = []string{"int", "float", "text"}
)

func importKindLabels() []string {
	return []string{locales.ImportKindInt(), locales.ImportKindFloat(), locales.ImportKindText()}
}

func importConflictValues() []string {
	return []string{database.ConflictStop, database.ConflictIgnore, database.ConflictReplace, database.ConflictUpdate}
}

func importConflictLabels() []string {
	return []string{locales.ImportConflictStop(), locales.ImportConflictIgnore(), locales.ImportConflictReplace(), locales.ImportConflictUpdate()}
}

// labelSelect 是值和显示名分开的下拉框；值变了返回 true。
func labelSelect(c *ui.Context, value *string, values, labels []string, width float32) bool {
	cur := *value
	for i, v := range values {
		if v == *value {
			cur = labels[i]
		}
	}
	if !ui.Select(c, &cur, labels).MinWidth(0).Width(width).Height(sizeBtnSm).Changed() {
		return false
	}
	for i, l := range labels {
		if l == cur {
			*value = values[i]
		}
	}
	return true
}

func newImportDialog(w *Workspace, table string) importDialog {
	d := importDialog{open: true, w: w, table: table, atomic: true, emptyAsNull: true, delimiter: "auto", mapBy: "name", encoding: "UTF-8", quote: `"`, newTable: table == ""}
	d.opts.HasHeader = true
	return d
}

func (s *Shell) importInto(w *Workspace, table string) {
	s.imp = newImportDialog(w, table)
	s.loadImportCols(table)
}

// importFile 打开导入对话框，不指定表：可以导入到某张现有表，也可以新建。
func (s *Shell) importFile(w *Workspace) {
	s.imp = newImportDialog(w, "")
}

func (s *Shell) loadImportCols(table string) {
	d := &s.imp
	d.tableCols = nil
	if table == "" {
		return
	}
	w := d.w
	var cols []model.ColumnInfo
	s.up.bg(func() { cols, _ = s.app.Databases.GetColumns(w.cfg.ID, w.db, table) }, func() {
		if d.table != table {
			return
		}
		for _, c := range cols {
			d.tableCols = append(d.tableCols, c.Name)
		}
		s.autoMapImport()
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
			d.err, d.preview, d.sql = "", nil, nil
			if app.IsSQLFile(path) {
				s.previewSQLFile()
				return
			}
			if d.newName == "" {
				base := filepath.Base(path)
				for _, ext := range []string{".gz", ".csv", ".tsv", ".json", ".jsonl", ".ndjson", ".txt"} {
					base = strings.TrimSuffix(strings.ToLower(base), ext)
				}
				d.newName = strings.Map(func(r rune) rune {
					if r == ' ' || r == '-' || r == '.' {
						return '_'
					}
					return r
				}, base)
			}
			s.refreshImportPreview()
		}
	})
}

func (s *Shell) previewSQLFile() {
	d := &s.imp
	var pv *app.SQLFilePreview
	var err error
	path := d.path
	s.up.bg(func() { pv, err = s.app.Imports.PreviewSQLFile(path) }, func() {
		if err != nil {
			d.err = err.Error()
			return
		}
		d.sql = pv
	})
}

func (s *Shell) refreshImportPreview() {
	d := &s.imp
	if d.path == "" || d.sql != nil {
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
	opts.Encoding = d.encoding
	opts.Quote = d.quote
	d.opts = opts
	var pv *dataimport.Preview
	var kinds []dataimport.Kind
	var err error
	s.up.bg(func() {
		pv, err = s.app.Imports.PreviewImportFile(d.path, opts)
		if err == nil {
			kinds, _ = dataimport.InferKinds(d.path, opts, 0)
		}
	}, func() {
		if err != nil {
			d.err, d.preview = err.Error(), nil
			return
		}
		d.err, d.preview = "", pv
		d.newCols = make([]importNewCol, len(pv.Columns))
		for i, name := range pv.Columns {
			d.newCols[i] = importNewCol{name: name, kind: "text"}
			if i < len(kinds) {
				d.newCols[i].kind = string(kinds[i])
			}
		}
		s.autoMapImport()
	})
}

// autoMapImport 按所选方式把文件的列对到表的列。
func (s *Shell) autoMapImport() {
	d := &s.imp
	if d.preview == nil {
		return
	}
	d.target = make([]string, len(d.preview.Columns))
	for i, name := range d.preview.Columns {
		if d.mapBy == "order" {
			if i < len(d.tableCols) {
				d.target[i] = d.tableCols[i]
			}
			continue
		}
		for _, tc := range d.tableCols {
			if strings.EqualFold(tc, name) {
				d.target[i] = tc
			}
		}
	}
}

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
			d.err = err.Error()
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
			d.err = err.Error()
			return
		}
		d.open = false
		s.toastOK(locales.ImportStarted())
	})
}
