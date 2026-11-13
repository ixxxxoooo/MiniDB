package ui

import (
	"minidb/internal/database"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 导出对话框：选两张表导出为 Excel 文件，选项落到请求里。
func TestExportDialogMultiTable(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "x.csv")
	e.sh.host = fakeHost{save: out}
	e.sh.app = rebuildWithHost(e, fakeHost{save: out})
	w := openWorkspace(e)

	e.sh.exportTables(w)
	e.tt.Frame()
	e.has("导出的表")
	if len(e.sh.exp.chosenTables()) != 0 {
		t.Fatal("开始时不应预选表")
	}
	e.sh.exp.sel["users"], e.sh.exp.sel["orders"] = true, true
	e.sh.exp.format = 3
	e.tt.Frame()
	e.has("文件名")
	e.hasNot("压缩为 .gz") // Excel 本身已压缩
	e.shot("47-export-multi")
	e.sh.exp.format = 2
	e.sh.exp.limit = "100"
	e.tt.Frame()
	e.has("INSERT 合并")
	e.has("文件名示例")
	e.shot("47-export-sql")
	e.sh.exp.format = 3

	if e.sh.exp.limitRows() != 100 {
		t.Fatalf("行数上限没解析出来: %d", e.sh.exp.limitRows())
	}
	e.sh.exp.limit = "abc"
	if e.sh.exp.limitRows() != 0 {
		t.Fatal("非数字的上限应按全部行")
	}
	opts := e.sh.exp.exportOptions()
	if opts.Format != "xlsx" || opts.Gzip {
		t.Fatalf("选项 %+v", opts)
	}
	e.sh.exp.format = 0
	e.sh.exp.prefs.Delim, e.sh.exp.prefs.Quote, e.sh.exp.prefs.NullKeep = ";", "none", true
	opts = e.sh.exp.exportOptions()
	if opts.CSV.Delimiter != ";" || opts.CSV.Quote != "none" || opts.CSV.NullText != "NULL" {
		t.Fatalf("CSV 选项 %+v", opts.CSV)
	}

	// 只选一张表：列清单出现，去掉一列后只导出剩下的
	e.sh.exportTable(w, "users")
	e.tt.Frame()
	e.tt.Frame()
	if len(e.sh.exp.cols) == 0 {
		t.Fatal("没有读到列")
	}
	e.sh.exp.colSel["bio"] = false
	e.sh.exp.prefs.Gzip = false
	e.sh.exp.prefs.NoBOM = true
	e.sh.startExport(e.sh.exp)
	var data string
	for i := 0; i < 300; i++ {
		if b, err := os.ReadFile(out); err == nil && strings.Count(string(b), "\n") >= 450 {
			data = string(b)
			break
		}
		sleepMs(10)
	}
	if !strings.HasPrefix(data, "id,name,email,age\n") {
		t.Fatalf("列没有被限定: %q", firstLine(data))
	}
	if !e.sh.prefs.Export.NoBOM {
		t.Error("导出选项应被记住")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// 导入对话框：选一个 CSV，建新表导入；再选一个 SQL 转储执行。
func TestImportDialogNewTableAndSQLDump(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "city list.csv")
	os.WriteFile(csvPath, []byte("id,name,score\n1,北京,1.5\n2,上海,2\n"), 0o600)
	e.sh.app = rebuildWithHost(e, fakeHost{open: csvPath})
	w := openWorkspace(e)

	e.sh.importFile(w)
	e.tt.Frame()
	e.has("导入数据")
	e.sh.pickImportFile()
	e.tt.Frame()
	e.tt.Frame()
	d := &e.sh.imp
	if d.preview == nil || !d.newTable || d.newName != "city_list" {
		t.Fatalf("预览/新表名不对: %+v %q", d.preview != nil, d.newName)
	}
	if d.newCols[0].kind != "int" || d.newCols[2].kind != "float" || d.newCols[1].kind != "text" {
		t.Fatalf("类型推断: %+v", d.newCols)
	}
	e.has("预览")
	e.shot("48-import-new")
	e.sh.confirmImport()
	for i := 0; i < 300 && queryCount(t, e, `SELECT COUNT(*) FROM sqlite_master WHERE name='city_list'`) != "1"; i++ {
		sleepMs(10)
	}
	for i := 0; i < 300 && queryCount(t, e, `SELECT COUNT(*) FROM city_list`) != "2"; i++ {
		sleepMs(10)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM city_list WHERE name='北京'`); got != "1" {
		t.Fatalf("导入后: %s", got)
	}

	sqlPath := filepath.Join(dir, "d.sql")
	os.WriteFile(sqlPath, []byte("CREATE TABLE from_dump (x INTEGER);\nINSERT INTO from_dump VALUES (1),(2);\nDROP TABLE IF EXISTS nothing;\n"), 0o600)
	e.sh.app = rebuildWithHost(e, fakeHost{open: sqlPath})
	e.sh.importFile(w)
	e.sh.pickImportFile()
	e.tt.Frame()
	e.tt.Frame()
	if e.sh.imp.sql == nil || e.sh.imp.sql.Count != 3 {
		t.Fatalf("SQL 预览: %+v", e.sh.imp.sql)
	}
	e.has("SQL 转储")
	e.shot("49-import-sql")
	e.sh.confirmImport()
	for i := 0; i < 300 && queryCount(t, e, `SELECT COUNT(*) FROM sqlite_master WHERE name='from_dump'`) != "1"; i++ {
		sleepMs(10)
	}
	for i := 0; i < 300 && queryCount(t, e, `SELECT COUNT(*) FROM from_dump`) != "2"; i++ {
		sleepMs(10)
	}
}

func TestProcessDialogHelpers(t *testing.T) {
	d := processDialog{sel: map[string]bool{"2": true, "3": true}, items: []database.ProcessInfo{
		{ID: "1", Database: "a", Info: "select 1"},
		{ID: "2", Database: "b", User: "u", Info: "select\t2\nx"},
		{ID: "3", Database: "a"},
	}}
	if got := d.databases(); strings.Join(got, ",") != "a,b" {
		t.Fatal(got)
	}
	d.dbSel = "a"
	if v := d.visible(); len(v) != 2 || v[0].ID != "1" {
		t.Fatalf("过滤: %+v", v)
	}
	// 勾选了 2、3，但 2 被过滤掉，只剩 3
	if sel := d.selected(); len(sel) != 1 || sel[0].ID != "3" {
		t.Fatalf("勾选: %+v", sel)
	}
	tsv := processTSV(d.items[1:2])
	if !strings.HasPrefix(tsv, "ID\tUser") || !strings.Contains(tsv, "select 2 x") || strings.Count(tsv, "\n") != 2 {
		t.Fatalf("TSV: %q", tsv)
	}
}

func TestProcessDialogShot(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{"8": true}, items: []database.ProcessInfo{
		{ID: "7", User: "root", Database: "shop", TimeSec: 3, Info: "SELECT * FROM orders"},
		{ID: "8", User: "app", Database: "shop", TimeSec: 41, Info: "UPDATE users SET age = age + 1"},
		{ID: "9", User: "app", Database: "blog", TimeSec: 0, Info: "SELECT 1"},
	}}
	e.tt.Frame()
	e.has("终止所选（1）")
	e.shot("50-process-multi")
}
