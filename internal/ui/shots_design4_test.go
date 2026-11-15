package ui

import (
	"os"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 数据页的设计稿对照截图：列头徽标 + 视图切换（e-grid）、筛选浮层（e-filter）、筛选芯片（e-chips）。

func TestShotsDesignDataPage(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	e.tt.Frame()
	tab := w.tabs[0].(*tableTab)
	e.shot("e-grid")
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	tab.filters = []filterRow{{on: true, col: "total", op: ">", val: "30"}, {on: true, col: "status", op: "=", val: "paid"}}
	e.shot("e-filter")
	tab.showFilter = false
	tab.applyFilter()
	e.tt.Frame()
	e.shot("e-chips")
}

// 标题栏连接胶囊与两张浮层：e-conn（已打开的连接）、e-db（切换数据库）。
func TestShotsDesignConnDB(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.cfg.Tag = "staging"
	w.openTable("orders")
	e.tt.Frame()
	e.sh.openWorkspaces()
	e.shot("e-conn")
	e.sh.wsPop.open = false
	e.sh.openSwitcher()
	w.dbs = []model.DatabaseInfo{{Name: "shop", TableCount: 24, Size: 248 << 20}, {Name: "shop_test", TableCount: 12, Size: 3 << 30}, {Name: "analytics", TableCount: 41, Size: 812 << 20}}
	e.shot("e-db")
}
