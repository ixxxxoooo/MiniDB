package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 标签的统一外观：主标签栏、侧栏顶部的「项目/查询/历史」、结果页的「结果 N」
// 都用同一套——选中的标签铺内容区底色、顶上一道 2px 的连接色线，其余的凹下去，
// 标签之间只隔一条 1px 的竖线，没有空隙。

// styleTab 给一个标签行盒套上选中 / 悬浮 / 普通三种外观。
// 顶部固定留 2px 边框（普通态透明），三种状态高度一致，切换时内容不会抖。
func styleTab(tab ui.Element, p tokens, active, hovered bool, accent ui.Color) ui.Element {
	switch {
	case active:
		return tab.Background(p.surface).BorderWidth(2, 0, 0, 0).BorderColor(accent)
	case hovered:
		return tab.Background(p.surface.Alpha(0.6)).BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
	default:
		return tab.BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
	}
}

// tabStrip 画一条不能关闭、标签之间没有空隙的标签栏，返回被点中的标签下标（没有则 -1）。
// grow 为真时各标签均分整行宽度（侧栏），否则按文字宽度排列（结果页）；
// trailing 非空时画在标签栏最右端（结构视图里的「添加」按钮），仅 grow 为假时有效。
// 底边的分隔线由各段自己带（见 tabBaseline），选中的标签那一段没有，和下面的内容连成一体。
func tabStrip(c *ui.Context, p tokens, key string, labels []string, cur int, accent ui.Color, grow bool, trailing func()) (clicked int) {
	clicked = -1
	ui.Row(c.Key(key)).FillWidth().Height(sizeTab).Shrink(0).Background(p.toolbar).Children(func() {
		for i, label := range labels {
			active := i == cur
			tab := ui.Row(c.Key(fmt.Sprint("t", i))).AlignItems(ui.Center).Padding(0, 14, 2, 14).Height(sizeTab).Shrink(0)
			if grow {
				tab.Grow(1).Shrink(1).MinWidth(0).Padding(0, 6, 2, 6)
			}
			hovered := tab.Hovered()
			styleTab(tab, p, active, hovered, accent).Children(func() {
				fg := p.fgSecondary
				if active {
					fg = p.fg
				} else {
					tabBaseline(c, p)
				}
				t := ui.Text(c, label).FontSize(font2XS).FixedLineHeight(16).SingleLine().TextColor(fg).TruncateMiddle()
				if active {
					t.FontWeight(500)
				}
				if grow {
					t.Grow(1).TextAlign(ui.Center)
				}
			})
			if tab.Clicked() {
				clicked = i
			}
			ui.Box(c.Key(fmt.Sprint("s", i))).Width(1).FillHeight().Background(p.border).Shrink(0)
		}
		if !grow { // 标签排不满时，右边留白，底线照样延伸；trailing 是靠右的操作按钮
			ui.Box(c.Key("rest")).Grow(1).FillHeight().Children(func() { tabBaseline(c, p) })
			if trailing != nil {
				ui.Row(c.Key("trailing")).AlignItems(ui.Center).Padding(0, 6, 1, 6).Gap(4).Shrink(0).Children(func() {
					tabBaseline(c, p)
					trailing()
				})
			}
		}
	})
	return clicked
}

// tabBaseline 在当前元素的最底下画 1px 的分隔线（绝对定位，不占布局）。
// 标签栏的底线就由这样一段一段拼成，选中的标签不画这一段，它下面就没有线。
func tabBaseline(c *ui.Context, p tokens) {
	ui.Box(c.Key("tab-line")).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
}
