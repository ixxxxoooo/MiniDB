package ui

import (
	"minidb/internal/app"
	"strings"
	"testing"
)

func TestUserEditorStatements(t *testing.T) {
	// 新建 MySQL 用户：创建 + 全局权限 + 库级权限
	e := userEditor{isNew: true, name: "app", host: "10.%", password: "s3cret", priv: map[string]bool{}, scopeDB: "shop"}
	e.priv["g:PROCESS"] = true
	e.priv["s:SELECT"] = true
	e.priv["s:INSERT"] = true
	e.priv["s:SHUTDOWN"] = true // 库级没有这项，应被忽略
	e.limits[0] = "100"
	got, err := e.statements("mysql", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CREATE USER 'app'@'10.%' IDENTIFIED BY 's3cret' WITH MAX_QUERIES_PER_HOUR 100",
		"GRANT PROCESS ON *.* TO 'app'@'10.%'",
		"GRANT SELECT, INSERT ON `shop`.* TO 'app'@'10.%'",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
	masked, _ := e.statements("mysql", true)
	if strings.Contains(strings.Join(masked, ""), "s3cret") || !strings.Contains(masked[0], "'***'") {
		t.Fatalf("预览不应显示密码: %q", masked)
	}

	// 修改：没改密码就不 ALTER；选择收回
	e = userEditor{orig: app.UserInfo{Name: "app", Host: "h"}, name: "app", host: "h", priv: map[string]bool{}, revoke: true}
	e.priv["g:SELECT"] = true
	got, _ = e.statements("mysql", false)
	if len(got) != 1 || got[0] != "REVOKE SELECT ON *.* FROM 'app'@'h'" {
		t.Fatalf("got %q", got)
	}
	e.priv = map[string]bool{"g:ALL PRIVILEGES": true}
	e.revoke = false
	got, _ = e.statements("mysql", false)
	if got[0] != "GRANT ALL PRIVILEGES ON *.* TO 'app'@'h'" {
		t.Fatalf("got %q", got)
	}
	e.limits[1] = "x"
	if _, err := e.statements("mysql", false); err == nil {
		t.Fatal("限制不是数字应报错")
	}
	if _, err := (&userEditor{}).statements("mysql", false); err == nil {
		t.Fatal("没有用户名应报错")
	}

	// PostgreSQL：只写改动过的属性
	e = userEditor{orig: app.UserInfo{Name: "r", CanLogin: true}, name: "r", login: true, createDB: true, priv: map[string]bool{},
		scopeDB: "app", schema: "public", table: "t"}
	e.priv["s:SELECT"] = true
	got, _ = e.statements("postgres", false)
	if len(got) != 2 || got[0] != `ALTER ROLE "r" WITH CREATEDB` || got[1] != `GRANT SELECT ON TABLE "public"."t" TO "r"` {
		t.Fatalf("got %q", got)
	}
}

func usersPageEnv(t *testing.T) (*env, *Workspace) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.usr = usersDialog{open: true, w: w, grantOf: "app@%",
		users: []app.UserInfo{{Name: "root", Host: "localhost", Superuser: true}, {Name: "app", Host: "%"}, {Name: "report", Host: "10.0.%"}},
		sel:   1,
		grants: []string{
			"GRANT USAGE ON *.* TO `app`@`%`",
			"GRANT SELECT, INSERT, UPDATE, DELETE, INDEX ON `shop`.* TO `app`@`%`",
			"GRANT SELECT, UPDATE ON `shop`.`orders` TO `app`@`%` WITH GRANT OPTION",
			"GRANT PROCESS ON *.* TO `app`@`%`"}}
	w.showUsersTab()
	e.tt.Frame()
	e.tt.Frame()
	return e, w
}

// 「用户与权限」是工作区里的一个标签：有账号列表、选中账号的页头和只读的授权矩阵。
func TestUsersPageShowsMatrix(t *testing.T) {
	e, w := usersPageEnv(t)
	if !w.onUsersTab() {
		t.Fatal("showUsersTab 应把标签切到前台")
	}
	e.has("新建用户")
	e.has("shop.orders") // 矩阵里的对象
	e.has("PROCESS")     // 矩阵列之外的权限进「其他」
	e.has("超级用户")
	e.shot("53-users")
	e.sh.usr.pageTab = 1
	e.tt.Frame()
	e.has("GRANT PROCESS ON")
	e.shot("53-users-sql")
	w.showUsersTab() // 已有就不重复开
	n := 0
	for _, tb := range w.tabs {
		if _, ok := tb.(*usersTab); ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("用户标签应只有一个: %d", n)
	}
}

// 搜索框按名字或主机筛账号，不分大小写。
func TestUsersSearchFilters(t *testing.T) {
	d := usersDialog{users: []app.UserInfo{{Name: "root", Host: "localhost"}, {Name: "App", Host: "%"}, {Name: "report", Host: "10.0.%"}}}
	d.query = "APP"
	if got := d.visibleUsers(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("按名字: %v", got)
	}
	d.query = "10.0"
	if got := d.visibleUsers(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("按主机: %v", got)
	}
	d.query = ""
	if len(d.visibleUsers()) != 3 {
		t.Fatal("空搜索词显示全部")
	}
}

// 新建 / 编辑账号仍是对话框：盖在页面上，返回后回到页面。
func TestUserEditorIsStillADialog(t *testing.T) {
	e, w := usersPageEnv(t)
	e.sh.newUserEditor()
	e.tt.Frame()
	e.tt.Frame()
	if !e.sh.anyModalOpen() {
		t.Fatal("编辑器应是对话框")
	}
	e.has("SQL 预览")
	e.sh.usr.ed.tab = 2
	e.tt.Frame()
	e.shot("54-user-editor")
	e.sh.usr.editing = false
	e.tt.Frame()
	if e.sh.anyModalOpen() || !w.onUsersTab() {
		t.Fatal("关掉编辑器后应回到页面")
	}
}
