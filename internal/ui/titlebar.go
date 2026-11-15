package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 标题栏：36px、底色 surface-3，标签并入其中；左边是连接胶囊（侧栏开关在活动栏底部），右侧只留四个高频入口——搜索（图标）、控制台日志、AI 助手、通知。
// 原来挤在标题栏里的东西各有去处：新建 / 断开连接、切库、安全模式在侧栏顶部的连接卡里，
// 事务、索引、后台任务、更新提示在底部状态栏（statusbar.go），主题和设置在活动栏（railbar.go）。

// trafficEnd 是红绿灯的右缘：窗口里它从 X=14 开始（shell.go 的 TrafficLightPosition），三个直径 12、间距 8。
const trafficEnd = 14 + 3*12 + 2*8

// titleLeftInset 是标题栏内容的左边距：有红绿灯的平台固定在「红绿灯右缘 + 一个正常的间距」（设计稿 12px，即 78）。
// 不取系统给的内缩（barLeft）：它可能只有红绿灯右缘（66），胶囊会贴着红绿灯；也可能带着一大截额外留白，离得太远。
// 没有红绿灯的平台（barLeft 为 0）只留一个间距。参数：barLeft 是 c.TitleBar().Left。返回值：左边距。
func titleLeftInset(barLeft float32) float32 {
	if barLeft <= 0 {
		return spaceMD
	}
	return trafficEnd + spaceMD
}

// titleBar 画标题栏。没有打开的连接时左边只有新建连接按钮，标签区留空。
func (s *Shell) titleBar(c *ui.Context, p tokens) {
	bar := c.TitleBar()
	w := s.activeWorkspace()
	if w == nil { // 设计稿：欢迎页是自绘的简化标题栏，居中品牌 + 右侧主题与设置
		s.welcomeTitleBar(c, p)
		return
	}
	left := titleLeftInset(bar.Left)
	ui.Box(c.Key("titlebar")).Height(sizeToolbar).FillWidth().Shrink(0).Background(p.surfaceTertiary).DragWindow().Children(func() {
		ui.Box(c.Key("tb-line")).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
		// 盖在标题栏上的每一层容器都要自己标成「拖动手柄」：按下落在最上层的元素上，外层的手柄接不到。
		// ui.Toolbar 会把放不下的子项折进「»」溢出菜单；标签条的固有宽度是所有标签之和，
		// 一旦放进 Toolbar，标签多了整行就被折叠、标题栏只剩一个「»」。所以 Toolbar 只包左右两小块固定宽度的按钮，标签条夹在中间。
		ui.Row(c).Gap(spaceSM-2).AlignItems(ui.Center).Padding(0, spaceSM+2, 0, left).FillWidth().Height(sizeToolbar).DragWindow().Children(func() {
			ui.Toolbar(c.Key("tb-left"), func() {
				s.connChip(c, p, w) // 设计稿：连接胶囊在侧栏开关右边，点开「已打开的连接」
			}).Padding(0).Gap(spaceSM - 2).Height(sizeToolbar).Shrink(0).DragWindow()
			w.tabBar(c, p)
			ui.Toolbar(c.Key("tb-right"), func() {
				s.searchIcon(c)
				if toggleBtn(c, icConsole, locales.ToolbarConsole()+s.keyHint("console"), s.consoleOpen).Size(sizeBtn, sizeBtn).Clicked() {
					s.consoleOpen = !s.consoleOpen
				}
				if toggleBtn(c, icSparkle, locales.ToolbarAI()+s.keyHint("ai"), w.panel == 2).Size(sizeBtn, sizeBtn).Clicked() {
					s.cmdToggleAI()
				}
				s.bellButton(c, p)
			}).Padding(0).Gap(spaceSM - 2).Height(sizeToolbar).Shrink(0).DragWindow()
		})
	})
}
