package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 设置窗口左边的导航（设计稿 A）：128px 宽、浅灰底，顶上是「设置」标题，每项一个小图标加文字，
// 选中项整行铺强调淡底、文字用强调色；没有图标瓦片。

// settingsNavItem 是导航里的一项。
type settingsNavItem struct {
	id    int
	icon  *ui.SVG
	label string
}

// settingsNav 画导航列；点一项就把 d.tab 切过去。
func (s *Shell) settingsNav(c *ui.Context, p tokens, d *settingsDialog, nav []settingsNavItem) {
	ui.Column(c).Width(sizeSetNav).FillHeight().Background(p.surfaceSecondary).Padding(spaceMD, spaceSM).Gap(1).Shrink(0).Children(func() {
		ui.Text(c, locales.SettingsTitle()).FontWeight(600).FontSize(fontCtl).Padding(spaceXS, spaceSM, spaceSM, spaceSM)
		for _, it := range nav {
			on := d.tab == it.id
			row := ui.Row(c.Key(it.label)).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceSM).Height(sizeBtnSm + 2).Radius(radiusBtn).FillWidth()
			fg, ic := p.fgSecondary, p.fgMuted
			switch {
			case on:
				row.Background(p.accentSoft)
				fg, ic = p.accentText, p.accentText
			case row.Hovered():
				row.Background(p.hover)
			}
			row.Children(func() {
				ui.Icon(c, it.icon).FontSize(btnActIcon).TextColor(ic).Shrink(0)
				t := ui.Text(c, it.label).FontSize(fontCtl).TextColor(fg).SingleLine()
				if on {
					t.FontWeight(600)
				}
			})
			if row.Clicked() {
				d.tab = it.id
			}
		}
	})
}
