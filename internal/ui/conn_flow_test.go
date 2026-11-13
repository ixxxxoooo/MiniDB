package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// @author ixxxxoooo

// click 点屏幕上的文字，找不到就让测试失败。
func (e *env) click(s string) {
	e.t.Helper()
	e.tt.Frame()
	if err := e.tt.Click(s); err != nil {
		e.t.Fatalf("click %q: %v; have %q", s, err, e.tt.Texts())
	}
	e.tt.Frame()
}

// TestConnectionManagerListThenForm 连接管理器是两级：先是已保存连接的列表，
// 列表下方的"再添加新的连接"进入配置页；类型是带图标的下拉；保存、返回、Esc 都回到列表。
func TestConnectionManagerListThenForm(t *testing.T) {
	e := newEnv(t)
	e.sh.newConnectionDialog()
	e.tt.Frame()
	e.tt.Frame()
	e.has("local-sqlite")
	e.has("再添加新的连接")
	e.hasNot("数据库类型") // 配置页的内容不在列表页上
	if e.sh.dlg.form || e.sh.selected != "c1" {
		t.Fatalf("打开后应停在列表页并选中已有连接: form=%v selected=%q", e.sh.dlg.form, e.sh.selected)
	}
	e.shot("90-conn-list")

	e.click("再添加新的连接")
	if !e.sh.dlg.form || !e.sh.dlg.draft || e.sh.dlg.cfg.Type != "mysql" || e.sh.dlg.port != "3306" {
		t.Fatalf("应进入新建配置页: form=%v draft=%v %+v", e.sh.dlg.form, e.sh.dlg.draft, e.sh.dlg.cfg)
	}
	e.has("数据库类型")
	e.hasNot("再添加新的连接")
	e.shot("91-conn-form")

	// 类型是下拉，不是一排卡片：点开后才看得到别的类型
	e.hasNot("StarRocks")
	e.click("MySQL")
	e.has("StarRocks")
	e.shot("92-conn-type-open")
	e.click("PostgreSQL")
	if d := e.sh.dlg; d.cfg.Type != "postgres" || d.port != "5432" || d.cfg.User != "postgres" {
		t.Fatalf("切换类型应同步默认端口和用户: type=%s port=%q user=%q", d.cfg.Type, d.port, d.cfg.User)
	}
	e.hasNot("StarRocks") // 选完下拉收起
	e.shot("93-conn-new-postgres")

	// 改过东西，Esc 先问是否放弃；取消后仍在配置页
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if !e.sh.confirm.open || !e.sh.dlg.form {
		t.Fatalf("有未保存的修改，Esc 应先确认: confirm=%v form=%v", e.sh.confirm.open, e.sh.dlg.form)
	}
	e.shot("94-conn-discard")
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.confirm.open || !e.sh.dlg.form || !e.sh.dlg.open {
		t.Fatalf("取消后应仍在配置页: confirm=%v form=%v open=%v", e.sh.confirm.open, e.sh.dlg.form, e.sh.dlg.open)
	}

	// 保存：回到列表，光标落在刚保存的连接上
	e.sh.dlg.cfg.Name = "pg-main"
	e.sh.saveConnection(false)
	e.tt.Frame()
	e.tt.Frame()
	if !e.sh.dlg.open || e.sh.dlg.form {
		t.Fatalf("保存后应回到列表页: open=%v form=%v", e.sh.dlg.open, e.sh.dlg.form)
	}
	e.has("pg-main")
	if cur := e.sh.selectedConnection(); cur == nil || cur.Name != "pg-main" {
		t.Fatalf("光标应落在刚保存的连接上: %+v", cur)
	}
	e.shot("95-conn-list-after-save")

	// 编辑：进入该连接的配置页，没改动时返回不再确认；返回后 Esc 关闭对话框
	e.sh.editConnection(*e.sh.selectedConnection())
	e.tt.Frame()
	if !e.sh.dlg.form || e.sh.dlg.draft || e.sh.dlg.cfg.Name != "pg-main" {
		t.Fatalf("应进入 pg-main 的配置页: %+v", e.sh.dlg.cfg)
	}
	e.shot("96-conn-edit")
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.confirm.open || e.sh.dlg.form || !e.sh.dlg.open {
		t.Fatalf("没改动时 Esc 应直接回列表: confirm=%v form=%v open=%v", e.sh.confirm.open, e.sh.dlg.form, e.sh.dlg.open)
	}
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.dlg.open {
		t.Fatal("列表页按 Esc 应关闭对话框")
	}
}

// TestConnectionListClickConnects 列表里点一行直接连接并关掉对话框。
func TestConnectionListClickConnects(t *testing.T) {
	e := newEnv(t)
	e.sh.newConnectionDialog()
	e.tt.Frame()
	e.click("local-sqlite")
	if len(e.sh.spaces) != 1 || e.sh.dlg.open {
		t.Fatalf("点一行应连接并关闭: spaces=%d open=%v", len(e.sh.spaces), e.sh.dlg.open)
	}
}

// TestConnectionManagerWithoutConnections 一条连接都没有时直接落在配置页，没有返回键；
// 保存后进入列表页。
func TestConnectionManagerWithoutConnections(t *testing.T) {
	e := newEnv(t)
	if err := e.sh.app.Connections.DeleteConnection("c1"); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.newConnectionDialog()
	e.tt.Frame()
	e.tt.Frame()
	if !e.sh.dlg.form || !e.sh.dlg.draft {
		t.Fatalf("没有连接时应直接进入新建配置页: form=%v draft=%v", e.sh.dlg.form, e.sh.dlg.draft)
	}
	e.hasNot("返回连接列表")
	e.shot("97-conn-first")
	// Esc 直接关闭：没有列表可回
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if e.sh.dlg.open {
		t.Fatal("没有列表可回时 Esc 应关闭对话框")
	}
}

// TestConnectionDeleteFromForm 在配置页里删除连接后回到（可能已空的）列表页。
func TestConnectionDeleteFromForm(t *testing.T) {
	e := newEnv(t)
	e.sh.editConnection(e.sh.conns[0])
	e.tt.Frame()
	e.sh.deleteConnection(e.sh.dlg.cfg)
	e.tt.Frame()
	e.sh.confirm.action() // 确认删除
	e.tt.Frame()
	e.tt.Frame()
	if e.sh.dlg.form || !e.sh.dlg.open || len(e.sh.conns) != 0 {
		t.Fatalf("删除后应回到列表页: form=%v open=%v conns=%d", e.sh.dlg.form, e.sh.dlg.open, len(e.sh.conns))
	}
	e.has("添加第一个连接")
	e.shot("98-conn-empty-list")
}
