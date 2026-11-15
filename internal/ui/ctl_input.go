package ui

import "github.com/egoist/mygo/ui"

// @author ixxxxoooo
//
// 单行输入框。库的 TextInput 竖向内边距是 Space(1.5)，固定了高度的框（22 / 26）里内容区会矮于一行字，
// 文字贴着内容区顶部往下溢出：26 高的框偏下约 1px，22 高的偏下约 3px。
// textField 把竖向内边距归零，文字就按行高在框里居中（input_center_test.go 量真实画面锁定）。
// 只用在写了固定高度的输入框上；没有固定高度的、嵌在自己容器里的输入框（侧栏过滤、搜索条）不走这里。

// textField 画一个单行输入框。参数：c 为界面上下文（同页有多个时传 c.Key），value 为绑定的字符串。
// 返回元素，继续链式 .Height(...)、.Placeholder(...) 等。
func textField(c *ui.Context, value *string) ui.Element {
	return ui.TextInput(c, value).Padding(0, c.Theme().Space(2.5))
}
