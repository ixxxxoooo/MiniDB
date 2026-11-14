package ui

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// ---- 通用 ----

// resetPrefs 把所有偏好恢复成默认值（连接、历史、收藏等数据不受影响）。
func (s *Shell) resetPrefs() {
	s.prefs = uiPrefs{}
	s.set.theme = 0
	if s.win != nil { // 测试里没有运行中的应用循环，切主题会一直等
		applyTheme("")
	}
	showRowNumbers = true
	s.applyRuntimePrefs()
	s.keymapChanged() // 键位也回到默认，会一并存盘
	s.toastOK(locales.SettingsResetDone())
	s.redraw()
}

// ---- 数据表 ----

func (s *Shell) dataSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsData(), locales.SettingsDataDesc())
	settingsRow(c, p, locales.SettingsPageSize(), locales.SettingsPageSizeDesc(), func() {
		cur := s.prefs.pageSize()
		if ui.Select(c, &cur, pageSizes).MinWidth(0).Width(110).Height(sizeBtn).Changed() {
			s.prefs.PageSize = cur
			s.savePrefs()
		}
	})
	show := !s.prefs.HideRowNumbers
	if settingsToggle(c, p, &show, locales.SettingsRowNumbers(), locales.SettingsRowNumbersDesc()) {
		s.prefs.HideRowNumbers = !show
		showRowNumbers = show
		s.savePrefs()
	}
	zebra := !s.prefs.NoZebra
	if settingsToggle(c, p, &zebra, locales.SettingsZebra(), locales.SettingsZebraDesc()) {
		s.prefs.NoZebra = !zebra
		showZebra = zebra
		s.savePrefs()
	}
	raw := !s.prefs.RawJSON
	if settingsToggle(c, p, &raw, locales.SettingsPrettyJSON(), locales.SettingsPrettyJSONDesc()) {
		s.prefs.RawJSON = !raw
		s.savePrefs()
	}
}

// ---- CSV ----

func (s *Shell) csvSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsCSV(), locales.SettingsCSVDesc())
	before := s.prefs.Export
	csvOptions(c, p, &s.prefs.Export)
	if s.prefs.Export != before {
		s.savePrefs()
	}
}

// ---- 连接 ----

// durationLabel 是"30 秒""5 分钟"这样的显示；sec<0 为关闭，0 为不限（由 zero 指定文案）。
func durationLabel(sec int, zero string) string {
	switch {
	case sec < 0:
		return locales.SettingsOff()
	case sec == 0:
		return zero
	case sec%60 == 0:
		return locales.SettingsMinutes(sec / 60)
	}
	return locales.SettingsSeconds(sec)
}

// choice 是一组"数值 ↔ 文案"的下拉选项。
func choice(c *ui.Context, key string, vals []int, cur int, zero string) (int, bool) {
	labels := make([]string, len(vals))
	label := ""
	for i, v := range vals {
		labels[i] = durationLabel(v, zero)
		if v == cur {
			label = labels[i]
		}
	}
	if label == "" { // 配置文件里手写了不在列表中的值
		label = durationLabel(cur, zero)
		labels = append(labels, label)
		vals = append(vals, cur)
	}
	changed := ui.Select(c.Key(key), &label, labels).MinWidth(0).Width(140).Height(sizeBtn).Changed()
	if changed {
		for i, l := range labels {
			if l == label {
				return vals[i], true
			}
		}
	}
	return cur, false
}

func (s *Shell) connSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsConn(), locales.SettingsConnDesc())
	restore := !s.prefs.NoRestore
	if settingsToggle(c, p, &restore, locales.SettingsRestoreConns(), locales.SettingsRestoreConnsDesc()) {
		s.prefs.NoRestore = !restore
		s.savePrefs()
	}
	settingsRow(c, p, locales.SettingsKeepalive(), locales.SettingsKeepaliveDesc(), func() {
		cur := s.prefs.KeepaliveSec
		if cur == 0 {
			cur = 30
		}
		if v, ok := choice(c, "set-keepalive", []int{15, 30, 60, 300, -1}, cur, ""); ok {
			s.prefs.KeepaliveSec = v
			s.applyRuntimePrefs()
			s.savePrefs()
		}
	})
	settingsRow(c, p, locales.SettingsQueryTimeout(), locales.SettingsQueryTimeoutDesc(), func() {
		if v, ok := choice(c, "set-timeout", []int{0, 30, 60, 300, 1800}, s.prefs.QueryTimeout, locales.SettingsUnlimited()); ok {
			s.prefs.QueryTimeout = v
			s.applyRuntimePrefs()
			s.savePrefs()
		}
	})
}

// ---- 字体与主题 ----

var editorFontSizes = []int{9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 24, 28, 32}

func (s *Shell) appearanceSettings(c *ui.Context, p tokens) {
	d := &s.set
	settingsHead(c, p, locales.SettingsAppearance(), locales.SettingsAppearanceDesc())
	settingsField(c, p, locales.SettingsTheme(), func() {
		type card struct {
			icon  *ui.SVG
			label string
		}
		cards := []card{{icSun, locales.SettingsThemeLight()}, {icMoon, locales.SettingsThemeDark()}, {icMonitor, locales.SettingsThemeSystem()}}
		// d.theme: 0 跟随系统, 1 浅色, 2 深色
		pos := []int{1, 2, 0}
		ui.Row(c).Gap(8).FillWidth().Children(func() {
			for i, cd := range cards {
				i, cd := i, cd
				on := d.theme == pos[i]
				box := ui.Column(c.Key(cd.label)).Grow(1).Gap(6).Padding(12, 8).Radius(radiusBtn).AlignItems(ui.Center).Border(1, p.border)
				switch {
				case on:
					box.Background(p.accent.Alpha(0.08)).BorderColor(p.accent)
				case box.Hovered():
					box.Background(p.hover)
				}
				box.Children(func() {
					col := p.fgSecondary
					if on {
						col = p.accent
					}
					ui.Icon(c, cd.icon).FontSize(18).TextColor(col)
					ui.Text(c, cd.label).FontSize(font2XS).TextColor(col)
				})
				if box.Clicked() && !on {
					d.theme = pos[i]
					name := []string{"", "light", "dark"}[d.theme]
					applyTheme(name)
					s.prefs.Theme = name
					s.savePrefs()
				}
			}
		})
	})
	settingsRow(c, p, locales.SettingsScrollbars(), locales.SettingsScrollbarsDesc(), func() {
		labels := []string{locales.SettingsScrollbarsScroll(), locales.SettingsScrollbarsHover(), locales.SettingsScrollbarsAlways()}
		mode := min(max(s.prefs.Scrollbars, 0), len(labels)-1)
		label := labels[mode]
		if ui.Select(c.Key("set-bars"), &label, labels).MinWidth(0).Width(200).Height(sizeBtn).Changed() {
			for i, l := range labels {
				if l == label {
					s.prefs.Scrollbars = i
					barsMode = i
				}
			}
			s.savePrefs()
		}
	})
	settingsRow(c, p, locales.SettingsEditorFont(), locales.SettingsEditorFontDesc(), func() {
		cur := int(s.prefs.EditorFont)
		if cur == 0 {
			cur = editorFontDefault
		}
		labels := make([]string, len(editorFontSizes))
		label := ""
		for i, v := range editorFontSizes {
			labels[i] = strconv.Itoa(v)
			if v == cur {
				label = labels[i]
			}
		}
		if label == "" {
			label = strconv.Itoa(cur)
			labels = append(labels, label)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Select(c.Key("set-editor-font"), &label, labels).MinWidth(0).Width(110).Height(sizeBtn).Changed() {
				n, _ := strconv.Atoi(label)
				s.prefs.EditorFont = float32(min(max(n, editorFontMin), editorFontMax))
				s.savePrefs()
			}
			if fieldBtn(c, icUndo, locales.SettingsDefault()).Disabled(s.prefs.EditorFont == 0).Clicked() {
				s.prefs.EditorFont = 0
				s.savePrefs()
			}
		})
	})
}

// ---- 安全 ----

func (s *Shell) securitySettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsSecurity(), locales.SettingsSecurityDesc())
	settingsField(c, p, locales.SettingsDefaultSafe(), func() {
		cur := s.prefs.defaultSafeMode()
		label := safeModeLabel(cur)
		if safeSelect(c.Key("set-safe"), p, &label, 160) {
			s.prefs.DefaultSafe = string(safeModeFromLabel(label))
			s.savePrefs()
		}
		ui.Text(c, safeModeTip(safeModeFromLabel(label))).FontSize(font2XS).TextColor(p.fgMuted)
		ui.Text(c, locales.SettingsDefaultSafeDesc()).FontSize(font2XS).TextColor(p.fgMuted)
	})
}

// defaultSafeMode 是新建连接时预选的安全模式，没设过就是静默。
func (u uiPrefs) defaultSafeMode() database.SafeMode {
	for _, m := range database.SafeModes {
		if string(m) == u.DefaultSafe {
			return m
		}
	}
	return database.SafeModeSilent
}
