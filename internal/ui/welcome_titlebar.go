package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 欢迎页的标题栏：设计稿里没有标签、搜索和通知，只有居中的品牌（应用图标 + MiniDB 字标），
// 右侧是主题切换和设置。整条都是拖动窗口的手柄；两个按钮自己接点击。

// welcomeTitleBar 画欢迎页（没有打开任何连接时）的标题栏。
// 参数：c 绘制上下文；p 当前配色令牌。
func (s *Shell) welcomeTitleBar(c *ui.Context, p tokens) {
	left := max(c.TitleBar().Left, spaceMD)
	ui.Box(c.Key("titlebar")).Height(sizeToolbar).FillWidth().Shrink(0).Background(p.surfaceTertiary).DragWindow().Children(func() {
		ui.Box(c.Key("tb-line")).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
		ui.Toolbar(c, func() {
			ui.Row(c).FillWidth().Height(sizeToolbar).AlignItems(ui.Center).Padding(0, spaceSM+2, 0, left).DragWindow().Children(func() {
				// 左右各一块等宽的留白，让品牌真正居中在窗口里（右边的按钮盖在右侧留白上）
				ui.Box(c.Key("wtb-l")).Grow(1).Height(sizeToolbar).DragWindow()
				s.brandLockup(c, p)
				ui.Row(c.Key("wtb-r")).Grow(1).Height(sizeToolbar).Justify(ui.End).AlignItems(ui.Center).Gap(spaceSM - 2).DragWindow().Children(func() {
					s.updateChip(c, p)    // 欢迎页没有状态栏，更新提示只能放这里
					if len(s.notes) > 0 { // 设计稿里没有通知铃；有通知时才出现，免得错过失败信息
						s.bellButton(c, p)
					}
					themeTip, themeIcon := locales.ToolbarToDark(), icMoon
					if c.Theme().Dark {
						themeTip, themeIcon = locales.ToolbarToLight(), icSun
					}
					if tipBtn(c, themeIcon, themeTip).Clicked() {
						s.toggleTheme(c.Theme().Dark)
					}
					if tipBtn(c, icSettings, locales.ToolbarSettings()+s.keyHint("settings")).Clicked() {
						s.openSettings(0)
					}
				})
			})
		}).Padding(0).DragWindow()
	})
}

// brandLockup 是应用图标加「MiniDB」字标（Mini 细而淡，DB 粗），对应设计稿的 LOGO.lockup。
func (s *Shell) brandLockup(c *ui.Context, p tokens) {
	ui.Row(c.Key("brand")).Gap(spaceXS + 1).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		logo(c, sizeBrandMark)
		ui.Row(c.Key("brand-wm")).AlignItems(ui.Center).Children(func() {
			ui.Text(c.Key("wm-mini"), "Mini").FontSize(fontBody).FontWeight(450).TextColor(p.fg.Alpha(0.72)).SingleLine()
			ui.Text(c.Key("wm-db"), "DB").FontSize(fontBody).FontWeight(600).TextColor(p.fg).SingleLine()
		})
	})
}
