package ui

import "github.com/egoist/mygo/ui"

// @author ixxxxoooo
//
// 模态的统一外壳：代替 ui.Modal。区别有三点（设计稿 A「弹层背景」「模态」）：
//  1. 背景是「窗口内容模糊 + 遮罩色」，不是一块纯黑；
//  2. 遮罩色走令牌 scrim，浅色深色各有各的；
//  3. 面板出场带 pop-c 动画（缩放 .96→1、下移 8px、淡入，轻微回弹）。
//
// 点背板、按 Esc 关闭、焦点陷在里面这些行为与 ui.Modal 一样（都是 DialogBase）。

// modalPanel 在窗口中央显示一个模态面板，*open 为真时有效。
// 参数：p 当前令牌；open 开关；fn 构建面板内容。返回面板元素（关闭时是空元素），调用方可以接着链式设置内边距、圆角、阴影。
func modalPanel(c *ui.Context, p tokens, open *bool, fn func()) ui.Element {
	if *open {
		blurLayer(c, p, open)
	}
	return ui.DialogBase(c, open, func(back, panel ui.Element) {
		t := c.Theme()
		w, h := c.Size()
		panel.Padding(t.Space(5)).Gap(t.Space(3)).Radius(t.Space(2.5)).Background(t.Background).
			MaxWidth(w-t.Space(10)).MaxHeight(h-t.Space(10)).Shadow(0, shadowY, shadowBlur, 0, p.shadow)
		enterPose(c, panel, posePop, ui.Snappy)
		fn()
	})
}
