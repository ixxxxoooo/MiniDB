package ui

import (
	"strings"
	"testing"

	"minidb/internal/database"
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
	e = userEditor{orig: database.UserInfo{Name: "app", Host: "h"}, name: "app", host: "h", priv: map[string]bool{}, revoke: true}
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
	e = userEditor{orig: database.UserInfo{Name: "r", CanLogin: true}, name: "r", login: true, createDB: true, priv: map[string]bool{},
		scopeDB: "app", schema: "public", table: "t"}
	e.priv["s:SELECT"] = true
	got, _ = e.statements("postgres", false)
	if len(got) != 2 || got[0] != `ALTER ROLE "r" WITH CREATEDB` || got[1] != `GRANT SELECT ON TABLE "public"."t" TO "r"` {
		t.Fatalf("got %q", got)
	}
}

func TestUsersDialogShot(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.usr = usersDialog{open: true, w: w, grantOf: "root@localhost",
		users:  []database.UserInfo{{Name: "root", Host: "localhost", Superuser: true}, {Name: "app", Host: "%"}, {Name: "report", Host: "10.0.%"}},
		grants: []string{"GRANT ALL PRIVILEGES ON *.* TO `root`@`localhost` WITH GRANT OPTION", "GRANT PROXY ON ''@'' TO 'root'@'localhost' WITH GRANT OPTION"}}
	e.tt.Frame()
	e.has("新建用户")
	e.shot("53-users")
	e.sh.usr.editing = true
	e.sh.usr.ed = userEditor{isNew: true, host: "%", name: "analyst", priv: map[string]bool{"s:SELECT": true}, scopeDB: "shop"}
	e.sh.usr.ed.tab = 2
	e.tt.Frame()
	e.has("SQL 预览")
	e.shot("54-user-editor")
}
