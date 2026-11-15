package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 设置窗口的导航与页头（设计稿：960×640，左边 220px 分组导航带搜索，右边每页是页头 + 卡片分区）：
// 导航分五组（常规 / 工作区 / 安全与数据 / 智能 / 其他），每项一个小图标加文字，选中项整行铺强调淡底；
// 页头是图标块 + 页名 + 一句话说明。

// settingsNavItem 是导航里的一项。
type settingsNavItem struct {
	id    int
	icon  *ui.SVG
	label string
}

// settingsNavGroup 是导航里的一组。
type settingsNavGroup struct {
	label string
	items []settingsNavItem
}

// settingsNavGroups 返回导航的全部分组，顺序就是画出来的顺序。
func settingsNavGroups() []settingsNavGroup {
	return []settingsNavGroup{
		{locales.SettingsGroupGeneral(), []settingsNavItem{{setGeneral, icSettings, locales.SettingsGeneral()}, {setAppearance, icSun, locales.SettingsAppearance()}}},
		{locales.SettingsGroupWork(), []settingsNavItem{{setData, icTable, locales.SettingsData()}, {setEditor, icTerminal, locales.SettingsEditor()}, {setCSV, icColumns, locales.SettingsCSV()}, {setConn, icPlug, locales.SettingsConn()}}},
		{locales.SettingsGroupSafe(), []settingsNavItem{{setSecurity, icLock, locales.SettingsSecurity()}, {setBackup, icBackup, locales.SettingsBackup()}}},
		{locales.SettingsGroupSmart(), []settingsNavItem{{setAI, icBot, locales.SettingsAI()}}},
		{locales.SettingsGroupOther(), []settingsNavItem{{setShortcuts, icKeyboard, locales.SettingsShortcuts()}, {setAbout, icInfo, locales.SettingsAbout()}}},
	}
}

// settingsDescOf 是一页的一句话说明（页头里页名下面那行）。
func settingsDescOf(id int) string {
	switch id {
	case setGeneral:
		return locales.SettingsGeneralDesc()
	case setData:
		return locales.SettingsDataDesc()
	case setEditor:
		return locales.SettingsEditorDesc()
	case setCSV:
		return locales.SettingsCSVDesc()
	case setConn:
		return locales.SettingsConnDesc()
	case setAppearance:
		return locales.SettingsAppearanceDesc()
	case setSecurity:
		return locales.SettingsSecurityDesc()
	case setBackup:
		return locales.SettingsBackupDesc()
	case setAI:
		return locales.SettingsAIDesc()
	case setShortcuts:
		return locales.SettingsShortcutsDesc()
	}
	return locales.SettingsAboutDesc()
}

// settingsNav 画导航列：顶上搜索框（按名称筛选），下面是分组标题和各项；点一项就把 d.tab 切过去。
func (s *Shell) settingsNav(c *ui.Context, p tokens, d *settingsDialog, groups []settingsNavGroup) {
	ui.Column(c).Width(sizeSetNav).FillHeight().Background(p.surfaceSecondary).Padding(spaceLG, spaceMD).Gap(1).Shrink(0).Children(func() {
		ui.SearchField(c.Key("set-q"), &d.navQ).Placeholder(locales.SettingsSearch()).Height(sizeBtn).FontSize(fontCtl).FillWidth().Margin(0, 0, spaceSM, 0)
		q := strings.ToLower(strings.TrimSpace(d.navQ))
		scrollbars(c, ui.Scroll(c.Key("set-nav")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).FillWidth().Gap(1).Children(func() {
				for _, g := range groups {
					var shown []settingsNavItem
					for _, it := range g.items {
						if q == "" || strings.Contains(strings.ToLower(it.label), q) {
							shown = append(shown, it)
						}
					}
					if len(shown) == 0 {
						continue
					}
					ui.Text(c, g.label).FontSize(fontCaption).FontWeight(600).LetterSpacing(0.5).TextColor(p.fgFaint).Padding(spaceMD, spaceSM, spaceXS, spaceSM)
					for _, it := range shown {
						s.settingsNavItem(c, p, d, it)
					}
				}
			})
		})
	})
}

// settingsNavItem 画导航里的一项。
func (s *Shell) settingsNavItem(c *ui.Context, p tokens, d *settingsDialog, it settingsNavItem) {
	on := d.tab == it.id
	row := ui.Row(c.Key(it.label)).Transition(rowHover).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceSM).Height(sizeBtn + spaceXS).Radius(radiusMenu - 1).FillWidth()
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

// settingsPageHead 画右边的页头：图标块 + 页名 + 一句话说明，右端关闭。参数：it 是当前页，close 在点关闭时调用。
func settingsPageHead(c *ui.Context, p tokens, it settingsNavItem, close func()) {
	ui.Row(c.Key("set-head")).Gap(spaceMD).AlignItems(ui.Center).Padding(0, spaceSM, 0, spaceXL-2).Height(sizeSetHead).FillWidth().Shrink(0).Children(func() {
		ui.Row(c).Size(sizeBtnLg+2, sizeBtnLg+2).Shrink(0).Center().Radius(radiusPanel).Background(p.accentSoft).Children(func() {
			ui.Icon(c, it.icon).FontSize(fontTitle + 2).TextColor(p.accent)
		})
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, it.label).FontWeight(650).FontSize(fontTitle).SingleLine()
			if d := settingsDescOf(it.id); d != "" {
				ui.Text(c, d).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
			}
		})
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			close()
		}
	})
}
