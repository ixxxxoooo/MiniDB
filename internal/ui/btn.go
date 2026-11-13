package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 全应用统一的按钮外观。只有三个层级，高度、字号、图标大小、内边距各自固定，
// 不要再在调用处单独设 Height / FontSize：
//
//	actBtn / actPrimary      24px  面板、列表、工具栏里的操作按钮：图标 + 文字
//	fieldBtn / fieldPrimary  28px  与 28px 输入框同排的按钮（选择文件、测试等）：图标 + 文字
//	footBtn / footPrimary    28px  对话框底部的取消、确认：纯文字（macOS 的对话框惯例）
//
// 只有图标的按钮用 tipBtn / toggleBtn（layout.go），带悬停提示。

// 三个层级的尺寸。
const (
	btnActH, btnActFont, btnActIcon, btnActPad         = sizeBtnSm, font2XS, 13, 9
	btnFieldH, btnFieldFont, btnFieldIcon, btnFieldPad = sizeBtn, fontBase, 14, 12
	btnFootPad                                         = 14
)

// buildBtn 画一个按钮：primary 为主按钮；ic 为空则只有文字；h、font、iconSize、pad 是该层级的尺寸。
// 图标和文字放在同一个居中的行里，行高固定为 16，所以图标和文字的视觉中心对得齐。
func buildBtn(c *ui.Context, ic *ui.SVG, label string, primary bool, h, font, iconSize, pad float32) ui.Element {
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c, "")
	} else {
		b = ui.Button(c, "")
	}
	return b.Label(label).Height(h).Padding(0, pad).Gap(5).AlignItems(ui.Center).Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(iconSize)
		}
		ui.Text(c, label).FontSize(font).FixedLineHeight(16).SingleLine()
	})
}

// fieldMenuBtn 是带下拉菜单的 fieldBtn：图标（颜色 iconCol）+ 文字 + 小箭头；菜单用 .Menu(...) 挂上去。
// 工具栏里的"安全模式"这类选择按钮用它。
func fieldMenuBtn(c *ui.Context, ic *ui.SVG, iconCol ui.Color, label string) ui.Element {
	p := baseTokens(c)
	return ui.Button(c, "").Label(label).Height(btnFieldH).Padding(0, 8, 0, btnFieldPad-2).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(btnFieldIcon).TextColor(iconCol)
		ui.Text(c, label).FontSize(btnFieldFont).FixedLineHeight(16).SingleLine()
		ui.Icon(c, icChevronD).FontSize(11).TextColor(p.fgMuted)
	})
}

// actBtn 是面板、列表、工具栏里的操作按钮（24px，图标 + 文字）。
// 参数 c 为界面上下文（同一处有多个时传 c.Key 区分），ic 为图标，label 为文字；返回按钮元素，可继续 .Clicked()、.Disabled()。
func actBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return buildBtn(c, ic, label, false, btnActH, btnActFont, btnActIcon, btnActPad)
}

// actPrimary 是 actBtn 的主按钮形态。
func actPrimary(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return buildBtn(c, ic, label, true, btnActH, btnActFont, btnActIcon, btnActPad)
}

// fieldBtn 是与 28px 输入框同排的按钮（图标 + 文字），高度和输入框一致。
func fieldBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return buildBtn(c, ic, label, false, btnFieldH, btnFieldFont, btnFieldIcon, btnFieldPad)
}

// fieldPrimary 是 fieldBtn 的主按钮形态。
func fieldPrimary(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return buildBtn(c, ic, label, true, btnFieldH, btnFieldFont, btnFieldIcon, btnFieldPad)
}

// footBtn 是对话框底部的次要按钮（取消、关闭等），纯文字。
func footBtn(c *ui.Context, label string) ui.Element {
	return buildBtn(c, nil, label, false, btnFieldH, btnFieldFont, 0, btnFootPad)
}

// footPrimary 是对话框底部的主按钮（确认、开始等），纯文字。
func footPrimary(c *ui.Context, label string) ui.Element {
	return buildBtn(c, nil, label, true, btnFieldH, btnFieldFont, 0, btnFootPad)
}
