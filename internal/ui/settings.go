package ui

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/appdata"
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
	CompSpace   bool              `json:"compSpace,omitempty"`   // 补全后补一个空格
	NoAutoClose bool              `json:"noAutoClose,omitempty"` // 不自动闭合括号和引号

	NoZebra      bool   `json:"noZebra,omitempty"`      // 数据表不画斑马纹
	Scrollbars   int    `json:"scrollbars,omitempty"`   // 滚动条显示方式：0 滚动时显示（默认），1 悬停时显示，2 始终显示
	QueryTimeout int    `json:"queryTimeout,omitempty"` // 编辑器查询的超时（秒），0 不限
	KeepaliveSec int    `json:"keepaliveSec,omitempty"` // 连接保活间隔（秒），0 为默认 30 秒，-1 关闭
	DefaultSafe  string `json:"defaultSafe,omitempty"`  // 新连接默认的安全模式，空为静默

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
	s.up.bg(func() { _ = s.app.Store.Put("settings", "ui_prefs", prefs) }, func() {})
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

	checking bool
	update   string
}

func (s *Shell) openSettings(tab int) {
	d := &s.set
	d.open, d.tab = true, tab
	var cfg *app.AIConfig
	var prefs uiPrefs
	s.up.bg(func() {
		cfg, _ = s.app.Settings.GetAIConfig()
		_ = s.app.Store.Get("settings", "ui_prefs", &prefs)
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
	_ = s.app.Store.Get("settings", "ui_prefs", &s.prefs)
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
	ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(740).Height(520).Children(func() {
			ui.Row(c).Fill().Children(func() {
				ui.Column(c).Width(150).FillHeight().Background(p.surfaceSecondary).Padding(12, 0).Gap(2).Shrink(0).Children(func() {
					ui.Text(c, locales.SettingsTitle()).FontWeight(600).Padding(0, 14, 8, 14)
					for _, it := range nav {
						it := it
						row := ui.Row(c.Key(it.label)).Gap(8).AlignItems(ui.Center).Padding(0, 14).Height(30).FillWidth()
						col := p.fgSecondary
						switch {
						case d.tab == it.id:
							row.Background(p.accent.Alpha(0.10))
							col = p.accent
						case row.Hovered():
							row.Background(p.hover)
						}
						row.Children(func() {
							ui.Icon(c, it.icon).FontSize(14).TextColor(col)
							t := ui.Text(c, it.label).TextColor(col).SingleLine()
							if d.tab == it.id {
								t.FontWeight(500)
							}
						})
						if row.Clicked() {
							d.tab = it.id
						}
					}
				})
				ui.Box(c).Width(1).FillHeight().Background(p.border)
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					ui.Row(c).Justify(ui.End).AlignItems(ui.Center).Padding(0, 6).Height(32).FillWidth().Shrink(0).Children(func() {
						if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
							d.open = false
						}
					})
					hairline(c, p.borderSubtle)
					scrollbars(c, ui.Scroll(c.Key("settings-body")).Grow(1).FillWidth()).Children(func() {
						ui.Column(c).Gap(16).Padding(16, 20).FillWidth().Children(func() {
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

// settingsHead is the title and description a section starts with.
func settingsHead(c *ui.Context, p tokens, title, desc string) {
	ui.Column(c).Gap(2).FillWidth().Children(func() {
		ui.Text(c, title).FontWeight(600)
		ui.Text(c, desc).FontSize(font2XS).TextColor(p.fgSecondary)
	})
}

// settingsField is a labelled control, the label above it.
func settingsField(c *ui.Context, p tokens, label string, fn func()) {
	ui.Column(c).Gap(4).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
		fn()
	})
}

// settingsToggle is a check box with its label and a line of help.
func settingsToggle(c *ui.Context, p tokens, on *bool, label, help string) bool {
	changed := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		changed = ui.Checkbox(c, on, "").Changed()
		ui.Text(c, label).FontWeight(500)
		ui.Text(c, help).FontSize(font2XS).TextColor(p.fgMuted)
	})
	return changed
}

func (s *Shell) generalSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsGeneral(), locales.SettingsGeneralDesc())
	if s.win != nil { // no running app loop in tests: the language calls would wait for it
		settingsField(c, p, locales.SettingsLanguage(), func() {
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
			if ui.Segmented(c, &cur, locales.SettingsThemeSystem(), locales.SettingsLanguageZh(), locales.SettingsLanguageEn()).Changed() {
				mygo.App.SetLocale([]string{"", "zh-Hans", "en"}[cur])
			}
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, locales.SettingsReset()).Height(sizeBtn).Clicked() {
			s.ask(locales.SettingsResetTitle(), locales.SettingsResetMsg(), locales.SettingsReset(), s.resetPrefs)
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
	settingsField(c, p, locales.SettingsPassphrase(), func() {
		ui.TextInput(c.Key("backup-pass"), &d.passphrase).Password().Placeholder(locales.SettingsPassphrasePH()).FillWidth().Height(sizeBtn)
	})
	ui.Row(c).Gap(8).Children(func() {
		if labeledButton(c, icUpload, locales.SettingsExport(), false).Height(sizeBtn).Clicked() {
			s.exportConnections(d.passphrase)
		}
		if labeledButton(c, icDownload, locales.SettingsImport(), false).Height(sizeBtn).Clicked() {
			s.importConnections(d.passphrase)
		}
	})
}

func (s *Shell) aiSettings(c *ui.Context, p tokens) {
	d := &s.set
	th := c.Theme()
	settingsHead(c, p, locales.SettingsAI(), locales.SettingsAIDesc())
	settingsField(c, p, locales.SettingsBaseURL(), func() { ui.TextInput(c.Key("ai-url"), &d.cfg.BaseURL).FillWidth().Height(sizeBtn) })
	settingsField(c, p, locales.SettingsAPIKey(), func() { ui.TextInput(c.Key("ai-key"), &d.cfg.APIKey).Password().FillWidth().Height(sizeBtn) })
	settingsField(c, p, locales.SettingsModel(), func() { ui.TextInput(c.Key("ai-model"), &d.cfg.Model).FillWidth().Height(sizeBtn) })
	settingsField(c, p, locales.SettingsSystemPrompt(), func() {
		ui.TextArea(c.Key("ai-prompt"), &d.cfg.SystemPrompt).Lines(3, 6).FillWidth()
		ui.Text(c, locales.SettingsAIPromptHint()).FontSize(font2XS).TextColor(p.fgMuted)
	})
	if d.testMsg != "" {
		col := th.Danger
		if d.testOK {
			col = th.Success
		}
		ui.Text(c, d.testMsg).TextColor(col).FontSize(font2XS).Selectable()
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, locales.SettingsTestAI()).Height(sizeBtn).Disabled(d.testing).Clicked() {
			d.testing, d.testMsg = true, ""
			cfg := d.cfg
			var msg string
			var err error
			s.up.bg(func() { msg, err = s.app.Settings.TestAI(cfg) }, func() {
				d.testing = false
				d.testOK = err == nil
				if err != nil {
					d.testMsg = err.Error()
				} else {
					d.testMsg = msg
				}
			})
		}
		if d.testing {
			ui.Spinner(c)
		}
		ui.Spacer(c)
		if ui.PrimaryButton(c, locales.CommonSave()).Height(sizeBtn).Clicked() {
			cfg := d.cfg
			var err error
			s.up.bg(func() {
				if err = s.app.Settings.SaveAIConfig(cfg); err == nil {
					s.app.AI.ReloadConfig()
				}
			}, func() {
				if err != nil {
					s.toastErr(err.Error())
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

func (s *Shell) aboutSettings(c *ui.Context, p tokens) {
	d := &s.set
	settingsHead(c, p, locales.SettingsAbout(), locales.SettingsAboutDesc())
	info := [][2]string{
		{locales.AppName(), appversion.Description},
		{locales.SettingsVersion(s.appVersion()), ""},
		{locales.SettingsRepository(), "github.com/ixxxxoooo/MiniDB"},
		{locales.SettingsDataDir(), appdata.RootDir()},
	}
	ui.Column(c).FillWidth().Radius(radiusBtn).Border(1, p.borderSubtle).Children(func() {
		for i, r := range info {
			ui.Row(c.Key(r[0])).Gap(12).Padding(8, 12).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, r[0]).FontWeight(500).Width(150).Shrink(0).SingleLine()
				ui.Text(c, r[1]).TextColor(p.fgSecondary).FontSize(font2XS).Grow(1).Selectable()
			})
			if i < len(info)-1 {
				hairline(c, p.borderSubtle)
			}
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, locales.SettingsDataDirOpen()).Height(sizeBtn).Clicked() {
			_ = s.host.OpenPath(appdata.RootDir())
		}
		if ui.Button(c, locales.SettingsLogs()).Height(sizeBtn).Clicked() {
			_ = s.host.OpenPath(filepath.Dir(appdata.LogFilePath()))
		}
		if ui.Button(c, locales.SettingsCheckUpdates()).Height(sizeBtn).Disabled(d.checking).Clicked() {
			s.checkUpdates()
		}
		if d.checking {
			ui.Spinner(c)
		}
	})
	if d.update != "" {
		ui.Text(c, d.update).FontSize(font2XS).Selectable()
	}
}

func (s *Shell) checkUpdates() {
	d := &s.set
	d.checking, d.update = true, ""
	var msg string
	s.up.bg(func() {
		if !mygo.Updater.Enabled() {
			msg = locales.SettingsUpToDate()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		up, err := mygo.Updater.Check(ctx)
		switch {
		case err != nil:
			msg = locales.SettingsUpdateFail(err.Error())
		case up == nil:
			msg = locales.SettingsUpToDate()
		default:
			msg = locales.SettingsUpdateAvailable(up.Version)
		}
	}, func() { d.checking, d.update = false, msg })
}

// appVersion is the version of mygo.json, which the build embeds.
func (s *Shell) appVersion() string {
	if s.win == nil { // no running app loop (tests)
		return appversion.CurrentVersion()
	}
	return mygo.App.Version()
}
