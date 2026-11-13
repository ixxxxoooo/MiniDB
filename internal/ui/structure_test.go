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
	st.newIndex.name, st.newIndex.columns, st.newIndex.unique = "idx_email", []string{"email"}, true
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
