package ui

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// @author ixxxxoooo

// TestImportDialogXLSXSheetPicker 选了 Excel 文件后：显示工作表下拉，CSV 的分隔符 / 引号选项收起；
// 切换工作表后重新预览。
func TestImportDialogXLSXSheetPicker(t *testing.T) {
	e := newEnv(t)
	x := excelize.NewFile()
	defer x.Close()
	_ = x.SetSheetRow("Sheet1", "A1", &[]interface{}{"id", "name"})
	_ = x.SetSheetRow("Sheet1", "A2", &[]interface{}{1, "alpha"})
	_, _ = x.NewSheet("Other")
	_ = x.SetSheetRow("Other", "A1", &[]interface{}{"k"})
	_ = x.SetSheetRow("Other", "A2", &[]interface{}{"beta-row"})
	path := filepath.Join(t.TempDir(), "d.xlsx")
	if err := x.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	openUsers(e)
	e.sh.importFile(e.sh.spaces[0])
	e.sh.imp.path = path
	e.sh.imp.opts.HasHeader = true
	e.sh.refreshImportPreview()
	e.tt.Frame()
	e.tt.Frame()
	d := e.sh.imp
	if d.err != "" || d.preview == nil || len(d.preview.Sheets) != 2 || d.opts.Sheet != "Sheet1" {
		t.Fatalf("预览不对: err=%q preview=%+v sheet=%q", d.err, d.preview, d.opts.Sheet)
	}
	e.has("工作表")
	e.has("alpha")
	e.hasNot("分隔符")

	e.sh.imp.opts.Sheet = "Other"
	e.sh.refreshImportPreview()
	e.tt.Frame()
	e.tt.Frame()
	e.has("beta-row")
}
