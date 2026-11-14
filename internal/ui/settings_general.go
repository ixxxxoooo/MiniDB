package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)
import (
	"github.com/egoist/mygo"
	appversion "minidb/internal/version"
	"strings"
)

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
