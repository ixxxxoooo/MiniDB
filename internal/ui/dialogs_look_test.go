package ui

import "testing"

// @author ixxxxoooo

// 安全模式的确认发送带等宽代码框，密码框出错时输入框变红并给出提示，没有标签时空态有「新建查询」和「搜索」。
func TestSafeDialogsAndEmptyState(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.tt.Frame()
	e.has("搜索")
	e.has("新建查询")
	e.shot("p7-empty-state")

	e.sh.askWarnCode("在 shop-prod 执行写操作", "连接 shop-prod（提醒 2）将发送 1 条语句", "UPDATE orders SET status = 'shipped'\nWHERE id IN (10230, 10229);", "发送", func() {})
	e.tt.Frame()
	e.has("UPDATE orders SET status = 'shipped'\nWHERE id IN (10230, 10229);")
	e.shot("p7-safe-send")
	e.sh.confirm.open = false
	e.tt.Frame()

	e.sh.pwp = pwPrompt{open: true, title: "输入数据库密码", msg: "该连接为「安全」模式，每次写操作都需要验证密码。", err: "密码不正确"}
	e.tt.Frame()
	e.has("密码不正确")
	e.shot("p7-safe-password")
}
