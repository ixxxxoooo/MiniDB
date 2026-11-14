package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// importDialog 选文件、设解析方式、把文件的列对到表的列（或新建一张表），然后开始导入。
type importDialog struct {
	open  bool
	w     *Workspace
	table string // 导入到的现有表
	path  string

	newTable bool   // 导入到新建的表
	newName  string // 新表名
	newCols  []importNewCol

	opts      app.ImportFileOptions
	delimiter string
	encoding  string
	quote     string // "\"" "'" "none"
	preview   *app.ImportPreview
	sql       *app.SQLFilePreview // 选了 .sql 文件时
	err       string

	target    []string // 每个源列对应的表列，空为跳过
	tableCols []string
	mapBy     string // "name" | "order"
	conflict  string // app.Conflict*

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
	return []string{app.ConflictStop, app.ConflictIgnore, app.ConflictReplace, app.ConflictUpdate}
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
			d.err = errMessage(err)
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
			d.err = errMessage(err)
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
	var pv *app.ImportPreview
	var kinds []app.ImportKind
	var err error
	s.up.bg(func() {
		pv, err = s.app.Imports.PreviewImportFile(d.path, opts)
		if err == nil {
			kinds, _ = app.InferImportKinds(d.path, opts, 0)
		}
	}, func() {
		if err != nil {
			d.err, d.preview = errMessage(err), nil
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
