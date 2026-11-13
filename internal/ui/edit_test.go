package ui

import (
	"database/sql"
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func queryCount(t *testing.T, e *env, q string) string {
	t.Helper()
	db, err := sql.Open("sqlite", e.dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var out sql.NullString
	if err := db.QueryRow(q).Scan(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid {
		return "<NULL>"
	}
	return out.String
}

func openUsers(e *env) *tableTab {
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	return w.tabs[0].(*tableTab)
}

func TestEditCommitCycle(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	if len(tab.es.pk) != 1 || tab.es.pk[0] != "id" {
		t.Fatalf("pk = %v", tab.es.pk)
	}

	// edit a cell via the double-click path
	tab.beginEdit(0, "name")
	if tab.es.editing == nil {
		t.Fatal("beginEdit did not start editing")
	}
	tab.es.buf = "Alice"
	tab.commitEdit()
	if tab.pending() != 1 || tab.g.rows[0]["name"] != "Alice" {
		t.Fatalf("pending=%d name=%v", tab.pending(), tab.g.rows[0]["name"])
	}
	// setting a cell back to its original value removes the edit
	tab.setCell(0, "name", tab.es.orig[0]["name"])
	if tab.pending() != 0 {
		t.Fatalf("reverted edit must not count, pending=%d", tab.pending())
	}
	tab.setCell(0, "name", "Alice")
	tab.setCell(1, "email", nil)

	// a new row
	tab.addRow()
	last := len(tab.g.rows) - 1
	tab.setCell(last, "name", "新用户")
	tab.setCell(last, "age", "33")

	// delete row 3 (id 3)
	tab.g.chosen.Clear()
	tab.g.chosen.Add(2)
	tab.deleteSelected()

	if got := tab.pending(); got != 4 { // two cell edits, one added row, one delete
		t.Fatalf("pending = %d", got)
	}
	e.shot("06-editing")
	e.sh.spaces[0].tabs[0].(*tableTab).buildPreview()
	stmts := tab.es.preview.stmts
	joined := strings.Join(stmts, "\n")
	for _, want := range []string{
		`DELETE FROM "users" WHERE "id" = 3;`,
		`INSERT INTO "users"`,
		`UPDATE "users" SET "name" = 'Alice' WHERE "id" = 1;`,
		`UPDATE "users" SET "email" = NULL WHERE "id" = 2;`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("preview lacks %q:\n%s", want, joined)
		}
	}
	// nothing written yet
	if got := queryCount(t, e, `SELECT name FROM users WHERE id=1`); got == "Alice" {
		t.Fatal("preview must not write")
	}
	tab.commitNow() // no dialog: the Commit button and ⌘S write at once
	if e.sh.confirm.open {
		t.Fatal("committing must not ask on a local connection")
	}
	if tab.es.preview.err != "" {
		t.Fatal(tab.es.preview.err)
	}
	if got := queryCount(t, e, `SELECT name FROM users WHERE id=1`); got != "Alice" {
		t.Fatalf("name = %q", got)
	}
	if got := queryCount(t, e, `SELECT email FROM users WHERE id=2`); got != "<NULL>" {
		t.Fatalf("email = %q", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM users`); got != "450" { // -1 +1
		t.Fatalf("count = %s", got)
	}
	if got := queryCount(t, e, `SELECT age FROM users WHERE name='新用户'`); got != "33" {
		t.Fatalf("age = %s", got)
	}
	if tab.pending() != 0 {
		t.Fatalf("pending after commit = %d", tab.pending())
	}
}

func TestEditFailureRollsBack(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "name", "Changed")
	tab.setCell(1, "name", nil) // name is NOT NULL → the transaction must fail
	tab.commitNow()
	if tab.es.preview.err == "" {
		t.Fatal("expected the commit to fail")
	}
	if got := queryCount(t, e, `SELECT name FROM users WHERE id=1`); got == "Changed" {
		t.Fatal("first update must have rolled back")
	}
	if tab.pending() == 0 {
		t.Fatal("edits must stay after a failed commit")
	}
}

func TestEditBlockedWhenReadOnlyOrNoPK(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.es.pk = nil
	tab.beginEdit(0, "name")
	if tab.es.editing != nil {
		t.Fatal("a table without a primary key must not edit existing rows")
	}
	tab.addRow() // added rows are fine without a key
	if tab.es.added != 1 {
		t.Fatal("adding rows works without a key")
	}

	e2 := newEnv(t)
	cfg := e2.cfg
	cfg.ReadOnly = true
	e2.sh.app.Connections.SaveConnection(cfg)
	e2.sh.loadConnections()
	tab2 := openUsers(e2)
	tab2.beginEdit(0, "name")
	tab2.addRow()
	if tab2.es.editing != nil || tab2.es.added != 0 {
		t.Fatal("read-only connections must not allow edits")
	}
}

func TestEditGuardOnPageChange(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "name", "X")
	e.tt.Frame()
	tab.reload(func() { tab.page = 2 })
	e.has("丢弃未提交的修改")
	if tab.page != 1 {
		t.Fatal("page must not change before the user agrees")
	}
	if err := e.tt.Click("丢弃"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if tab.page != 2 || tab.pending() != 0 {
		t.Fatalf("page=%d pending=%d", tab.page, tab.pending())
	}
}

func TestEditViaDoubleClick(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	rect, ok := e.tt.Find("user3@example.com")
	if !ok {
		t.Fatal("cell not found")
	}
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	e.tt.ClickAt(x, y)
	e.tt.ClickAt(x, y)
	e.tt.Frame()
	if tab.es.editing == nil || tab.es.editing.col != "email" || tab.es.editing.row != 2 {
		t.Fatalf("double click should edit the email of row 2, editing=%+v", tab.es.editing)
	}
	e.tt.Type("!")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if tab.es.editing != nil || tab.pending() != 1 {
		t.Fatalf("Enter commits the edit; editing=%+v pending=%d", tab.es.editing, tab.pending())
	}
}

func TestRowPreviewPane(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.g.chosen.Add(2)
	e.tt.Key(0, mui.KeySpace)
	e.tt.Frame()
	if !tab.showPreview {
		t.Fatal("Space should open the row preview")
	}
	e.has("行预览")
	e.has("user3@example.com")
	e.shot("15-row-preview")
	if got, ok := prettyValue(`{"a":[1,2]}`); !ok || !strings.Contains(got, "\n  \"a\"") {
		t.Fatalf("json must be indented: %q", got)
	}
	if got, ok := prettyValue("plain {text}"); ok || got != "plain {text}" {
		t.Fatalf("non-json stays: %q", got)
	}
	e.tt.Key(0, mui.KeySpace)
	e.tt.Frame()
	if tab.showPreview {
		t.Fatal("Space again should close it")
	}
}

// 在行预览里改完提交后，预览不关闭，仍选着这一行，并显示刚写入库里的最新值。
func TestRowPreviewRefreshesAfterCommit(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	for i := 0; i < 200 && len(tab.es.pk) == 0; i++ { // 等结构信息加载完（找主键）
		e.tt.Frame()
		sleepMs(10)
	}
	tab.g.chosen.Add(2)
	e.tt.Key(0, mui.KeySpace)
	e.tt.Frame()
	if !tab.showPreview {
		t.Fatal("Space 应打开行预览")
	}
	tab.setCell(2, "name", "刷新验证")
	if tab.pendingCount() != 1 {
		t.Fatalf("pending = %d", tab.pendingCount())
	}
	tab.commitNow()
	for i := 0; i < 300 && (tab.pendingCount() != 0 || tab.loading || queryCount(t, e, `SELECT COUNT(*) FROM users WHERE name='刷新验证'`) != "1"); i++ {
		e.tt.Frame()
		sleepMs(10)
	}
	e.tt.Frame()
	if !tab.showPreview {
		t.Fatal("提交后行预览不应关闭")
	}
	if rows := tab.chosenRows(); len(rows) != 1 || rows[0] != 2 {
		t.Fatalf("提交后应仍选着这一行，实际 %v", rows)
	}
	if got := tab.pv.buf["name"]; got != "刷新验证" {
		t.Fatalf("预览应显示最新值，实际 %q", got)
	}
}

// 行预览里的 JSON 值用带语法高亮的编辑器，类型标在字段名右侧，下拉箭头在输入框右端；
// 在编辑器里改动会进入待提交的修改，并且不会被反过来重新缩进。
func TestRowPreviewJSONFieldUsesHighlightedEditor(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.g.rows[2]["bio"] = `{"lang":"zh-CN","theme":"dark","notify":true,"tags":[1,2]}`
	tab.g.chosen.Add(2)
	tab.showPreview, tab.previewW = true, 340
	e.tt.Frame()
	e.tt.Frame()
	if !tab.pv.json["bio"] || tab.pv.ed["bio"] == nil {
		t.Fatalf("JSON 字段应使用编辑器: json=%v", tab.pv.json)
	}
	if tab.pv.json["email"] {
		t.Fatal("普通文本不应使用 JSON 编辑器")
	}
	e.shot("15-row-preview-json")
	before := tab.pendingCount()
	ed := tab.pv.ed["bio"]
	ed.Buffer.Set(`{"lang":"en"}`)
	e.tt.Frame()
	e.tt.Frame()
	if tab.pendingCount() != before+1 || tab.g.rows[2]["bio"] != `{"lang":"en"}` {
		t.Fatalf("在 JSON 编辑器里改动应成为待提交的修改: pending=%d bio=%v", tab.pendingCount(), tab.g.rows[2]["bio"])
	}
	if got := ed.Buffer.String(); got != `{"lang":"en"}` {
		t.Fatalf("输入内容不应被改写: %q", got)
	}
}
