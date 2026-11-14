package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 外观偏好：配色、表格密度、行样式、数字对齐、NULL 样式。字段并进 uiPrefs（匿名嵌入，JSON 仍是平铺的），
// 生效靠 applyLookPrefs 写进几个全局绘制开关；设置页里的控件在 lookSettings。

// lookPrefs 是设置里「外观」页的新增偏好；零值就是设计稿的默认（石墨、紧凑、细行线、数字右对齐、NULL 灰斜体）。
type lookPrefs struct {
	Zebra     bool   `json:"zebra,omitempty"`     // 表格画斑马纹（默认是设计稿的细行线）
	Scheme    string `json:"scheme,omitempty"`    // 配色：graphite（默认）/ paper / aurora
	Comfy     bool   `json:"comfy,omitempty"`     // 表格行高用标准的 30（默认紧凑的 26）
	NumLeft   bool   `json:"numLeft,omitempty"`   // 数值列不靠右（默认靠右）
	PlainNull bool   `json:"plainNull,omitempty"` // NULL 不用灰色斜体（默认灰色斜体）
}

// nullStyled 是「NULL 显示为灰色斜体」的开关，由 applyLookPrefs 写入。
var nullStyled = true

// applyLookPrefs 把外观偏好写进全局绘制开关；改动偏好、启动、恢复默认之后都要调用。只在界面线程里调用。
func (s *Shell) applyLookPrefs() {
	showZebra = s.prefs.Zebra
	activeScheme = schemeByName(s.prefs.Scheme)
	gridRowH = sizeRow
	if s.prefs.Comfy {
		gridRowH = sizeRowComfy
	}
	numRightAlign = !s.prefs.NumLeft
	nullStyled = !s.prefs.PlainNull
}

// nullColor 是 NULL 的文字色：灰斜体开着用最淡的灰，关掉就和普通文字一样清楚。
func nullColor(p tokens) ui.Color {
	if nullStyled {
		return p.fgMuted
	}
	return p.fgSecondary
}

// setLook 改完偏好后立即生效并存盘。
func (s *Shell) setLook(change func(*lookPrefs)) {
	change(&s.prefs.lookPrefs)
	s.applyLookPrefs()
	s.savePrefs()
}

// lookSettings 画设置「外观」页里设计稿新增的几行：配色、界面密度、表格行样式、数字右对齐、NULL 样式。
func (s *Shell) lookSettings(c *ui.Context, p tokens, row settingsRowFn, wide func(string, func())) {
	wide(locales.SettingsScheme(), func() {
		names := []string{locales.SchemeGraphite(), locales.SchemePaper(), locales.SchemeAurora()}
		cur := int(schemeByName(s.prefs.Scheme))
		ui.Row(c).Gap(8).FillWidth().Children(func() {
			for i, name := range names {
				i, name := i, name
				sw := surfaceDots(tokensFor(scheme(i), c.Theme().Dark))
				box := ui.Row(c.Key("scheme-"+name)).Grow(1).Basis(0).Gap(8).Padding(8, 10).Radius(radiusBtn).AlignItems(ui.Center).Border(1, p.border)
				switch {
				case i == cur:
					box.Background(p.accentSoft).BorderColor(p.accent)
				case box.Hovered():
					box.Background(p.hover)
				}
				box.Children(func() {
					for _, col := range sw {
						ui.Box(c).Size(sizeDot*2, sizeDot*2).Radius(sizeDot).Background(col).Border(1, p.borderSubtle)
					}
					ui.Text(c, name).FontSize(fontCtl).TextColor(p.fg)
				})
				if box.Clicked() && i != cur {
					s.setLook(func(l *lookPrefs) { l.Scheme = schemeNames[i] })
				}
			}
		})
	})
	row(locales.SettingsDensity(), locales.SettingsDensityDesc(), func() {
		sel := 0
		if s.prefs.Comfy {
			sel = 1
		}
		if segmented(c, &sel, locales.DensityCompact(), locales.DensityStandard()) {
			s.setLook(func(l *lookPrefs) { l.Comfy = sel == 1 })
		}
	})
	row(locales.SettingsRowStyle(), locales.SettingsRowStyleDesc(), func() {
		sel := 0
		if s.prefs.Zebra {
			sel = 1
		}
		if segmented(c, &sel, locales.RowStyleLines(), locales.SettingsZebra()) {
			s.setLook(func(l *lookPrefs) { l.Zebra = sel == 1 })
		}
	})
	right := !s.prefs.NumLeft
	if settingsToggle(c, row, &right, locales.SettingsNumRight(), locales.SettingsNumRightDesc()) {
		s.setLook(func(l *lookPrefs) { l.NumLeft = !right })
	}
	styled := !s.prefs.PlainNull
	if settingsToggle(c, row, &styled, locales.SettingsNullStyle(), locales.SettingsNullStyleDesc()) {
		s.setLook(func(l *lookPrefs) { l.PlainNull = !styled })
	}
}

// surfaceDots 取一套配色的三个代表色（底色、强调色、文字色），画在配色卡片上。
func surfaceDots(t tokens) [3]ui.Color { return [3]ui.Color{t.surfaceSecondary, t.accent, t.fg} }
