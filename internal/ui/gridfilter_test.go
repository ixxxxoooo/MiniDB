package ui

import (
	"database/sql"
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// ⇧ 点击表头追加排序列：多列排序按点击顺序生成 ORDER BY，其余列的表头带序号。
func TestMultiColumnSort(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)

	tt.addSort(mui.SortOrder{Column: "age", Descending: true}, false)
	tt.addSort(mui.SortOrder{Column: "name"}, true)
	if len(tt.sorts) != 2 || tt.sorts[0].Column != "age" || tt.sorts[1].Column != "name" {
		t.Fatalf("sorts = %+v", tt.sorts)
	}
	marks := tt.sortMarks()
	if marks["age"] != (sortMark{desc: true, n: 1}) || marks["name"] != (sortMark{n: 2}) {
		t.Fatalf("marks = %v", marks)
	}
	// 再点同一列只改方向，不重复追加
	tt.addSort(mui.SortOrder{Column: "name", Descending: true}, true)
	if len(tt.sorts) != 2 || tt.sorts[1].Direction != "DESC" {
		t.Fatalf("sorts = %+v", tt.sorts)
	}
	tt.addSort(mui.SortOrder{Column: "name"}, true)
	if !strings.Contains(tt.tableSelectSQL(), "ORDER BY `age` DESC, `name` ASC") && !strings.Contains(tt.tableSelectSQL(), `ORDER BY "age" DESC, "name" ASC`) {
		t.Fatalf("SQL = %s", tt.tableSelectSQL())
	}

	tt.g.marks = marks
	tt.load()
	e.tt.Frame()
	if got := tt.g.rows[0]["name"]; got != "用户039" {
		t.Fatalf("age 降序再按名字升序，第一行应是 用户039，得到 %v", got)
	}

	// 不按 ⇧ 的点击回到单列排序
	tt.addSort(mui.SortOrder{Column: "id"}, false)
	if len(tt.sorts) != 1 || tt.sortMarks() != nil {
		t.Fatalf("单击应重置为单列: %+v", tt.sorts)
	}
}

// 过滤条件可以用 OR 连接。
func TestFilterJoinOr(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	tt.filters = []filterRow{
		{on: true, col: "name", op: "=", val: "用户001"},
		{on: true, col: "name", op: "=", val: "用户002"},
	}
	tt.setFilter()
	tt.load()
	if len(tt.g.rows) != 0 {
		t.Fatalf("AND 连接两个不同的名字应无结果: %d", len(tt.g.rows))
	}
	tt.filterOr = true
	tt.setFilter()
	tt.load()
	if len(tt.g.rows) != 2 {
		t.Fatalf("OR 连接应有 2 行: %d", len(tt.g.rows))
	}
	if !strings.Contains(tt.tableSelectSQL(), " OR ") {
		t.Fatalf("SQL = %s", tt.tableSelectSQL())
	}
}

// 过滤栏里 ⌘I 增加条件（而不是打开 AI 面板），⌘⇧I 删除，⌘B 勾选。
func TestFilterBarShortcuts(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	tt.toggleFilter()
	e.tt.Frame()
	e.tt.Frame()
	if !tt.filterFocused {
		t.Fatal("打开过滤栏后输入框应获得焦点")
	}
	e.tt.Key(mui.Cmd, mui.KeyI)
	e.tt.Frame()
	if len(tt.filters) != 2 {
		t.Fatalf("⌘I 应增加一行: %d", len(tt.filters))
	}
	if w.panel == 2 {
		t.Fatal("在过滤栏里 ⌘I 不应打开 AI 面板")
	}
	w.cfg.Tag = "staging"
	e.shot("96-filterbar")
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyI)
	e.tt.Frame()
	if len(tt.filters) != 1 {
		t.Fatalf("⌘⇧I 应删除一行: %d", len(tt.filters))
	}
	on := tt.filters[0].on
	e.tt.Key(mui.Cmd, mui.KeyB)
	e.tt.Frame()
	if tt.filters[0].on == on {
		t.Fatal("⌘B 应切换勾选")
	}
}

// 外键单元格可以跳到被引用的表，并只显示对应的那一行。
func TestFollowForeignKey(t *testing.T) {
	e := newEnv(t)
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`,
		`INSERT INTO items(user_id) VALUES (7), (NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	w := openWorkspace(e)
	w.loadTables()
	w.openTable("items")
	e.tt.Frame()
	e.tt.Frame()
	items := w.tabs[0].(*tableTab)
	if got := items.foreignKey("user_id"); got != "users.id" {
		t.Fatalf("foreignKey = %q", got)
	}
	if len(items.g.rows) != 2 {
		t.Fatalf("rows = %d", len(items.g.rows))
	}

	items.followForeignKey(1, "user_id") // NULL：不跳
	if len(w.tabs) != 1 {
		t.Fatal("NULL 不应跳转")
	}
	items.followForeignKey(0, "user_id")
	e.tt.Frame()
	if len(w.tabs) != 2 {
		t.Fatalf("应打开 users 标签: %v", tableTitles(w))
	}
	users, ok := w.tabs[w.active].(*tableTab)
	if !ok || users.table != "users" {
		t.Fatal("活动标签应是 users")
	}
	if len(users.g.rows) != 1 {
		t.Fatalf("users 应只剩 1 行: %d", len(users.g.rows))
	}
	if v := users.g.rows[0]["id"]; v == nil {
		t.Fatal("缺少 id")
	}
	if !users.showFilter || users.filters[0].col != "id" || users.filters[0].val != "7" {
		t.Fatalf("过滤栏应显示 id = 7: %+v", users.filters)
	}
}
