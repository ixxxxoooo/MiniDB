package ui

import (
	"strings"
	"testing"

	"minidb/internal/app"
	"minidb/internal/model"
)

func execSQL(t *testing.T, e *env, w *Workspace, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if err := e.sh.app.Databases.ExecuteRawSQL(w.cfg.ID, w.db, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// 触发器就地编辑：新增一行、改已有的语句、删除，全部攒在本地，提交时才一起执行。
func TestStructureTriggersInlineEdit(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	execSQL(t, e, w, `CREATE TABLE audit (msg TEXT)`,
		`CREATE TRIGGER trg_old AFTER INSERT ON orders FOR EACH ROW BEGIN INSERT INTO audit VALUES ('old'); END`)
	w.openTable("orders")
	e.tt.Frame()
	tab := w.tabs[0].(*tableTab)
	tab.mode = 1
	tab.modeChanged()
	e.has("触发器")
	tab.st.sub = 2 // 下半区切到「触发器」标签
	e.has("trg_old")
	o := &tab.st.obj
	if len(o.trigs) != 1 || tab.st.changeCount() != 0 {
		t.Fatalf("trigs=%+v changes=%d", o.trigs, tab.st.changeCount())
	}

	// 刚点「添加」的空行不算改动；填了才算
	tab.addTrigger()
	if tab.st.changeCount() != 0 {
		t.Fatal("空白新行不应算改动")
	}
	n := &o.trigs[len(o.trigs)-1]
	n.Name, n.Timing, n.Event, n.Definition = "trg_new", "BEFORE", "DELETE", "INSERT INTO audit VALUES ('x')"
	// 已有的触发器改语句
	o.trigs[0].Definition = strings.Replace(o.trigs[0].Definition, "'old'", "'changed'", 1)
	if got := tab.st.changeCount(); got != 2 {
		t.Fatalf("changes = %d, want 2 (新增 + 修改)", got)
	}
	tab.st.selTrig = 0
	e.tt.Frame()
	e.has("CREATE TRIGGER") // 右侧编辑器显示选中触发器的完整语句
	e.shot("20-structure-objects")

	tab.buildStructurePreview()
	joined := strings.Join(tab.st.preview.stmts, "\n")
	if !strings.Contains(joined, "DROP TRIGGER") || !strings.Contains(joined, "trg_new") || !strings.Contains(joined, "'changed'") {
		t.Fatalf("preview: err=%q\n%s", tab.st.preview.err, joined)
	}
	tab.commitStructure()
	if tab.st.preview.err != "" {
		t.Fatal(tab.st.preview.err)
	}
	e.tt.Frame()
	if got := queryCount(t, e, `SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND tbl_name='orders'`); got != "2" {
		t.Fatalf("提交后触发器数 = %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM sqlite_master WHERE name='trg_old' AND sql LIKE '%changed%'`); got != "1" {
		t.Fatalf("trg_old 应已改成新语句: %s", got)
	}
	if len(o.trigs) != 2 || tab.st.changeCount() != 0 {
		t.Fatalf("提交后应重新载入: %+v", o.trigs)
	}

	// 删除：先标成待删除（可撤销），提交才真删
	for i := range o.trigs {
		if o.trigs[i].Name == "trg_old" {
			tab.removeTrigger(i)
		}
	}
	if tab.st.changeCount() != 1 {
		t.Fatalf("待删除应算 1 项改动: %d", tab.st.changeCount())
	}
	tab.commitStructure()
	e.tt.Frame()
	if len(o.trigs) != 1 || o.trigs[0].Name != "trg_new" {
		t.Fatalf("删除后: %+v", o.trigs)
	}
}

// 已有索引能就地改名、改列、改唯一：提交时先删旧的再建新的。
func TestStructureIndexInlineModify(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 1
	tab.modeChanged()
	st := &tab.st
	if len(st.idx) != 1 || st.changeCount() != 0 {
		t.Fatalf("idx=%+v changes=%d", st.idx, st.changeCount())
	}
	st.idx[0].Name, st.idx[0].IsUnique, st.idx[0].Columns = "idx_users_nm", true, []string{"name", "age"}
	if got := st.changeCount(); got != 1 {
		t.Fatalf("changes = %d, want 1", got)
	}
	if st.workingIndexes()[0].Status != "modified" {
		t.Fatalf("status = %q", st.workingIndexes()[0].Status)
	}
	// 重置后回到原样，且没有改到原始数据
	st.reset()
	if st.idx[0].Name != "idx_users_name" || len(st.idx[0].Columns) != 1 || st.changeCount() != 0 {
		t.Fatalf("reset 后: %+v", st.idx[0])
	}
	st.idx[0].Name, st.idx[0].IsUnique, st.idx[0].Columns = "idx_users_nm", true, []string{"name", "age"}
	tab.commitStructure()
	if st.preview.err != "" {
		t.Fatal(st.preview.err)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_list('users') WHERE name='idx_users_name'`); got != "0" {
		t.Fatalf("旧索引应已删除: %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_list('users') WHERE name='idx_users_nm' AND "unique"=1`); got != "1" {
		t.Fatalf("新索引应是唯一的: %s", got)
	}
	if got := queryCount(t, e, `SELECT COUNT(*) FROM pragma_index_info('idx_users_nm')`); got != "2" {
		t.Fatalf("新索引应有 2 列: %s", got)
	}
}

// SQLite 不能给已有表加外键：按钮置灰，也不会生成语句。
func TestStructureForeignKeysUnsupportedOnSQLite(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	e.tt.Frame()
	tab := w.tabs[0].(*tableTab)
	tab.mode = 1
	tab.modeChanged()
	e.has("外键")
	if canEditFK("sqlite") {
		t.Fatal("SQLite 不应允许改外键")
	}
	if _, err := app.BuildAddForeignKeySQL("sqlite", "orders", model.ForeignKeyInfo{Name: "x", Columns: []string{"a"}, RefTable: "t", RefColumns: []string{"a"}}); err == nil {
		t.Fatal("应报错")
	}
}

// 视图：侧栏分区、定义标签里改写并应用、删除。
func TestViewDefinitionEditAndDrop(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	execSQL(t, e, w, `CREATE VIEW v_adults AS SELECT id, name FROM users WHERE age >= 30`)
	w.loadTables()
	e.tt.Frame()
	e.has("v_adults")
	if !w.isViewName("v_adults") {
		t.Fatal("v_adults 应是视图")
	}

	w.openDefinition("view", "v_adults")
	e.tt.Frame()
	d := w.tabs[len(w.tabs)-1].(*defTab)
	if !strings.Contains(d.ed.Buffer.String(), "CREATE VIEW") || d.pending() != 0 {
		t.Fatalf("定义未载入: %q", d.ed.Buffer.String())
	}
	d.ed.Buffer.Set(strings.Replace(d.ed.Buffer.String(), "age >= 30", "age >= 50", 1))
	if d.pending() != 1 {
		t.Fatal("改动后应有 1 项待应用")
	}
	e.shot("21-view-definition")
	d.apply()
	e.tt.Frame()
	if d.pending() != 0 || d.err != "" {
		t.Fatalf("应用失败: %q", d.err)
	}
	got, _ := e.sh.app.Databases.GetDDL(w.cfg.ID, w.db, "v_adults")
	if !strings.Contains(got, "age >= 50") {
		t.Fatalf("视图没有被替换: %s", got)
	}

	// 写错的定义：应用失败，旧视图还在
	d.ed.Buffer.Set(`CREATE VIEW v_adults AS SELEKT 1`)
	d.apply()
	if d.err == "" {
		t.Fatal("错误的定义应报错")
	}
	if got, _ = e.sh.app.Databases.GetDDL(w.cfg.ID, w.db, "v_adults"); !strings.Contains(got, "age >= 50") {
		t.Fatalf("失败后应还原旧视图: %q", got)
	}

	d.ed.Buffer.Set(d.orig)
	w.dropObject("view", "v_adults")
	if !e.sh.confirm.open {
		t.Fatal("删除视图应先确认")
	}
	e.sh.confirm.action()
	e.tt.Frame()
	if w.isViewName("v_adults") {
		t.Fatal("视图应已删除")
	}
	for _, tb := range w.tabs {
		if dd, ok := tb.(*defTab); ok && dd == d {
			t.Fatal("视图的定义标签应随之关闭")
		}
	}
}

// 克隆表：提示框里默认名为 users_copy，勾选后连数据一起复制。
func TestCloneTablePrompt(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.cloneTable("users")
	e.tt.Frame()
	if !e.sh.prm.open || e.sh.prm.value != "users_copy" {
		t.Fatalf("提示框: %+v", e.sh.prm)
	}
	e.has("同时复制数据")
	e.sh.prm.check = true
	if err := e.tt.Click("克隆"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if got := queryCount(t, e, `SELECT COUNT(*) FROM users_copy`); got != "450" {
		t.Fatalf("克隆的行数 = %s", got)
	}
	found := false
	for _, tb := range w.tables {
		found = found || tb.Name == "users_copy"
	}
	if !found {
		t.Fatal("侧栏应刷新出 users_copy")
	}
}

// 库内搜索：命中的表按行数列出，点一下打开该表并带上“任意列”过滤。
func TestSearchInDatabase(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openDatabaseSearch()
	e.sh.srch.term = "user7@example.com"
	e.sh.runDatabaseSearch()
	e.tt.Frame()
	if len(e.sh.srch.hits) != 1 || e.sh.srch.hits[0].Table != "users" || e.sh.srch.hits[0].Count != 1 {
		t.Fatalf("hits=%+v err=%q", e.sh.srch.hits, e.sh.srch.err)
	}
	e.has("1 行")
	e.shot("22-search-database")
	if err := e.tt.Click("users"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	tt, ok := w.tabs[w.active].(*tableTab)
	if !ok || tt.table != "users" || len(tt.applied) != 1 || tt.applied[0].Column != "__any" {
		t.Fatalf("应打开 users 并带过滤: %+v", w.tabs)
	}
	if len(tt.g.rows) != 1 {
		t.Fatalf("过滤后应只剩 1 行，实际 %d", len(tt.g.rows))
	}

	e.sh.openDatabaseSearch()
	e.sh.srch.term = "nothing-like-this"
	e.sh.runDatabaseSearch()
	e.has("没有表包含")
}

// SQLite 的数据库就是文件：新建、重命名、删除都不提供。
func TestDatabaseLevelOperationsNeedServer(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.newDatabase(w)
	if e.sh.prm.open {
		t.Fatal("SQLite 不应弹出新建数据库")
	}
}

// 新建数据库的提示框：MySQL 带字符集，PG 没有。
func TestNewDatabasePromptCharset(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.cfg.Type = "mysql"
	e.sh.newDatabase(w)
	if !e.sh.prm.open || len(e.sh.prm.options) == 0 || e.sh.prm.sel != "utf8mb4" {
		t.Fatalf("MySQL 应有字符集选择: %+v", e.sh.prm)
	}
	e.sh.prm = promptDialog{}
	w.cfg.Type = "postgres"
	e.sh.newDatabase(w)
	if !e.sh.prm.open || len(e.sh.prm.options) != 0 {
		t.Fatalf("PG 没有字符集选择: %+v", e.sh.prm)
	}
	e.sh.prm = promptDialog{}
	w.cfg.ReadOnly = true
	e.sh.newDatabase(w)
	if e.sh.prm.open {
		t.Fatal("只读连接不应新建数据库")
	}
}

// 外键页：SQLite 里已有的外键照常列出（只读）；工作副本的状态流转（新增、改、删、重置）正确。
func TestStructureForeignKeyWorkingCopy(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	execSQL(t, e, w, `CREATE TABLE child (id INTEGER PRIMARY KEY, uid INTEGER, FOREIGN KEY (uid) REFERENCES users(id) ON DELETE CASCADE)`)
	w.loadTables()
	w.openTable("child")
	e.tt.Frame()
	tab := w.tabs[0].(*tableTab)
	tab.mode = 1
	tab.modeChanged()
	tab.st.sub = 1
	e.tt.Frame()
	o := &tab.st.obj
	if len(o.fks) != 1 || o.fks[0].RefTable != "users" || o.fks[0].OnDelete != "CASCADE" {
		t.Fatalf("fks=%+v", o.fks)
	}
	e.has("users")
	e.shot("23-structure-fk")
	if tab.st.changeCount() != 0 {
		t.Fatal("刚载入不应有改动")
	}

	// 改已有的外键 → modified；改回去 → 无改动
	o.fks[0].OnDelete = "RESTRICT"
	if got := o.workingFKs("child")[0].Status; got != "modified" {
		t.Fatalf("status = %q", got)
	}
	o.fks[0].OnDelete = "CASCADE"
	if tab.st.changeCount() != 0 {
		t.Fatal("改回原值后不应算改动")
	}
	// 新增的空行不算；填了名字留空也会取默认名
	tab.addFK()
	if tab.st.changeCount() != 0 {
		t.Fatal("空白新行不应算改动")
	}
	n := &o.fks[len(o.fks)-1]
	n.Columns, n.RefTable, n.RefColumns = []string{"uid"}, "users", []string{"id"}
	if got := o.workingFKs("child")[1]; got.Status != "new" || got.Name != "fk_child_uid" {
		t.Fatalf("new fk = %+v", got)
	}
	// 删除已有的 → deleted，再点一次撤销；新增的直接去掉
	tab.st.selFK = 0
	if canEditFK("sqlite") {
		t.Fatal("SQLite 不应允许改外键")
	}
	o.fkDel[o.fks[0].UID] = true
	if got := o.workingFKs("child")[0].Status; got != "deleted" {
		t.Fatalf("status = %q", got)
	}
	tab.st.reset()
	if len(o.fks) != 1 || tab.st.changeCount() != 0 {
		t.Fatalf("重置后应回到原样: %+v", o.fks)
	}
}
