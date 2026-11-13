package ui

import (
	"strings"
	"testing"

	"minidb/internal/database"
)

func execSQL(t *testing.T, e *env, w *Workspace, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if err := e.sh.app.Databases.ExecuteRawSQL(w.cfg.ID, w.db, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// 结构视图能列出触发器，表单生成的 SQL 能建出新触发器，删除也走确认。
func TestStructureTriggersAddAndDrop(t *testing.T) {
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
	if len(tab.st.obj.trigs) != 1 {
		t.Fatalf("trigs=%+v", tab.st.obj.trigs)
	}
	e.shot("20-structure-objects")

	// 添加：表单里填的内容要出现在预览 SQL 里
	tab.st.obj.newTrig = trigForm{open: true, name: "trg_new", timing: "BEFORE", event: "DELETE", body: "INSERT INTO audit VALUES ('x')"}
	e.has("将执行的 SQL")
	e.has("CREATE TRIGGER")
	stmt, err := database.BuildCreateTriggerSQL("sqlite", "orders", tab.triggerFromForm())
	if err != nil {
		t.Fatal(err)
	}
	tab.st.obj.newTrig.open = false
	tab.runObjectSQL(stmt, false)
	e.tt.Frame()
	if len(tab.st.obj.trigs) != 2 {
		t.Fatalf("添加后应有 2 个触发器: %+v", tab.st.obj.trigs)
	}

	// 删除：先确认
	tab.dropTrigger("trg_old")
	if !e.sh.confirm.open {
		t.Fatal("删除触发器应先确认")
	}
	e.sh.confirm.action()
	e.tt.Frame()
	if len(tab.st.obj.trigs) != 1 || tab.st.obj.trigs[0].Name != "trg_new" {
		t.Fatalf("删除后: %+v", tab.st.obj.trigs)
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
	if _, err := database.BuildAddForeignKeySQL("sqlite", "orders", database.ForeignKeyInfo{Name: "x", Columns: []string{"a"}, RefTable: "t", RefColumns: []string{"a"}}); err == nil {
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
