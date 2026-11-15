package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 带快捷键提示的操作按钮（设计稿 A 的 SQL 工具栏：「执行 ⌘↵」「AI 编辑 ⌘K」）：22px 档，
// 图标 + 文字 + 右端一枚键帽（keycapOn）。没有快捷键时 hint 为空，等同普通按钮。

// hintBtn 画一个带键位提示的按钮。primary 为真是主按钮；hintCol 是键位文字的颜色
// （主按钮上用实心底上文字色的淡化，淡底按钮上用强调文字色的淡化）。
func hintBtn(c *ui.Context, ic *ui.SVG, label, hint string, primary bool, hintCol ui.Color) ui.Element {
	if hint == "" {
		if primary {
			return actPrimary(c, ic, label)
		}
		return actBtn(c, ic, label)
	}
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c, "")
	} else {
		b = ui.Button(c, "")
	}
	return b.Label(label).Height(btnActH).Padding(0, btnActPad).Gap(5).AlignItems(ui.Center).Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(btnActIcon)
		}
		ui.Text(c, label).FontSize(btnActFont).FixedLineHeight(16).SingleLine()
		keycapOn(c, hint, primary).Margin(0, 0, 0, 2)
	})
}
