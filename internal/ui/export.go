package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
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
func (d exportDialog) exportOptions() app.ExportOptions {
	pf := d.prefs
	o := app.ExportOptions{Format: exportFormats[d.format], Gzip: pf.Gzip && d.format != 3}
	o.CSV = app.ExportCSVOptions{
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
	o.SQL = app.ExportSQLOptions{Structure: pf.SQLStructure, DropTable: pf.SQLDrop, NoData: pf.SQLNoData, BatchSize: pf.SQLBatch}
	o.JSON = app.ExportJSONOptions{Lines: pf.JSONLines}
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
			s.toastErr(errMessage(err))
		}
	})
}
