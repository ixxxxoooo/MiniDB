package dataimport

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"

	"minidb/internal/export"
)

// 本包的测试可以引用 export：只有生产代码受依赖白名单约束。

// 用自带的 xlsx 导出器写文件，再用导入读回来：两边要对得上。
func TestXLSXRoundTripWithExporter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.xlsx")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := export.NewWriter(f, "users", export.Options{Format: "xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	cols := []string{"id", "name", "note"}
	if err := w.Begin(cols); err != nil {
		t.Fatal(err)
	}
	rows := []map[string]interface{}{
		{"id": 1, "name": "张三", "note": "a<b>&c"},
		{"id": 2, "name": "Bob", "note": nil},
	}
	if err := w.WriteRows(cols, rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if DetectFormat(p) != FormatXLSX {
		t.Fatalf("扩展名 .xlsx 应识别为 xlsx，得到 %q", DetectFormat(p))
	}
	r, err := Open(p, Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), cols) {
		t.Fatalf("columns=%v", r.Columns())
	}
	got := readAll(t, r)
	want := [][]interface{}{{"1", "张三", "a<b>&c"}, {"2", "Bob", nil}} // 空单元格是 NULL
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows=%v", got)
	}
}

// 手工造一个多工作表、带空行与不规则行的工作簿。
func makeWorkbook(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	set := func(sheet, cell string, v interface{}) {
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	s1 := f.GetSheetName(0)
	for i, h := range []string{"id", "", "id", "price"} { // 空名、重名都要整理
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		set(s1, cell, h)
	}
	set(s1, "A2", 1)
	set(s1, "B2", "x")
	set(s1, "C2", 7)
	set(s1, "D2", 3.5)
	// 第 3 行整行空白，应跳过
	set(s1, "A4", 2)
	set(s1, "D4", 10)
	if _, err := f.NewSheet("第二张"); err != nil {
		t.Fatal(err)
	}
	set("第二张", "A1", "k")
	set("第二张", "A2", "v1")
	p := filepath.Join(t.TempDir(), "m.xlsx")
	if err := f.SaveAs(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestXLSXSheetsHeaderBlankRowsAndRagged(t *testing.T) {
	p := makeWorkbook(t)
	names, err := XLSXSheets(p)
	if err != nil || !reflect.DeepEqual(names, []string{"Sheet1", "第二张"}) {
		t.Fatalf("sheets=%v err=%v", names, err)
	}
	r, err := Open(p, Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"id", "column_2", "id_2", "price"}) {
		t.Fatalf("columns=%v", r.Columns())
	}
	want := [][]interface{}{{"1", "x", "7", "3.5"}, {"2", nil, nil, "10"}}
	if got := readAll(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows=%v", got)
	}

	// 选第二张工作表
	r2, err := Open(p, Options{HasHeader: true, Sheet: "第二张"})
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	if !reflect.DeepEqual(r2.Columns(), []string{"k"}) || !reflect.DeepEqual(readAll(t, r2), [][]interface{}{{"v1"}}) {
		t.Fatalf("第二张工作表读取不对: %v", r2.Columns())
	}

	// 不勾「首行是列名」：首行当数据
	r3, err := Open(p, Options{HasHeader: false, Sheet: "第二张"})
	if err != nil {
		t.Fatal(err)
	}
	defer r3.Close()
	if !reflect.DeepEqual(r3.Columns(), []string{"column_1"}) || len(readAll(t, r3)) != 2 {
		t.Fatalf("无表头应保留首行: %v", r3.Columns())
	}
}

func TestXLSXPreviewAndInfer(t *testing.T) {
	p := makeWorkbook(t)
	pv, err := BuildPreview(p, Options{HasHeader: true}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Format != FormatXLSX || pv.Sheet != "Sheet1" || len(pv.Sheets) != 2 || pv.TotalRows != 2 {
		t.Fatalf("预览不对: %+v", pv)
	}
	kinds, err := InferKinds(p, Options{HasHeader: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kinds, []Kind{KindInt, KindText, KindInt, KindFloat}) {
		t.Fatalf("kinds=%v", kinds)
	}
}

func TestXLSXRejectsNonWorkbookAndGz(t *testing.T) {
	bad := writeTemp(t, "bad.xlsx", "not a zip")
	if _, err := Open(bad, Options{HasHeader: true}); err == nil {
		t.Fatal("坏文件应报错")
	}
	if _, err := Open(bad+".gz", Options{Format: FormatXLSX}); err == nil {
		t.Fatal(".gz 的 Excel 应报错")
	}
}
