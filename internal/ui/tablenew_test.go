package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// 新建表：打开只有结构页的标签，填表名和列，⌘S 建出来，标签变成这张表的普通标签。
func TestNewTableCreate(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newTable()
	e.tt.Frame()
	tab := w.tabs[len(w.tabs)-1].(*tableTab)
	if !tab.isNew || tab.mode != 1 || len(tab.st.cols) != 1 || tab.st.cols[0].Name != "id" {
		t.Fatalf("新建表标签: %+v", tab.st.cols)
	}
	if tab.title() != "未命名表" || tab.pending() != 0 {
		t.Fatalf("没动过的新建表不应算有改动: title=%q pending=%d", tab.title(), tab.pending())
	}
	e.has("表名")
	e.shot("25-new-table")

	// 没有表名：报错，不建
	tab.st.name = ""
	tab.st.cols = append(tab.st.cols, newCol("title", "TEXT"))
	if tab.pending() == 0 {
		t.Fatal("填了列就应算有改动")
	}
	tab.commitStructure()
	if _, err := tab.newTableStatements(); err == nil {
		t.Fatal("没有表名应报错")
	}

	tab.st.name = "posts"
	tab.st.idx = append(tab.st.idx, newIdx("idx_posts_title", "title", false))
	if tab.title() != "posts" {
		t.Fatalf("title = %q", tab.title())
	}
	tab.buildStructurePreview()
	joined := strings.Join(tab.st.preview.stmts, "\n")
	if !strings.Contains(joined, "CREATE TABLE") || !strings.Contains(joined, "AUTOINCREMENT") || !strings.Contains(joined, "idx_posts_title") {
		t.Fatalf("preview: %q err=%q", joined, tab.st.preview.err)
	}
	tab.commitStructure()
	e.tt.Frame()
	if tab.isNew || tab.table != "posts" {
		t.Fatalf("建好后应变成普通标签: isNew=%v table=%q err=%q", tab.isNew, tab.table, tab.st.preview.err)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_table_info('posts')`); got != "2" {
		t.Fatalf("列数 = %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_list('posts') WHERE name='idx_posts_title'`); got != "1" {
		t.Fatalf("索引 = %s", got)
	}
	found := false
	for _, tb := range w.tables {
		found = found || tb.Name == "posts"
	}
	if !found {
		t.Fatal("侧栏应刷新出 posts")
	}
	if tab.st.newTable || len(tab.st.cols) != 2 {
		t.Fatalf("建好后应按已有的表重新载入: %+v", tab.st.cols)
	}
}

// 新建表的标签不进会话，也不会因为刷新去查名为空的表。
func TestNewTableNotInSessionAndRefreshSafe(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newTable()
	tab := w.tabs[len(w.tabs)-1].(*tableTab)
	for _, ts := range w.snapshot().Tabs {
		if ts.Table == "" && ts.Kind == "table" {
			t.Fatal("新建表不应存进会话")
		}
	}
	tab.reload(nil)
	tab.load()
	e.tt.Frame()
	if tab.err != "" || tab.loading {
		t.Fatalf("刷新不应去查表: err=%q loading=%v", tab.err, tab.loading)
	}
	// 丢弃修改：回到预先给的那一列
	tab.st.name = "x"
	tab.st.cols = append(tab.st.cols, newCol("a", "TEXT"))
	tab.st.reset()
	if tab.st.name != "" || len(tab.st.cols) != 1 || tab.st.changeCount() != 0 {
		t.Fatalf("reset 后: %q %+v", tab.st.name, tab.st.cols)
	}
}

// 重命名：改结构页顶部的表名，⌘S 一起提交；标签、侧栏、固定项都跟着改。
func TestRenameTableFromStructure(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	st := &tab.st
	if st.name != "users" || st.changeCount() != 0 {
		t.Fatalf("name=%q changes=%d", st.name, st.changeCount())
	}
	tab.w.togglePin("users")
	st.name = "  people "
	if got := st.changeCount(); got != 1 {
		t.Fatalf("改名应算 1 项改动: %d", got)
	}
	tab.buildStructurePreview()
	if joined := strings.Join(st.preview.stmts, "\n"); !strings.Contains(joined, `RENAME TO "people"`) {
		t.Fatalf("preview: %q", joined)
	}
	// 同时改列：先改列，最后改名
	st.cols[1].Name = "full_name"
	tab.commitStructure()
	e.tt.Frame()
	if st.preview.err != "" {
		t.Fatal(st.preview.err)
	}
	if tab.table != "people" || tab.title() != "people" || tab.g.table != "people" {
		t.Fatalf("标签应改名: table=%q", tab.table)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM people`); got != "450" {
		t.Fatalf("改名后的行数 = %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_table_info('people') WHERE name='full_name'`); got != "1" {
		t.Fatalf("列改名没生效: %s", got)
	}
	if !tab.w.isPinned("people") || tab.w.isPinned("users") {
		t.Fatalf("固定项应跟着改名: %v", tab.w.side.pins)
	}
	if st.name != "people" || st.changeCount() != 0 {
		t.Fatalf("提交后: name=%q changes=%d", st.name, st.changeCount())
	}
}

// 清空、删除表的确认框：只有 PostgreSQL 有 Restart identity / Cascade 选项，勾选后写进语句。
func TestTableOpOptionsByDatabase(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	for _, tc := range []struct {
		db          string
		trunc, drop int
	}{{"sqlite", 0, 0}, {"mysql", 0, 0}, {"postgres", 2, 1}} {
		w.cfg.Type = tc.db
		w.truncateTable("users")
		if got := len(e.sh.confirm.opts); got != tc.trunc {
			t.Fatalf("%s 清空的选项数 = %d, want %d", tc.db, got, tc.trunc)
		}
		w.dropTable("users")
		if got := len(e.sh.confirm.opts); got != tc.drop {
			t.Fatalf("%s 删除的选项数 = %d, want %d", tc.db, got, tc.drop)
		}
	}
	w.cfg.Type = "sqlite"
	e.sh.confirm = confirmDialog{}
}

// 侧栏里选中表后单按 Delete 删除（先确认）；确认后表真的没了。
func TestSidebarDeleteKeyDropsTable(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if err := e.tt.Click("orders"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.tt.Key(0, mui.KeyDelete)
	e.tt.Frame()
	if !e.sh.confirm.open || !strings.Contains(e.sh.confirm.message, "orders") {
		t.Fatalf("Delete 应先确认删除 orders: %+v", e.sh.confirm)
	}
	e.sh.confirm.action()
	e.tt.Frame()
	for _, tb := range w.tables {
		if tb.Name == "orders" {
			t.Fatal("orders 应已删除")
		}
	}
}

// 结构页里改动过的行在提交之前一直有底色，和数据行一致：新增绿、待删除红、修改黄。
func TestStructureRowsAreTintedUntilCommitted(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	st := &tab.st
	pt := lightTokens
	if structTint(pt, false, false, false) != (mui.Color{}) {
		t.Fatal("没动过的行没有底色")
	}
	if structTint(pt, true, false, false) != pt.rowNew || structTint(pt, false, true, false) != pt.rowDelete || structTint(pt, false, false, true) != pt.cellEdit {
		t.Fatal("新增绿、删除红、修改黄")
	}
	if st.colModified(st.cols[1]) {
		t.Fatal("刚载入不应有修改")
	}
	st.cols[1].Name = "renamed"
	if !st.colModified(st.cols[1]) {
		t.Fatal("改名应算修改")
	}
	st.cols[1].Name = st.orig[1].Name
	if st.colModified(st.cols[1]) {
		t.Fatal("改回去就不再算")
	}
	st.idx[0].IsUnique = true
	if !st.idxModified(st.idx[0]) {
		t.Fatal("索引改了唯一应算修改")
	}
	st.addColumn()
	e.tt.Frame()
	e.shot("27-struct-tint")
}
