package ui

import (
	mui "github.com/egoist/mygo/ui"

	"minidb/locales"
	"os"
	"testing"
)

// @author ixxxxoooo
//
// 第五轮改动的对照截图（设计稿 docs/design/v2 第五轮）：筛选芯片、标签列表、菜单、监控、用户、AI 助手等。
// 只在设置了 MINIDB_SHOTS 时运行，输出 r5-xxx.png；用法见 CLAUDE.md「测试」一节。

// TestShotsRound5Tabs 渲染筛选芯片和「已打开的标签」列表。
func TestShotsRound5Tabs(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	e.tt.Frame()
	tab := w.tabs[w.active].(*tableTab)
	tab.filters = []filterRow{{on: true, col: "total", op: ">", val: "30"}, {on: true, col: "status", op: "=", val: "paid"}}
	tab.applyFilter()
	e.shot("r5-chips")
	for i := 0; i < 8; i++ {
		w.newQuery("SELECT " + itoa(i))
	}
	w.openTable("v_daily_sales")
	w.openTable("users")
	for i := 0; i < 4; i++ {
		e.tt.Frame()
	}
	if err := e.tt.Click("已打开的标签"); err != nil {
		t.Fatal(err)
	}
	e.shot("r5-tabs-list")
}

// TestShotsRound5Menus 渲染「更多」菜单、安全级别子菜单和盾牌下拉。
func TestShotsRound5Menus(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	openWorkspace(e)
	e.tt.Frame()
	if err := e.tt.Click(locales.ConnMore()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.shot("r5-more-menu")
	t.Log(e.tt.Menu())
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if err := e.tt.Click(locales.SafeSilent()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.shot("r5-safe-pop")
}
