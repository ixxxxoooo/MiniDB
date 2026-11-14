package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func openWorkspace(e *env) *Workspace {
	e.sh.openConnection(e.sh.conns[0])
	return e.sh.spaces[0]
}

func TestQueryRunCurrentAndAll(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT name FROM users WHERE id = 1;\n\nSELECT COUNT(*) AS n FROM users")
	e.tt.Frame()

	// caret on the first statement: run current
	q.ed.Select(5, 5)
	q.run(false)
	e.has("用户001")
	if len(q.results) != 1 {
		t.Fatalf("results=%d", len(q.results))
	}
	e.shot("03-query")

	// run all: two results
	q.run(true)
	if len(q.results) != 2 {
		t.Fatalf("results=%d", len(q.results))
	}
	e.has("450")
	e.has("结果 1")

	// selection wins over the caret statement
	q.ed.Select(0, len("SELECT name FROM users WHERE id = 1"))
	q.run(false)
	if len(q.results) != 1 || len(q.results[0].res.Rows) != 1 {
		t.Fatalf("selection run: %d results", len(q.results))
	}
}

func TestQueryErrorsAndWrites(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELEC nope")
	q.run(false)
	if len(q.results) != 1 || q.results[0].res.Error == "" {
		t.Fatalf("a syntax error must come back as a result error: %+v", q.results)
	}
	e.shot("04-query-error")

	q.ed.Buffer.Set("INSERT INTO orders(user_id,total) VALUES (1, 9.5), (2, 3)")
	q.run(true)
	e.has("影响 2 行")

	q.ed.Buffer.Set("SELECT COUNT(*) AS n FROM orders")
	q.run(true)
	e.has("2")
}

func TestQueryReadOnlyConnection(t *testing.T) {
	e := newEnv(t)
	cfg := e.cfg
	cfg.ReadOnly = true
	if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	w := openWorkspace(e)
	q := w.newQuery("DELETE FROM users")
	q.run(true)
	e.has("只读")
	res, _ := e.sh.app.Queries.ExecuteSQL(e.cfg.ID, "", "SELECT COUNT(*) AS n FROM users")
	if res == nil || len(res.Rows) != 1 || res.Rows[0]["n"] == nil {
		t.Fatalf("users must be intact: %+v", res)
	}
	if v, _ := cellText(res.Rows[0]["n"]); v != "450" {
		t.Fatalf("rows left = %s", v)
	}
}

func TestQueryShortcutsAndHistory(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	e.tt.ClickAt(600, 250) // focus the editor
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if len(q.results) != 1 {
		t.Fatalf("Cmd+Enter did not run: results=%d", len(q.results))
	}
	hist, _ := e.sh.app.History.List(e.cfg.ID, "", 0)
	if len(hist) != 1 || hist[0].SQL != "SELECT 1 AS one" {
		t.Fatalf("history = %+v", hist)
	}
	// 查询器里不再有自己的历史弹窗（⌘Y 已去掉）：历史统一放在侧栏的「历史」页，按天分组
	e.tt.Key(mui.Cmd, mui.KeyY)
	e.tt.Frame()
	e.hasNot("清空历史")
	w.side.seg = sideHistory
	w.loadSideQueries()
	e.tt.Frame()
	e.has("今天")
	e.has("SELECT 1 AS one")
	e.shot("05-history")
}

func TestQueryFormat(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("select id,name from users where id=1")
	q.format()
	if got := q.ed.Buffer.String(); got != "SELECT\n  id,\n  name\nFROM users\nWHERE id = 1" {
		t.Fatalf("formatted = %q", got)
	}
}

// ⌘D / 工具栏星标：收藏光标所在的那一条语句（不是整个脚本），带名字；再按一次取消。
func TestQueryFavoriteTogglesTheStatementAtTheCaret(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one;\n\nSELECT 2 AS two")
	e.tt.Frame()

	q.ed.Select(3, 3) // 第一条
	if _, ok := q.favoriteItem(); ok {
		t.Fatal("nothing is favorited yet")
	}
	q.toggleFavorite()
	if !e.sh.prm.open || e.sh.prm.value != "SELECT 1 AS one" {
		t.Fatalf("a name should be asked for, prefilled from the statement: open=%v value=%q", e.sh.prm.open, e.sh.prm.value)
	}
	e.sh.prm.action("第一条", false, "")
	it, ok := e.sh.app.History.FavoriteOf(e.cfg.ID, "SELECT 1 AS one")
	if !ok || it.Name != "第一条" {
		t.Fatalf("favorite not saved: %+v ok=%v", it, ok)
	}
	if _, ok := e.sh.app.History.FavoriteOf(e.cfg.ID, "SELECT 2 AS two"); ok {
		t.Fatal("only the statement at the caret is favorited, not the whole script")
	}
	if got, ok := q.favoriteItem(); !ok || got.Name != "第一条" {
		t.Fatalf("the star should show as on for the first statement: %+v ok=%v", got, ok)
	}

	// 光标移到第二条：星标熄灭；回到第一条再按一次：取消收藏
	q.ed.Select(len(q.ed.Buffer.String())-2, len(q.ed.Buffer.String())-2)
	if _, ok := q.favoriteItem(); ok {
		t.Fatal("the second statement is not a favorite")
	}
	q.ed.Select(3, 3)
	q.toggleFavorite()
	if _, ok := e.sh.app.History.FavoriteOf(e.cfg.ID, "SELECT 1 AS one"); ok {
		t.Fatal("toggling again removes the favorite")
	}
}

func TestQueryFavoriteOfEmptyEditorWarns(t *testing.T) {
	e := newEnv(t)
	q := openWorkspace(e).newQuery("  ")
	q.toggleFavorite()
	if e.sh.prm.open {
		t.Fatal("an empty editor has nothing to favorite")
	}
}

func TestDefaultFavoriteName(t *testing.T) {
	for in, want := range map[string]string{
		"SELECT 1":                       "SELECT 1",
		"select  a,\n b from t":          "select a,",
		"-- name: 活跃用户\nselect * from u": "活跃用户",
	} {
		if got := defaultFavoriteName(in); got != want {
			t.Errorf("defaultFavoriteName(%q) = %q, want %q", in, got, want)
		}
	}
	long := defaultFavoriteName("SELECT " + string(make([]rune, 0)) + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if len([]rune(long)) != 41 { // 40 个字 + 省略号
		t.Errorf("a long first line is cut to 40 characters: %q (%d)", long, len([]rune(long)))
	}
}

// 没有选中时，格式化只排光标所在的那条语句，其余原样；缩进和关键字大小写按设置。
func TestQueryFormatStatementAtCaretAndOptions(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	src := "select 1;\nselect a,b from t;"
	q := w.newQuery(src)
	e.tt.Frame()
	n := len("select 1;\nsel") // 落在第二条语句里
	q.ed.Select(n, n)
	q.format()
	if got := q.ed.Buffer.String(); got != "select 1;\nSELECT\n  a,\n  b\nFROM t;" {
		t.Fatalf("只应排第二条: %q", got)
	}
	e.sh.prefs.IndentTabs, e.sh.prefs.FormatLower = true, true
	q.ed.Buffer.Set(src)
	q.ed.Select(n, n)
	q.format()
	if got := q.ed.Buffer.String(); got != "select 1;\nselect\n\ta,\n\tb\nfrom t;" {
		t.Fatalf("制表符 + 小写: %q", got)
	}
}

// 结果翻页：没写 LIMIT 的查询一次取一页，上一页 / 下一页只重新执行这一条语句，行号接着数。
func TestQueryResultPaging(t *testing.T) {
	e := newEnv(t)
	e.sh.prefs.RowLimit = 100
	w := openWorkspace(e)
	q := w.newQuery("SELECT id, name FROM users ORDER BY id")
	q.run(false)
	e.tt.Frame()
	r := q.results[0]
	if len(r.res.Rows) != 100 || !r.paged() || r.pages() != 5 || r.page != 1 {
		t.Fatalf("第一页: rows=%d paged=%v pages=%d page=%d total=%d", len(r.res.Rows), r.paged(), r.pages(), r.page, r.res.Total)
	}
	e.has("第 1–100 行，共 450 行")
	e.shot("28-query-paging")

	q.gotoPage(r, 2, 0)
	e.tt.Frame()
	if r.page != 2 || len(r.g.rows) != 100 || plainText(r.g.rows[0]["id"]) != "101" || r.g.offset != 100 {
		t.Fatalf("第二页: page=%d rows=%d first=%v offset=%d", r.page, len(r.g.rows), r.g.rows[0]["id"], r.g.offset)
	}
	e.has("第 101–200 行，共 450 行")

	q.gotoPage(r, 5, 0) // 最后一页只有 50 行
	e.tt.Frame()
	if len(r.g.rows) != 50 {
		t.Fatalf("最后一页应有 50 行: %d", len(r.g.rows))
	}
	e.has("第 401–450 行，共 450 行")

	// 把每页行数改大：一页装得下就不再分页
	q.gotoPage(r, 1, 500)
	e.tt.Frame()
	if len(r.g.rows) != 450 || r.paged() {
		t.Fatalf("每页 500 行时应一页取完: rows=%d paged=%v", len(r.g.rows), r.paged())
	}
	// 控制台日志记下了翻页执行的语句
	found := false
	for _, en := range e.sh.console {
		found = found || strings.Contains(en.sql, "LIMIT 100 OFFSET 100")
	}
	if !found {
		t.Fatal("日志里应有第二页的 LIMIT 100 OFFSET 100")
	}
}

// 分页结果的表头排序是对整个结果排（由数据库做），不是只排当前这一页：
// 按 id 降序，第 1 页应当是 450..351，而不是原来这一页里的 100..1。
func TestQueryResultPagedSortIsGlobal(t *testing.T) {
	e := newEnv(t)
	e.sh.prefs.RowLimit = 100
	w := openWorkspace(e)
	q := w.newQuery("SELECT id, name FROM users")
	q.run(false)
	e.tt.Frame()
	r := q.results[0]
	if !r.paged() {
		t.Fatal("前提：结果应分页")
	}
	r.g.sort = mui.SortOrder{Column: "id", Descending: true} // 模拟点了表头
	q.sortPaged(r)
	e.tt.Frame()
	if got := plainText(r.g.rows[0]["id"]); got != "450" || len(r.g.rows) != 100 || r.page != 1 {
		t.Fatalf("降序后第 1 页首行应是 450: first=%s rows=%d page=%d", got, len(r.g.rows), r.page)
	}
	q.gotoPage(r, 2, 0) // 翻页沿用排序
	e.tt.Frame()
	if got := plainText(r.g.rows[0]["id"]); got != "350" {
		t.Fatalf("第 2 页首行应是 350: %s", got)
	}
	if r.g.sort.Column != "id" || !r.g.sort.Descending {
		t.Fatalf("表头应保持降序: %+v", r.g.sort)
	}
}
