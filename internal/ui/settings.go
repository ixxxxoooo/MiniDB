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
	setBackup
	setAI
	setShortcuts
	setAbout
)

func (s *Shell) settingsView(c *ui.Context) {
	d := &s.set
	if !d.open {
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
					ui.Scroll(c.Key("settings-body")).Grow(1).FillWidth().Children(func() {
						ui.Column(c).Gap(16).Padding(16, 20).FillWidth().Children(func() {
							switch d.tab {
							case setGeneral:
								s.generalSettings(c, p)
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
	d := &s.set
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
	settingsField(c, p, locales.SettingsTheme(), func() {
		type card struct {
			icon  *ui.SVG
			label string
		}
		cards := []card{{icSun, locales.SettingsThemeLight()}, {icMoon, locales.SettingsThemeDark()}, {icMonitor, locales.SettingsThemeSystem()}}
		// d.theme: 0 system, 1 light, 2 dark
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
	settingsField(c, p, locales.SettingsPageSize(), func() {
		cur := s.prefs.pageSize()
		if ui.Select(c, &cur, pageSizes).MinWidth(0).Width(110).Height(sizeBtn).Changed() {
			s.prefs.PageSize = cur
			s.savePrefs()
		}
		ui.Text(c, locales.SettingsPageSizeDesc()).FontSize(font2XS).TextColor(p.fgMuted)
	})
	show := !s.prefs.HideRowNumbers
	if settingsToggle(c, p, &show, locales.SettingsRowNumbers(), locales.SettingsRowNumbersDesc()) {
		s.prefs.HideRowNumbers = !show
		showRowNumbers = show
		s.savePrefs()
	}
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
					s.toast(err.Error())
					return
				}
				s.toast(locales.SettingsAISaved())
				for _, w := range s.spaces {
					if w.ai != nil {
						w.ai.checked = false
					}
				}
			})
		}
	})
}

// shortcutGroups is the list the Shortcuts page shows; it matches shortcuts.go.
func shortcutGroups() []struct {
	title string
	rows  [][2]string
} {
	return []struct {
		title string
		rows  [][2]string
	}{
		{locales.ScGlobal(), [][2]string{
			{"⌘P", locales.ScQuickSearch()}, {"⌘N", locales.ScNewConn()}, {"⌘O", locales.ScOpenConn()}, {"⌘T", locales.ScNewQuery()},
			{"⌘K", locales.ScSwitchDB()}, {"⌘,", locales.ScSettings()}, {"⌘W", locales.ScCloseTab()},
			{"⌘[ / ⌘]", locales.ScTabs()}, {"⌘⇧[ / ⌘⇧]", locales.ScWorkspaces()}, {"⌘I", locales.ScAI()},
			{"⌘1 – ⌘9", locales.ScTabNumber()}, {"⌘⇧\\", locales.WsAllTitle()}, {"⌘0", locales.ToolbarSidebar()},
			{"⌘E", locales.ScNewSQL()}, {"⌘.", locales.MenuProcesses()}, {"⌘⇧E", locales.MenuExport()},
			{"⌘⇧R", locales.ToolbarReconnect()}, {"⌘⌃← / ⌘⌃→", locales.ScBackForward()}, {"⌃⌘C", locales.ToolbarConsole()},
		}},
		{locales.ScTables(), [][2]string{
			{"⌘F", locales.ScFilter()}, {"⌘S", locales.ScCommit()}, {"⌘⇧P", locales.TablePreviewSQL()},
			{"⌘⇧⌫", locales.TableDiscard()}, {"⌘R", locales.ScRefresh()}, {"⌘⌥R", locales.MenuRefreshTab()},
			{"⌘⌥[ / ⌘⌥]", locales.ScSubViews()}, {"↑ / ↓", locales.ScMoveRow()}, {"Space", locales.ScPreview()},
			{"Insert", locales.ScInsert()}, {"⌫", locales.ScDelete()}, {"⌘C", locales.ScCopy()},
		}},
		{locales.ScEditor(), [][2]string{
			{"⌘↵", locales.ScRun()}, {"⌘⇧↵", locales.ScRunAll()}, {"⌘⇧F", locales.ScFormat()},
			{"⌘D", locales.ScFavorite()}, {"⌘Y", locales.ScHistory()}, {"⌘= / ⌘-", locales.ScFontSize()},
			{"⌘/", locales.ScComment()}, {"⌘F", locales.ScFind()},
		}},
		{locales.ScDialogs(), [][2]string{{"Esc", locales.ScEscape()}, {"↵", locales.ScEnter()}}},
	}
}

func (s *Shell) shortcutSettings(c *ui.Context, p tokens) {
	settingsHead(c, p, locales.SettingsShortcuts(), locales.SettingsShortcutsDesc())
	for _, g := range shortcutGroups() {
		ui.Column(c.Key(g.title)).Gap(0).FillWidth().Children(func() {
			ui.Text(c, g.title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary).Padding(0, 0, 4, 0)
			for i, r := range g.rows {
				ui.Row(c.Key(r[1])).Gap(10).AlignItems(ui.Center).Height(28).FillWidth().Children(func() {
					ui.Text(c, r[1]).Grow(1).SingleLine()
					for j, k := range strings.Split(r[0], " / ") {
						if j > 0 {
							ui.Text(c, "/").FontSize(font2XS).TextColor(p.fgMuted)
						}
						ui.Text(c.Key(k), k).Font(mono).FontSize(font2XS).TextColor(p.fgSecondary).Padding(1, 6).Radius(4).Background(p.surfaceSecondary).Border(1, p.borderSubtle)
					}
				})
				if i < len(g.rows)-1 {
					hairline(c, p.borderSubtle)
				}
			}
		})
	}
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
