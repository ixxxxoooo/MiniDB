package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 标题栏连接胶囊、已打开的连接浮层、侧栏的库名与切库浮层、搜索图标。

// 标题栏上有连接胶囊（连接名 + 已打开数），搜索只剩一枚图标；点胶囊弹出「已打开的连接 · N」。
func TestTitleBarConnChipOpensPopover(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if !e.tt.HasText(w.cfg.Name) {
		t.Fatalf("标题栏应有连接名 %q: %v", w.cfg.Name, e.tt.Texts())
	}
	if e.tt.HasText(locales.TitlebarSearch()) {
		t.Fatal("标题栏的搜索只应是图标，不该再有「搜索…」文字框")
	}
	if _, ok := e.tt.Find(locales.ToolbarSearch() + e.sh.keyHint("palette")); !ok {
		t.Fatal("应有搜索图标按钮")
	}
	r, ok := e.tt.Find(w.cfg.Name)
	if !ok {
		t.Fatal("找不到连接胶囊")
	}
	e.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	e.tt.Frame()
	e.tt.Frame()
	if !e.sh.wsPop.open {
		t.Fatal("点连接胶囊应弹出已打开的连接")
	}
	for _, want := range []string{locales.WsOpenedN(1), locales.WsOpenConn(), locales.WsNewConnection()} {
		if !e.tt.HasText(want) {
			t.Fatalf("浮层里应有 %q: %v", want, e.tt.Texts())
		}
	}
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.wsPop.open {
		t.Fatal("Esc 应收起浮层")
	}
}

// 浮层里每行的 × 断开这个连接。
func TestConnPopoverCloseButtonDisconnects(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openWorkspaces()
	e.tt.Frame()
	e.tt.Frame()
	if err := e.tt.Click(locales.WsDisconnectNamed(w.cfg.Name)); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(e.sh.spaces) != 0 {
		t.Fatalf("点 × 应断开连接: %d 个", len(e.sh.spaces))
	}
}

// 侧栏顶部是当前库一行（库名 + 上下箭头 + 新建库 + 更多），点库名弹出切库浮层：搜索框 + 库列表 + 新建数据库。
func TestSidebarDBHeaderOpensDBPopover(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.dbs = []model.DatabaseInfo{{Name: "main", TableCount: 3, Size: 3 << 20}, {Name: "other", TableCount: 7}}
	e.tt.Frame()
	if _, ok := e.tt.Find(locales.DbNew()); !ok && e.sh.sw.open {
		t.Fatal("切库浮层不该一开始就开着")
	}
	r, ok := e.tt.Find(w.dbLabel())
	if !ok {
		t.Fatalf("侧栏顶部应有库名 %q: %v", w.dbLabel(), e.tt.Texts())
	}
	e.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	e.tt.Frame()
	e.tt.Frame()
	if !e.sh.sw.open {
		t.Fatal("点库名应打开切库浮层")
	}
	if !e.tt.HasText(locales.DbSwitchSearch()) && !strings.Contains(strings.Join(e.tt.Texts(), "|"), "main") {
		t.Fatalf("浮层里应有搜索框和库列表: %v", e.tt.Texts())
	}
	if got := dbSub(model.DatabaseInfo{Name: "main", TableCount: 3, Size: 3 << 20}); !strings.Contains(got, "3") || !strings.Contains(got, "MB") {
		t.Fatalf("库的第二行应是「大小 · 表数」: %q", got)
	}
	if got := dbSub(model.DatabaseInfo{Name: "other", TableCount: 7}); got != locales.DbSwitchTables(7) {
		t.Fatalf("没有大小时只写表数: %q", got)
	}
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.sw.open {
		t.Fatal("Esc 应收起切库浮层")
	}
}
