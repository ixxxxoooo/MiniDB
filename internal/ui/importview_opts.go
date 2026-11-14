package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// importCSVOpts 是 CSV 才有的解析选项：分隔符、编码、引号。
func (s *Shell) importCSVOpts(c *ui.Context, p tokens) {
	d := &s.imp
	ui.Text(c, locales.ImportDelimiter()).TextColor(p.fgSecondary).FontSize(font2XS)
	if ui.Select(c, &d.delimiter, delimiterChoices).MinWidth(0).Width(80).Height(sizeBtnSm).Changed() {
		s.refreshImportPreview()
	}
	ui.Text(c, locales.ImportEncoding()).TextColor(p.fgSecondary).FontSize(font2XS)
	if ui.Select(c, &d.encoding, app.ImportEncodings).MinWidth(0).Width(110).Height(sizeBtnSm).Changed() {
		s.refreshImportPreview()
	}
	ui.Text(c, locales.ImportQuote()).TextColor(p.fgSecondary).FontSize(font2XS)
	if labelSelect(c, &d.quote, importQuotes, []string{`"`, `'`, locales.ExportQuoteNone()}, 90) {
		s.refreshImportPreview()
	}
}

// importSheetPick 是 Excel 的解析选项：选工作表（列名与类型随之重新推断）。
func (s *Shell) importSheetPick(c *ui.Context, p tokens) {
	d := &s.imp
	ui.Text(c, locales.ImportSheet()).TextColor(p.fgSecondary).FontSize(font2XS)
	if ui.Select(c.Key("imp-sheet"), &d.opts.Sheet, d.preview.Sheets).MinWidth(0).Width(160).Height(sizeBtnSm).Changed() {
		s.refreshImportPreview()
	}
}
