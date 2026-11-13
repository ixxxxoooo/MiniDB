package ui

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/app"
)

func TestStructureEditPreviewApply(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	e.has("添加列")
	st := &tab.st
	if !st.loaded || len(st.cols) != 5 {
		t.Fatalf("loaded=%v cols=%d", st.loaded, len(st.cols))
	}
	if len(st.idx) == 0 {
		t.Fatal("indexes not loaded")
	}
	e.shot("08-structure")

	// no edits → no changes
	if st.changeCount() != 0 {
		t.Fatalf("changes = %d", st.changeCount())
	}
	// rename a column comment-free: change the age column type, add a column, add an index
	for i := range st.cols {
		if st.cols[i].UID == "age" {
			st.cols[i].Name = "years"
		}
	}
	st.nextUID++
	st.cols = append(st.cols, newCol("nickname", "TEXT"))
	st.idx = append(st.idx, newIdx("idx_email", "email", true))
	if st.changeCount() != 3 {
		t.Fatalf("changes = %d, want 3 (rename, add column, add index)", st.changeCount())
	}
	tab.buildStructurePreview()
	joined := strings.Join(st.preview.stmts, "\n")
	if !strings.Contains(joined, "nickname") || !strings.Contains(joined, "idx_email") {
		t.Fatalf("preview: err=%q\n%s", st.preview.err, joined)
	}
	tab.commitStructure() // straight to the database, no dialog
	if st.preview.err != "" {
		t.Fatal(st.preview.err)
	}
	// reflected in the database
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name IN ('years','nickname')`); got != "2" {
		t.Fatalf("columns after apply = %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_list('users') WHERE name='idx_email'`); got != "1" {
		t.Fatalf("index after apply = %s", got)
	}
	// data survived the table rebuild SQLite may do
	if got := queryCount(t, e, `SELECT COUNT(*) FROM users`); got != "450" {
		t.Fatalf("rows after apply = %s", got)
	}
	if st.changeCount() != 0 {
		t.Fatal("working copy must reset after apply")
	}
}

func TestStructureReadOnlyAndDDL(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 2
	tab.modeChanged()
	e.has("复制 DDL")
	e.shot("10-ddl")
	if !strings.Contains(tab.ddl.ed.Buffer.String(), "CREATE TABLE") {
		t.Fatal("ddl is not a CREATE TABLE")
	}
	if !strings.Contains(tab.ddl.ed.Buffer.String(), "users") {
		t.Fatal("ddl lacks the table name")
	}
}

func TestDropAndTruncate(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	w.truncateTable("orders") // asks first
	e.has("清空表")
	e.tt.Key(0, mui.KeyEnter) // the last button is the default
	e.tt.Frame()
	if got := queryCount(t, e, `SELECT COUNT(*) FROM orders`); got != "0" {
		t.Fatalf("orders rows = %s", got)
	}
	w.dropTable("orders")
	e.has("删除表")
	e.tt.Key(0, mui.KeyTab) // 破坏性确认的默认焦点在"取消"，Tab 到确认键再回车
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if got := queryCount(t, e, `SELECT COUNT(*) FROM sqlite_master WHERE name='orders'`); got != "0" {
		t.Fatalf("orders still exists: %s", got)
	}
	if len(w.tabs) != 0 {
		t.Fatalf("tab of a dropped table must close, tabs=%d", len(w.tabs))
	}
}

func TestExportViaTasks(t *testing.T) {
	e := newEnv(t)
	out := filepath.Join(t.TempDir(), "users.csv")
	e.sh.host = fakeHost{save: out}
	e.sh.app = rebuildWithHost(e, fakeHost{save: out})
	w := openWorkspace(e)
	e.sh.exportTable(w, "users")
	e.sh.exp.format = 0
	e.sh.startExport(e.sh.exp)
	var records [][]string
	for i := 0; i < 300; i++ {
		if f, err := os.Open(out); err == nil {
			r := csv.NewReader(strings.NewReader(strings.TrimPrefix(readAll(f), "\ufeff")))
			r.FieldsPerRecord = -1
			records, _ = r.ReadAll()
			f.Close()
			if len(records) >= 451 {
				break
			}
		}
		sleepMs(10)
	}
	if len(records) != 451 || strings.Join(records[0], ",") != "id,name,email,age,bio" {
		t.Fatalf("export: %d records, header %v", len(records), records[0])
	}
	if records[1][1] != "用户001" || records[1][4] != "line1\nline2" {
		t.Fatalf("first row = %q", records[1])
	}
}

var _ = app.TopicExport

func readAll(r io.Reader) string { b, _ := io.ReadAll(r); return string(b) }

func TestMoveItems(t *testing.T) {
	list := []string{"a", "b", "c", "d", "e"}
	for _, c := range []struct {
		rows []int
		to   int
		want string
	}{
		{[]int{1}, 4, "acdbe"},    // 往下挪：to 是原下标，要先扣掉自己
		{[]int{3}, 0, "dabce"},    // 挪到最前
		{[]int{0}, 5, "bcdea"},    // 挪到末尾
		{[]int{1, 3}, 0, "bdace"}, // 多行一起挪，保持相对顺序
		{[]int{2}, 2, "abcde"},    // 原地不动
		{[]int{2}, 3, "abcde"},    // 紧挨着自己的下一个位置，也等于没动
		{[]int{0, 1}, 4, "cdabe"}, // 一组连续行
	} {
		if got := strings.Join(moveItems(list, c.rows, c.to), ""); got != c.want {
			t.Errorf("moveItems(%v, %d) = %s, want %s", c.rows, c.to, got, c.want)
		}
	}
	if strings.Join(list, "") != "abcde" {
		t.Fatal("moveItems 不该改动原切片")
	}
}

// 「添加索引」不再弹窗：表格里直接多出一行，名字格拿到焦点，填完列就能提交，
// 名字留空则自动起名。
func TestStructureInlineAddIndex(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	e.tt.Frame()
	st := &tab.st
	before := len(st.idx)
	tab.addIndex()
	e.shot("08-structure-add-index")
	if len(st.idx) != before+1 || st.sub != 0 {
		t.Fatalf("应多出一行: idx=%d sub=%d", len(st.idx), st.sub)
	}
	if st.changeCount() != 0 {
		t.Fatal("还没选列的空行不算修改")
	}
	ix := &st.idx[len(st.idx)-1]
	ix.Columns, ix.IsUnique = []string{"email", "age"}, true
	e.shot("08-structure-add-index-filled")
	if st.changeCount() != 1 {
		t.Fatalf("选了列之后应算 1 处修改, got %d", st.changeCount())
	}
	tab.buildStructurePreview()
	if joined := strings.Join(st.preview.stmts, "\n"); !strings.Contains(joined, "uniq_users_email_age") {
		t.Fatalf("没起名时应自动取名 uniq_users_email_age: %s", joined)
	}
	tab.commitStructure()
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_list('users') WHERE name='uniq_users_email_age' AND "unique"=1`); got != "1" {
		t.Fatalf("索引没建出来: %s", got)
	}
}

// 列行可以上下拖动换位（MySQL 系）；其他库不允许拖，也就不会有 Reorder。
func TestStructureDragReorderColumns(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	e.tt.Frame()
	st := &tab.st
	e.tt.Frame()
	if st.colList.Reorder != nil {
		t.Fatal("SQLite 改不了列序，不该能拖")
	}
	tab.w.cfg.Type = "mysql" // 只为打开拖动：列顺序的 SQL 由后端单测覆盖
	e.tt.Frame()
	e.tt.Frame()
	if st.colList.Reorder == nil {
		t.Fatal("MySQL 系应能拖动列")
	}
	if st.cols[0].Name != "id" || st.cols[1].Name != "name" {
		t.Fatalf("起始顺序: %v", st.cols)
	}
	// 抓住第 4 行（age）的序号格，拖到第 2 行（name）上半，应该排到 name 前面
	src, ok := e.tt.Find("4")
	if !ok {
		t.Fatal("找不到第 4 行的序号")
	}
	dst, ok := e.tt.Find("2")
	if !ok {
		t.Fatal("找不到第 2 行的序号")
	}
	e.tt.Press(src.X+src.W/2, src.Y+src.H/2)
	e.tt.Move(src.X+src.W/2, src.Y+src.H/2-8)
	e.tt.Move(dst.X+dst.W/2, dst.Y+2)
	e.shot("08-structure-drag")
	e.tt.Release(dst.X+dst.W/2, dst.Y+2)
	e.tt.Frame()
	var names []string
	for _, c := range st.cols {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "id,age,name,email,bio" {
		t.Fatalf("拖动后的列序 = %s", got)
	}
	if st.changeCount() != 1 {
		t.Fatalf("只调了顺序也算 1 处修改（要能提交）, got %d", st.changeCount())
	}
	st.reset()
	if st.changeCount() != 0 || st.cols[1].Name != "name" {
		t.Fatal("丢弃修改应恢复列序")
	}
}
