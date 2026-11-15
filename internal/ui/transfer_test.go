package ui

import (
	"minidb/internal/app"
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
	tab, ok := e.sh.xf.t, e.sh.xf.open
	if !ok || tab.mode != xferExport || tab.step != 0 {
		t.Fatalf("应打开导出向导的第一步: %#v", e.sh.xf)
	}
	e.has("选择对象")
	if len(e.sh.exp.chosenTables()) != 0 {
		t.Fatal("开始时不应预选表")
	}
	e.sh.exp.sel["users"], e.sh.exp.sel["orders"] = true, true
	e.sh.exp.format = 3
	tab.step = 1
	e.tt.Frame()
	e.tt.Frame()
	e.shot("47-export-multi")
	e.sh.exp.format = 2
	e.sh.exp.limit = "100"
	e.tt.Frame()
	e.has("INSERT 合并")
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
	tab.step = 0
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
	itab, ok := e.sh.xf.t, e.sh.xf.open
	if !ok || itab.mode != xferImport || itab.step != 0 {
		t.Fatalf("应打开导入向导的第一步: %#v", e.sh.xf)
	}
	e.has("选择要导入的文件")
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
	e.has("北京")
	e.shot("48-import-new")
	itab.step = 2
	e.tt.Frame()
	e.tt.Frame()
	e.shot("48-import-map")
	itab.waiting = true
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
	d := processDialog{sel: map[string]bool{"2": true, "3": true}, items: []app.ProcessInfo{
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
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{"8": true}, items: []app.ProcessInfo{
		{ID: "7", User: "root", Database: "shop", Command: "Query", TimeSec: 3, Info: "SELECT * FROM orders"},
		{ID: "8", User: "app", Database: "shop", Command: "Query", TimeSec: 41, Info: "UPDATE users SET age = age + 1"},
		{ID: "9", User: "app", Database: "blog", Command: "Sleep", TimeSec: 0, Info: "SELECT 1"},
	}}
	w.showMonitorTab()
	e.tt.Frame()
	e.has("终止所选（1）")
	e.shot("50-process-multi")
}

// 任务事件可能比 begin(id) 先到（后台协程里启动任务，事件已经发出）：begin 之后要能补上。
func TestXferRunEventBeforeBegin(t *testing.T) {
	var r xferRun
	r.note(xferEvent{id: "t1", status: "progress", cur: 3, total: 10})
	if r.active || r.running() {
		t.Fatal("没 begin 前不该算在运行")
	}
	r.begin("t1")
	if !r.active || r.cur != 3 || r.status != "progress" || !r.running() {
		t.Fatalf("begin 应补上先到的事件: %+v", r)
	}
	r.note(xferEvent{id: "other", status: "done"})
	if r.status != "progress" {
		t.Fatal("别的任务的事件不该串进来")
	}
	r.note(xferEvent{id: "t1", status: "done", cur: 10, total: 10})
	if r.running() || r.bar() != 1 {
		t.Fatalf("完成后: %+v bar=%v", r, r.bar())
	}
}

// 向导的步骤条：走过的步可以点回去，执行步要开始后才进得去。
func TestTransferStepNavigation(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.exportTables(w)
	e.tt.Frame()
	tab := e.sh.xf.t
	e.has("下一步")
	e.sh.exp.sel["users"] = true
	e.tt.Frame()
	e.tt.Click("下一步")
	e.tt.Frame()
	if tab.step != 1 {
		t.Fatalf("点下一步应到第二步: %d", tab.step)
	}
	e.tt.Click("上一步")
	e.tt.Frame()
	if tab.step != 0 {
		t.Fatalf("点上一步应回第一步: %d", tab.step)
	}
	// 再打开一次沿用同一份向导；换成导入就是另一份
	e.sh.exportTables(w)
	if !e.sh.xf.open || e.sh.xf.t.mode != xferExport {
		t.Fatalf("再次打开导出向导应仍是导出: %+v", e.sh.xf)
	}
	e.sh.importFile(w)
	if !e.sh.xf.open || e.sh.xf.t.mode != xferImport {
		t.Fatalf("导入向导是另一份: %+v", e.sh.xf)
	}
}
