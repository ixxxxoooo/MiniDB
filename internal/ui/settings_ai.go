package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// headerRow 是请求头编辑器里的一行。
type headerRow struct{ key, val string }

// aiForm 保存 AI 设置页中不能直接绑定到 app.AIConfig 的编辑状态：
// 请求头是 map，需要按行编辑；最大 Token 和温度要允许输入框为空。
type aiForm struct {
	headers []headerRow
	maxTok  string
	temp    string
}

// load 用保存的配置初始化编辑状态。
func (f *aiForm) load(cfg app.AIConfig) {
	f.headers = f.headers[:0]
	keys := make([]string, 0, len(cfg.Headers))
	for k := range cfg.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f.headers = append(f.headers, headerRow{k, cfg.Headers[k]})
	}
	f.maxTok, f.temp = "", ""
	if cfg.MaxTokens > 0 {
		f.maxTok = strconv.Itoa(cfg.MaxTokens)
	}
	if cfg.Temperature != 0 {
		f.temp = strconv.FormatFloat(cfg.Temperature, 'f', -1, 64)
	}
}

// apply 把编辑状态写回配置；数字不合法时返回 false。空的请求头名会被丢弃。
func (f *aiForm) apply(cfg *app.AIConfig) bool {
	cfg.Headers = nil
	for _, h := range f.headers {
		if k := strings.TrimSpace(h.key); k != "" {
			if cfg.Headers == nil {
				cfg.Headers = map[string]string{}
			}
			cfg.Headers[k] = h.val
		}
	}
	cfg.MaxTokens, cfg.Temperature = 0, 0
	if t := strings.TrimSpace(f.maxTok); t != "" {
		n, err := strconv.Atoi(t)
		if err != nil || n <= 0 {
			return false
		}
		cfg.MaxTokens = n
	}
	if t := strings.TrimSpace(f.temp); t != "" {
		v, err := strconv.ParseFloat(t, 64)
		if err != nil || v < 0 || v > 2 {
			return false
		}
		cfg.Temperature = v
	}
	return true
}

// presetLabel 是预设在下拉框里的名字。
func presetLabel(p app.AIPreset) string {
	if p.ID == app.AIPresetCustom {
		return locales.SettingsAIPresetCustom()
	}
	return p.Name
}

// currentCfg 取出带编辑状态的完整配置；数字不合法时提示并返回 false。
func (s *Shell) currentAIConfig() (app.AIConfig, bool) {
	cfg := s.set.cfg
	if !s.set.aiForm.apply(&cfg) {
		s.toastErr(locales.SettingsAINumInvalid())
		return cfg, false
	}
	return cfg, true
}

// choosePreset 切换预设：先把已编辑的内容收回配置，套用预设后再重新装载编辑状态。
func (s *Shell) choosePreset(id string) {
	d := &s.set
	cfg := d.cfg
	_ = d.aiForm.apply(&cfg) // 数字不合法时保留原值，不拦截切换
	app.ApplyAIPreset(&cfg, id)
	maxTok, temp := d.aiForm.maxTok, d.aiForm.temp
	d.cfg = cfg
	d.aiForm.load(cfg)
	d.aiForm.maxTok, d.aiForm.temp = maxTok, temp // 生成参数与服务商无关，保留输入
}

func (s *Shell) aiSettings(c *ui.Context, p tokens) {
	d := &s.set
	th := c.Theme()
	settingsHead(c, p, locales.SettingsAI(), locales.SettingsAIDesc())
	settingsGroup(c, p, func(row settingsRowFn, wide func(string, func())) {
		wide(locales.SettingsAIPreset(), func() { s.presetPicker(c) })
		if app.AIPresetOf(d.cfg).ID == app.AIPresetCustom { // 只有自定义服务才需要自己选协议
			wide(locales.SettingsAIProtocol(), func() { s.protocolPicker(c) })
		}
		wide(locales.SettingsBaseURL(), func() { ui.TextInput(c.Key("ai-url"), &d.cfg.BaseURL).FillWidth().Height(sizeBtn) })
		wide(locales.SettingsAPIKey(), func() {
			ui.TextInput(c.Key("ai-key"), &d.cfg.APIKey).Password().FillWidth().Height(sizeBtn)
			if app.AIKeyOptional(d.cfg) {
				ui.Text(c, locales.SettingsAIKeyOptional()).FontSize(font2XS).TextColor(p.fgMuted)
			}
		})
		wide(locales.SettingsModel(), func() { ui.TextInput(c.Key("ai-model"), &d.cfg.Model).FillWidth().Height(sizeBtn) })
		wide(locales.SettingsAIHeaders(), func() { s.headerEditor(c, p) })
		wide(locales.SettingsAIMaxTokens(), func() {
			ui.TextInput(c.Key("ai-maxtok"), &d.aiForm.maxTok).Width(140).Height(sizeBtn)
			ui.Text(c, locales.SettingsAIMaxTokensHint()).FontSize(font2XS).TextColor(p.fgMuted)
		})
		wide(locales.SettingsAITemperature(), func() {
			ui.TextInput(c.Key("ai-temp"), &d.aiForm.temp).Width(140).Height(sizeBtn)
			ui.Text(c, locales.SettingsAITemperatureHint()).FontSize(font2XS).TextColor(p.fgMuted)
		})
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
			if cfg, ok := s.currentAIConfig(); ok {
				d.testing, d.testMsg = true, ""
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
		}
		if d.testing {
			ui.Spinner(c)
		}
		ui.Spacer(c)
		if fieldPrimary(c, icSave, locales.CommonSave()).Clicked() {
			if cfg, ok := s.currentAIConfig(); ok {
				s.saveAIConfig(cfg)
			}
		}
	})
}

// saveAIConfig 在后台保存配置并让所有 AI 面板重新检查可用性。
func (s *Shell) saveAIConfig(cfg app.AIConfig) {
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

// presetPicker 是服务商下拉框；选项来自 app.AIPresets。
func (s *Shell) presetPicker(c *ui.Context) {
	d := &s.set
	labels := make([]string, len(app.AIPresets))
	cur := presetLabel(app.AIPresetOf(d.cfg))
	for i, pr := range app.AIPresets {
		labels[i] = presetLabel(pr)
	}
	if ui.Select(c.Key("ai-preset"), &cur, labels).FillWidth().Height(sizeBtn).Changed() {
		for i, l := range labels {
			if l == cur {
				s.choosePreset(app.AIPresets[i].ID)
				break
			}
		}
	}
}

// headerEditor 按行编辑请求头；值用密码框，因为常常是密钥。
func (s *Shell) headerEditor(c *ui.Context, p tokens) {
	f := &s.set.aiForm
	del := -1
	for i := range f.headers {
		h := &f.headers[i]
		id := strconv.Itoa(i)
		ui.Row(c.Key("ai-h" + id)).Gap(6).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("ai-hk"+id), &h.key).Placeholder(locales.SettingsAIHeaderName()).Width(180).Height(sizeBtn)
			ui.TextInput(c.Key("ai-hv"+id), &h.val).Password().Placeholder(locales.SettingsAIHeaderValue()).Grow(1).Height(sizeBtn)
			if tipBtn(c.Key("ai-hd"+id), icTrash, locales.CommonDelete()).Size(sizeBtn, sizeBtn).Clicked() {
				del = i
			}
		})
	}
	if del >= 0 {
		f.headers = append(f.headers[:del], f.headers[del+1:]...)
	}
	if fieldBtn(c, icPlus, locales.SettingsAIAddHeader()).Clicked() {
		f.headers = append(f.headers, headerRow{})
	}
	ui.Text(c, locales.SettingsAIHeadersHint()).FontSize(font2XS).TextColor(p.fgMuted)
}

// protocolPicker 自定义服务的接口协议：OpenAI 兼容或 Anthropic 原生。
func (s *Shell) protocolPicker(c *ui.Context) {
	d := &s.set
	labels := []string{locales.SettingsAIProtocolOpenAI(), "Anthropic"}
	cur := labels[0]
	if d.cfg.Provider == app.AIProviderAnthropic {
		cur = labels[1]
	}
	if ui.Select(c.Key("ai-protocol"), &cur, labels).FillWidth().Height(sizeBtn).Changed() {
		d.cfg.Provider = ""
		if cur == labels[1] {
			d.cfg.Provider = app.AIProviderAnthropic
		}
	}
}
