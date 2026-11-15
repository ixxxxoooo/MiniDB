package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 设置 → 外观里的强调色色板：六个 18px 圆点（靛蓝 / 蓝 / 青 / 玫红 / 琥珀 / 单色），选中的外面套一圈文字色描边。
// 选择即时生效并存盘（setLook），悬浮提示是颜色名。

// accentLabel 返回强调色的显示名；i 是 accentNames 里的下标。
func accentLabel(i int) string {
	return [...]string{locales.AccentIndigo(), locales.AccentBlue(), locales.AccentTeal(), locales.AccentRose(), locales.AccentAmber(), locales.AccentMono()}[i]
}

// accentRow 画「强调色」一行。row 是设置页的窄控件行。
func (s *Shell) accentRow(c *ui.Context, p tokens, row settingsRowFn) {
	row(locales.SettingsAccent(), locales.SettingsAccentDesc(), func() {
		dark := c.Theme().Dark
		pal := schemeByName(s.prefs.Scheme)
		ui.Row(c).Gap(spaceSM + 2).AlignItems(ui.Center).Children(func() {
			for i, name := range accentNames {
				i, name := i, name
				sw := tipOn(c, ui.Button(c.Key("accent-"+accentLabel(i)), "").Label(accentLabel(i)), accentLabel(i)).Size(sizeDot*3, sizeDot*3).Padding(0).
					Radius(sizeDot * 3 / 2).Background(accentColor(name, pal, dark)).BorderWidth(0)
				if s.prefs.Accent == name {
					sw.Border(2, p.fg)
				}
				if sw.Clicked() && s.prefs.Accent != name {
					s.setLook(func(l *lookPrefs) { l.Accent = name })
				}
			}
		})
	})
}
