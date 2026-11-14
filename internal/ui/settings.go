package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	appversion "minidb/internal/version"
	"minidb/locales"
)

// uiPrefs are the choices kept across launches.
type uiPrefs struct {
	Theme          string  `json:"theme"` // "", "light", "dark"
	PageSize       string  `json:"pageSize,omitempty"`
	HideRowNumbers bool    `json:"hideRowNumbers,omitempty"`
	EditorFont     float32 `json:"editorFont,omitempty"` // SQL 编辑器字号，0 为默认
	RawJSON        bool    `json:"rawJson,omitempty"`    // 右侧栏不自动美化 JSON（默认美化）

	// 查询编辑器
	RunAll      bool              `json:"runAll,omitempty"`      // ⌘↵ 运行全部语句（默认只运行光标所在的一条）
	RowLimit    int               `json:"rowLimit,omitempty"`    // 查询结果的行数上限，0 为默认
	Params      bool              `json:"params,omitempty"`      // 启用查询参数
	ParamFormat string            `json:"paramFormat,omitempty"` // 占位符写法，见 sqltext.ParamFormat
	ParamQuoted bool              `json:"paramQuoted,omitempty"` // 引号里的占位符也算
	ParamValues map[string]string `json:"paramValues,omitempty"` // 记住的参数值
	CompUpper   bool              `json:"compUpper,omitempty"`   // 补全的关键字用大写
	FormatLower bool              `json:"formatLower,omitempty"` // 格式化时关键字写成小写（默认大写）
	CompSpace   bool              `json:"compSpace,omitempty"`   // 补全后补一个空格
	NoAutoClose bool              `json:"noAutoClose,omitempty"` // 不自动闭合括号和引号

	// 编辑器外观与缩进：零值保持原有行为
	NoWrap         bool `json:"noWrap,omitempty"`         // 不自动换行（默认换行）
	NoLineNumbers  bool `json:"noLineNumbers,omitempty"`  // 不显示行号
	NoActiveLine   bool `json:"noActiveLine,omitempty"`   // 不高亮当前行
	NoFolding      bool `json:"noFolding,omitempty"`      // 不显示折叠标记
	NoIndentGuides bool `json:"noIndentGuides,omitempty"` // 不显示缩进参考线
	ShowWhitespace bool `json:"showWhitespace,omitempty"` // 始终显示空白字符（默认只在选区里显示）
	TabSize        int  `json:"tabSize,omitempty"`        // 缩进宽度，0 为默认的 2
	IndentTabs     bool `json:"indentTabs,omitempty"`     // 用制表符缩进（默认用空格）

	NoZebra      bool   `json:"noZebra,omitempty"`      // 数据表不画斑马纹
	Scrollbars   int    `json:"scrollbars,omitempty"`   // 滚动条显示方式：0 滚动时显示（默认），1 悬停时显示，2 始终显示
	QueryTimeout int    `json:"queryTimeout,omitempty"` // 编辑器查询的超时（秒），0 不限
	KeepaliveSec int    `json:"keepaliveSec,omitempty"` // 连接保活间隔（秒），0 为默认 30 秒，-1 关闭
	DefaultSafe  string `json:"defaultSafe,omitempty"`  // 新连接默认的安全模式，空为静默
	NoRestore    bool   `json:"noRestore,omitempty"`    // 启动时不恢复上次打开的连接（默认恢复）
	NoAutoUpdate bool   `json:"noAutoUpdate,omitempty"` // 发现新版本后不自动下载（默认自动下载）

	Keymap map[string]string `json:"keymap,omitempty"` // 改过的快捷键：命令 -> "cmd+shift+f"，"none" 为不绑定

	Export exportPrefs `json:"export,omitempty"` // 上次的导出选项
	Dump   dumpPrefs   `json:"dump,omitempty"`   // 上次的备份选项
}

// pageSize is the rows per page new tables start with.
func (u uiPrefs) pageSize() string {
	for _, v := range pageSizes {
		if v == u.PageSize {
			return v
		}
	}
	return "200"
}

func (s *Shell) savePrefs() {
	prefs := s.prefs
	s.up.bg(func() { _ = s.app.UIState.SavePrefs(prefs) }, func() {})
}

type settingsDialog struct {
	open bool
	tab  int

	cfg     app.AIConfig
	loaded  bool
	testing bool
	testMsg string
	testOK  bool
	theme   int // 0 system, 1 light, 2 dark

	passphrase string
}

func (s *Shell) openSettings(tab int) {
	d := &s.set
	d.open, d.tab = true, tab
	var cfg *app.AIConfig
	var prefs uiPrefs
	s.up.bg(func() {
		cfg, _ = s.app.Settings.GetAIConfig()
		_ = s.app.UIState.LoadPrefs(&prefs)
	}, func() {
		if cfg != nil {
			d.cfg = *cfg
		}
		d.loaded = true
		switch prefs.Theme {
		case "light":
			d.theme = 1
		case "dark":
			d.theme = 2
		default:
			d.theme = 0
		}
	})
}

// ApplyPrefs restores the saved appearance; call before the window opens.
func (s *Shell) ApplyPrefs() {
	_ = s.app.UIState.LoadPrefs(&s.prefs)
	s.loadLayout()
	applyTheme(s.prefs.Theme)
	showRowNumbers = !s.prefs.HideRowNumbers
	s.applyRuntimePrefs()
}

// applyRuntimePrefs 把需要后端配合的偏好（超时、保活）推给 app 层。
func (s *Shell) applyRuntimePrefs() {
	showZebra = !s.prefs.NoZebra
	barsMode = min(max(s.prefs.Scrollbars, barsOnScroll), barsAlways)
	s.app.Queries.SetQueryTimeout(time.Duration(s.prefs.QueryTimeout) * time.Second)
	s.app.Connections.SetKeepaliveInterval(s.prefs.keepalive())
}

// keepalive 是保活间隔：默认 30 秒，负数关闭。
func (u uiPrefs) keepalive() time.Duration {
	switch {
	case u.KeepaliveSec < 0:
		return 0
	case u.KeepaliveSec == 0:
		return 30 * time.Second
	}
	return time.Duration(u.KeepaliveSec) * time.Second
}

func applyTheme(name string) {
	switch name {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// settings sections, in the order of the left navigation.
const (
	setGeneral = iota
	setData
	setEditor
	setCSV
	setConn
	setAppearance
	setSecurity
	setBackup
	setAI
	setShortcuts
	setAbout
)

func (s *Shell) settingsView(c *ui.Context) {
	d := &s.set
	if !d.open {
		if s.rec.active() { // 设置关了就不再录制
			s.stopRecording()
		}
		return
	}
	p := s.tokens(c)
	// The original's settings use 12px text and 28px controls.
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, font2XS+1
	type navItem struct {
		id    int
		icon  *ui.SVG
		label string
	}
	nav := []navItem{
		{setGeneral, icSettings, locales.SettingsGeneral()},
		{setData, icTable, locales.SettingsData()},
		{setEditor, icTerminal, locales.SettingsEditor()},
		{setCSV, icColumns, locales.SettingsCSV()},
		{setConn, icPlug, locales.SettingsConn()},
		{setAppearance, icSun, locales.SettingsAppearance()},
		{setSecurity, icLock, locales.SettingsSecurity()},
		{setBackup, icBackup, locales.SettingsBackup()},
		{setAI, icBot, locales.SettingsAI()},
		{setShortcuts, icKeyboard, locales.SettingsShortcuts()},
		{setAbout, icInfo, locales.SettingsAbout()},
	}
	title := ""
	for _, it := range nav {
		if it.id == d.tab {
			title = it.label
		}
	}
	ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(820).Height(560).Children(func() {
			ui.Row(c).Fill().Children(func() {
				// 左侧导航：每项一枚小图标瓦片，选中项整行淡淡着色（仿 macOS 系统设置）
				ui.Column(c).Width(200).FillHeight().Background(p.surfaceSecondary).Padding(16, 10).Gap(2).Shrink(0).Children(func() {
					ui.Text(c, locales.SettingsTitle()).FontWeight(700).FontSize(15).Padding(0, 8, 12, 8)
					for _, it := range nav {
						it := it
						on := d.tab == it.id
						row := ui.Row(c.Key(it.label)).Gap(9).AlignItems(ui.Center).Padding(0, 8).Height(32).Radius(8).FillWidth()
						switch {
						case on:
							row.Background(p.accent.Alpha(0.12))
						case row.Hovered():
							row.Background(p.hover)
						}
						row.Children(func() {
							tile, ic := p.fgMuted.Alpha(0.16), p.fgSecondary
							if on {
								tile, ic = p.accent, ui.Hex("#ffffff")
							}
							ui.Row(c).Size(20, 20).Shrink(0).Center().Radius(6).Background(tile).Children(func() {
								ui.Icon(c, it.icon).FontSize(12).TextColor(ic)
							})
							t := ui.Text(c, it.label).TextColor(map[bool]ui.Color{true: p.fg, false: p.fgSecondary}[on]).SingleLine()
							if on {
								t.FontWeight(600)
							}
						})
						if row.Clicked() {
							d.tab = it.id
						}
					}
				})
				ui.Box(c).Width(1).FillHeight().Background(p.borderSubtle)
				ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Padding(0, 10, 0, 22).Height(46).FillWidth().Shrink(0).Children(func() {
						ui.Text(c, title).FontWeight(600).FontSize(16)
						ui.Spacer(c)
						if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
							d.open = false
						}
					})
					hairline(c, p.borderSubtle)
					scrollbars(c, ui.Scroll(c.Key("settings-body")).Grow(1).FillWidth()).Children(func() {
						ui.Column(c).Gap(10).Padding(16, 22, 22, 22).FillWidth().Children(func() {
							switch d.tab {
							case setGeneral:
								s.generalSettings(c, p)
							case setData:
								s.dataSettings(c, p)
							case setEditor:
								settingsHead(c, p, locales.SettingsEditor(), locales.SettingsEditorDesc())
								s.editorOptions(c, p)
							case setCSV:
								s.csvSettings(c, p)
							case setConn:
								s.connSettings(c, p)
							case setAppearance:
								s.appearanceSettings(c, p)
							case setSecurity:
								s.securitySettings(c, p)
							case setBackup:
								s.backupSettings(c, p)
							case setAI:
								s.aiSettings(c, p)
							case setShortcuts:
								s.shortcutSettings(c, p)
							default:
								s.aboutSettings(c, p)
							}
						})
					})
				})
			})
		})
	}).Padding(0).Clip()
}

// settingsHead 是一页开头的说明文字（页标题已在窗口顶栏）。
func settingsHead(c *ui.Context, p tokens, title, desc string) {
	ui.Text(c, desc).FontSize(font2XS+1).TextColor(p.fgSecondary).Padding(0, 0, 4, 0)
}

// 设置页的条目：一组设置放进同一张圆角卡片，每项一行——左边名称加一句说明，右边是控件，项与项之间一条细线
// （和「编辑器」页一样，见 optGroup）。输入框这类宽控件用 wide：名称在上、控件在下，同样在卡片里。

// settingsRowFn 往一组设置里加一行：左边 label 和说明 help，右边是 control 画的窄控件（下拉、开关、按钮）。
type settingsRowFn func(label, help string, control func())

// settingsGroup 画一组设置的卡片。body 拿到 row（一行一个窄控件）和 wide（名称在上的宽控件）。
func settingsGroup(c *ui.Context, p tokens, body func(row settingsRowFn, wide func(label string, fn func()))) {
	ui.Column(c).FillWidth().Radius(radiusPanel-2).Background(p.surfaceSecondary.Alpha(0.55)).Border(1, p.borderSubtle).Clip().Children(func() {
		n := 0
		gap := func() {
			if n > 0 {
				hairline(c, p.borderSubtle)
			}
			n++
		}
		row := func(label, help string, control func()) {
			gap()
			ui.Row(c.Key("sr:"+label)).Gap(14).AlignItems(ui.Center).FillWidth().Padding(9, 14).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, label).FontWeight(500).TextColor(p.fg)
					if help != "" {
						ui.Text(c, help).FontSize(font2XS).TextColor(p.fgSecondary)
					}
				})
				control()
			})
		}
		wide := func(label string, fn func()) {
			gap()
			ui.Column(c.Key("sw:"+label)).Gap(8).FillWidth().Padding(10, 14).Children(func() {
				ui.Text(c, label).FontWeight(500).TextColor(p.fg)
				fn()
			})
		}
		body(row, wide)
	})
}

// settingsToggle 是一行开关；返回开关这一帧有没有被拨动。
func settingsToggle(c *ui.Context, row settingsRowFn, on *bool, label, help string) bool {
	changed := false
	row(label, help, func() { changed = ui.Switch(c, on).Changed() })
	return changed
}

func (s *Shell) generalSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsGeneral(), locales.SettingsGeneralDesc())
	if s.win != nil { // no running app loop in tests: the language calls would wait for it
		settingsGroup(c, p, func(row settingsRowFn, _ func(string, func())) {
			row(locales.SettingsLanguage(), "", func() {
				cur := 0
				switch strings.ToLower(mygo.App.Locale()) {
				case "en", "en-us":
					cur = 2
				}
				if mygo.App.ChosenLocale() == "" {
					cur = 0
				} else if cur == 0 {
					cur = 1
				}
				labels := []string{locales.SettingsThemeSystem(), locales.SettingsLanguageZh(), locales.SettingsLanguageEn()}
				label := labels[cur]
				if ui.Select(c.Key("set-lang"), &label, labels).MinWidth(0).Width(200).Height(sizeBtn).Changed() {
					for i, l := range labels {
						if l == label {
							mygo.App.SetLocale([]string{"", "zh-Hans", "en"}[i])
						}
					}
				}
			})
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		if fieldBtn(c, icUndo, locales.SettingsReset()).Clicked() {
			s.askWarn(locales.SettingsResetTitle(), locales.SettingsResetMsg(), locales.SettingsReset(), s.resetPrefs)
		}
	})
}

func (s *Shell) backupSettings(c *ui.Context, p tokens) {
	d := &s.set
	settingsHead(c, p, locales.SettingsBackup(), locales.SettingsBackupDesc())
	hint := locales.SettingsPassphraseWarn()
	col := p.warning
	if d.passphrase != "" {
		hint, col = locales.SettingsPassphraseOn(), p.success
	}
	ui.Text(c, hint).FontSize(font2XS).TextColor(col).Padding(8, 10).FillWidth().Radius(radiusBtn).Background(col.Alpha(0.10))
	settingsGroup(c, p, func(row settingsRowFn, wide func(string, func())) {
		wide(locales.SettingsPassphrase(), func() {
			ui.TextInput(c.Key("backup-pass"), &d.passphrase).Password().Placeholder(locales.SettingsPassphrasePH()).FillWidth().Height(sizeBtn)
		})
	})
	ui.Row(c).Gap(8).Children(func() {
		if fieldBtn(c, icUpload, locales.SettingsExport()).Clicked() {
			s.exportConnections(d.passphrase)
		}
		if fieldBtn(c, icDownload, locales.SettingsImport()).Clicked() {
			s.importConnections(d.passphrase)
		}
	})
}

func (s *Shell) aiSettings(c *ui.Context, p tokens) {
	d := &s.set
	th := c.Theme()
	settingsHead(c, p, locales.SettingsAI(), locales.SettingsAIDesc())
	settingsGroup(c, p, func(row settingsRowFn, wide func(string, func())) {
		wide(locales.SettingsBaseURL(), func() { ui.TextInput(c.Key("ai-url"), &d.cfg.BaseURL).FillWidth().Height(sizeBtn) })
		wide(locales.SettingsAPIKey(), func() { ui.TextInput(c.Key("ai-key"), &d.cfg.APIKey).Password().FillWidth().Height(sizeBtn) })
		wide(locales.SettingsModel(), func() { ui.TextInput(c.Key("ai-model"), &d.cfg.Model).FillWidth().Height(sizeBtn) })
		wide(locales.SettingsSystemPrompt(), func() {
			ui.TextArea(c.Key("ai-prompt"), &d.cfg.SystemPrompt).Lines(3, 6).FillWidth()
			ui.Text(c, locales.SettingsAIPromptHint()).FontSize(font2XS).TextColor(p.fgMuted)
		})
	})
	if d.testMsg != "" {
		col := th.Danger
		if d.testOK {
			col = th.Success
		}
		ui.Text(c, d.testMsg).TextColor(col).FontSize(font2XS).Selectable()
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if fieldBtn(c, icActivity, locales.SettingsTestAI()).Disabled(d.testing).Clicked() {
			d.testing, d.testMsg = true, ""
			cfg := d.cfg
			var msg string
			var err error
			s.up.bg(func() { msg, err = s.app.Settings.TestAI(cfg) }, func() {
				d.testing = false
				d.testOK = err == nil
				if err != nil {
					d.testMsg = errMessage(err)
				} else {
					d.testMsg = msg
				}
			})
		}
		if d.testing {
			ui.Spinner(c)
		}
		ui.Spacer(c)
		if fieldPrimary(c, icSave, locales.CommonSave()).Clicked() {
			cfg := d.cfg
			var err error
			s.up.bg(func() {
				if err = s.app.Settings.SaveAIConfig(cfg); err == nil {
					s.app.AI.ReloadConfig()
				}
			}, func() {
				if err != nil {
					s.toastErr(errMessage(err))
					return
				}
				s.toastOK(locales.SettingsAISaved())
				for _, w := range s.spaces {
					if w.ai != nil {
						w.ai.checked = false
					}
				}
			})
		}
	})
}

// appVersion is the version of mygo.json, which the build embeds.
func (s *Shell) appVersion() string {
	if s.win == nil { // no running app loop (tests)
		return appversion.CurrentVersion()
	}
	return mygo.App.Version()
}
