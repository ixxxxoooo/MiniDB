package ui

import (
	"os"
	"testing"

	"minidb/internal/app"
)

// @author ixxxxoooo
//
// 备份、恢复、用户与权限的设计稿对照截图：输出 d-backup / d-restore / d-users。

func TestShotsDesignDumpUsers(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	e.sh.bak = dumpDialog{open: true, w: w, prefs: e.sh.prefs.Dump}
	e.tt.Frame()
	e.shot("d-backup")
	e.sh.bak.open = false
	e.sh.rst = restoreDialog{open: true, w: w}
	e.tt.Frame()
	e.shot("d-restore")
	e.sh.rst.open = false
	e.sh.usr = usersDialog{open: true, w: w, grantOf: "root@localhost",
		users:  []app.UserInfo{{Name: "root", Host: "localhost", Superuser: true}, {Name: "app", Host: "%"}, {Name: "report", Host: "10.0.%"}},
		grants: []string{"GRANT ALL PRIVILEGES ON *.* TO `root`@`localhost` WITH GRANT OPTION"}}
	e.tt.Frame()
	e.shot("d-users")
}
