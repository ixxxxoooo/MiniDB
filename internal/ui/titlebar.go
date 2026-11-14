package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 标题栏：40px，标签并入其中，右侧只留四个高频入口——⌘K 搜索、控制台日志、AI 助手、通知。
// 原来挤在标题栏里的东西各有去处：新建 / 断开连接、切库、安全模式在侧栏顶部的连接卡里，
// 事务、索引、后台任务、更新提示在底部状态栏（statusbar.go），主题和设置在活动栏（railbar.go）。
// 选中的标签压在标题栏底线上，和下面的内容连成一体，所以底线先画，标签后画。

// titleBar 画标题栏。没有打开的连接时左边只有新建连接按钮，标签区留空。
func (s *Shell) titleBar(c *ui.Context, p tokens) {
	bar := c.TitleBar()
	w := s.activeWorkspace()
	left := max(bar.Left, spaceMD) + spaceXS
	ui.Box(c.Key("titlebar")).Height(sizeToolbar).FillWidth().Shrink(0).Background(p.toolbar).DragWindow().Children(func() {
		ui.Box(c.Key("tb-line")).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
		ui.Toolbar(c, func() {
			ui.Row(c).Gap(spaceXS).AlignItems(ui.End).Padding(0, spaceMD, 0, left).FillWidth().Height(sizeToolbar).Children(func() {
				ui.Row(c.Key("tb-left")).Gap(spaceXS).AlignItems(ui.Center).Height(sizeToolbar).Shrink(0).Children(func() {
					if w == nil {
						if tipBtn(c, icPlug, locales.ToolbarNewConnection()+s.keyHint("new-conn")).Clicked() {
							s.newConnectionDialog()
						}
						return
					}
					if toggleBtn(c, icSidebar, locales.ToolbarSidebar()+s.keyHint("sidebar"), !w.sidebarHidden).Size(sizeBtn, sizeBtn).Clicked() {
						w.sidebarHidden = !w.sidebarHidden
					}
				})
				if w != nil {
					w.tabBar(c, p)
				} else {
					ui.Spacer(c)
				}
				ui.Row(c.Key("tb-right")).Gap(spaceXS).AlignItems(ui.Center).Height(sizeToolbar).Shrink(0).Children(func() {
					if w == nil {
						s.updateChip(c, p) // 有工作区时更新提示在状态栏里，没有连接时只能放这儿
					}
					s.searchButton(c, p)
					vline(c, p.border, 16)
					if toggleBtn(c, icConsole, locales.ToolbarConsole()+s.keyHint("console"), s.consoleOpen).Size(sizeBtn, sizeBtn).Clicked() {
						s.consoleOpen = !s.consoleOpen
					}
					if toggleBtn(c, icSparkle, locales.ToolbarAI()+s.keyHint("ai"), w != nil && w.panel == 2).Size(sizeBtn, sizeBtn).Disabled(w == nil).Clicked() {
						s.cmdToggleAI()
					}
					s.bellButton(c, p)
				})
			})
		}).Padding(0)
	})
}

// searchButton 是标题栏里的搜索框：长得像输入框，点一下打开命令面板。
// 无障碍标签与悬浮提示沿用「快速搜索 + 快捷键」。
func (s *Shell) searchButton(c *ui.Context, p tokens) {
	tip := locales.ToolbarSearch() + s.keyHint("palette")
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Width(sizeSearch).Height(sizeBtnSm).Padding(0, 7).Gap(5).
		Radius(radiusSm).Border(1, p.border).Background(p.surface).Justify(ui.Start).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, icSearch).FontSize(btnActIcon).TextColor(p.fgMuted)
		ui.Text(c, locales.TitlebarSearch()).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().Grow(1)
		if k := s.cmdKeys("palette"); k != "" { // 按实际绑定的键显示（默认 ⌘P；⌘K 是切换数据库）
			kbd(c, k)
		}
	})
	if b.Clicked() {
		s.cmdPalette()
	}
}
