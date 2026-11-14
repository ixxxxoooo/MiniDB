package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// @author ixxxxoooo

// 危险确认框底栏有按键提示，⌘⏎ 才会确认；回车落在默认的「取消」上。
func TestConfirmDangerCmdEnter(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	ran := 0
	e.sh.askDanger("删除表 orders？", "将永久删除这张表，无法撤销。", "删除表", func() { ran++ })
	e.tt.Frame()
	e.has("⏎ 取消 · ⌘⏎ 删除表")
	e.shot("p7-confirm-danger")
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if ran != 1 || e.sh.confirm.open {
		t.Fatalf("⌘⏎ 应确认危险操作: ran=%d open=%v", ran, e.sh.confirm.open)
	}
	// 普通确认没有这条提示
	e.sh.askWarn("关闭前要提交修改吗？", "orders 有 3 项未提交的修改。", "丢弃", func() {})
	e.tt.Frame()
	if e.tt.HasText("⏎ 取消 · ⌘⏎ 丢弃") {
		t.Fatal("非危险确认框不该显示 ⌘⏎ 提示")
	}
	e.shot("p7-confirm-warn")
}
