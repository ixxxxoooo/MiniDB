package ui

import (
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 欢迎页（没有任何打开的连接时占满内容区）：卡片、文件夹筛选、列表模式、双击连接。

// welcomeEnv 造一个有两条连接、一个文件夹的欢迎页。
func welcomeEnv(t *testing.T) *env {
	e := newEnv(t)
	s := e.sh
	s.conns = []model.ConnectionConfig{
		{ID: "a", Name: "alpha-conn", Type: "mysql", Host: "10.0.0.1", Port: 3306, Tag: "production", FolderID: "f1"},
		e.cfg,
	}
	s.conns[0].Pinned, s.conns[0].LastUsed = true, time.Now().Add(-2*time.Hour).Unix()
	s.folders = []model.ConnectionFolder{{ID: "f1", Name: "生产环境"}}
	e.tt.Frame()
	return e
}

func TestWelcomePageShowsCardsAndFolders(t *testing.T) {
	e := welcomeEnv(t)
	for _, want := range []string{"连接", "全部连接", "文件夹", "生产环境", "alpha-conn", "local-sqlite", "10.0.0.1:3306", "卡片", "列表", "导入", "新建连接"} {
		e.has(want)
	}
	if e.sh.dlg.open {
		t.Fatal("没有打开的连接时欢迎页自己就是列表，不应再弹对话框")
	}
	e.shot("p6-welcome-cards")
}

func TestWelcomePageSingleClickSelectsDoubleClickConnects(t *testing.T) {
	e := welcomeEnv(t)
	r, ok := e.tt.Find("local-sqlite")
	if !ok {
		t.Fatal("card not found")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	e.tt.ClickAt(x, y)
	e.tt.Frame()
	if e.sh.selected != "c1" || len(e.sh.spaces) != 0 {
		t.Fatalf("单击只选中不连接: selected=%q spaces=%d", e.sh.selected, len(e.sh.spaces))
	}
	e.tt.ClickAt(x, y)
	e.tt.ClickAt(x, y)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 {
		t.Fatalf("双击应连接: spaces=%d", len(e.sh.spaces))
	}
}

func TestWelcomePageFolderFilterAndListMode(t *testing.T) {
	e := welcomeEnv(t)
	e.click("生产环境") // 分类栏里的文件夹
	e.has("alpha-conn")
	e.hasNot("local-sqlite")
	e.click("全部连接")
	e.has("local-sqlite")

	e.click("列表")
	if !e.sh.layout.ConnList {
		t.Fatal("切到列表后应记进布局状态")
	}
	e.has("alpha-conn")
	e.shot("p6-welcome-list")
	e.click("卡片")
	if e.sh.layout.ConnList {
		t.Fatal("切回卡片")
	}
}

func TestWelcomePageSearchNarrowsAndEnterConnects(t *testing.T) {
	e := welcomeEnv(t)
	e.sh.dlg.search = "local"
	e.has("local-sqlite")
	e.hasNot("alpha-conn")
	e.sh.selected = "c1"
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 {
		t.Fatalf("⏎ 应连接光标所在的连接: spaces=%d", len(e.sh.spaces))
	}
}

func TestWelcomePageEmptyState(t *testing.T) {
	e := newEnv(t)
	e.sh.conns = nil
	e.has("还没有保存的连接")
}
