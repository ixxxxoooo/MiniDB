package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// modelPicker 是 AI 设置页里「获取模型」的状态：拉到的列表、筛选词、是否展开勾选面板。
// 勾选的结果直接写进 cfg.Models，所以这里只放界面上的临时状态。
type modelPicker struct {
	open     bool     // 勾选面板是否展开
	fetching bool     // 正在向服务商请求
	fetched  []string // 服务商返回的模型
	filter   string
	err      string
}

// modelRows 返回勾选面板要显示的模型：已保存的排在前面，其余是服务商返回的，按 filter 筛选。
//
// @author ixxxxoooo
// 参数：saved 为已勾选保存的模型，fetched 为服务商返回的模型，filter 为筛选词（忽略大小写）。
// 返回：去重后的显示列表。
func modelRows(saved, fetched []string, filter string) []string {
	q := strings.ToLower(strings.TrimSpace(filter))
	var rows []string
	for _, m := range append(slices.Clone(saved), fetched...) {
		if (q == "" || strings.Contains(strings.ToLower(m), q)) && !slices.Contains(rows, m) {
			rows = append(rows, m)
		}
	}
	return rows
}

// toggleModel 把模型加入或移出已保存的列表。
//
// @author ixxxxoooo
// 参数：cfg 为正在编辑的配置，id 为模型名，on 为是否勾选。
func toggleModel(cfg *app.AIConfig, id string, on bool) {
	i := slices.Index(cfg.Models, id)
	switch {
	case on && i < 0:
		cfg.Models = append(cfg.Models, id)
	case !on && i >= 0:
		cfg.Models = slices.Delete(cfg.Models, i, i+1)
	}
}

// fetchModels 用当前正在编辑的配置（不必先保存）向服务商拉取可用模型，成功后展开勾选面板。
func (s *Shell) fetchModels() {
	cfg, ok := s.currentAIConfig()
	if !ok {
		return
	}
	m := &s.set.aiForm.models
	m.fetching, m.err = true, ""
	var list []string
	var err error
	s.up.bg(func() { list, err = s.app.Settings.ListAIModels(cfg) }, func() {
		m.fetching = false
		switch {
		case err != nil:
			m.err = errMessage(err)
		case len(list) == 0:
			m.err = locales.SettingsAIModelsEmpty()
		default:
			m.fetched, m.open = list, true
		}
	})
}

// modelField 是「模型」一行：可下拉选择也可手写的输入框，加一个「获取模型」按钮；
// 拉到列表后在下面展开勾选面板。
func (s *Shell) modelField(c *ui.Context, p tokens) {
	m := &s.set.aiForm.models
	ui.Row(c).Gap(6).FillWidth().AlignItems(ui.Center).Children(func() {
		s.modelCombo(c, p)
		if fieldBtn(c.Key("ai-fetch"), icDownload, locales.SettingsAIModelsFetch()).Disabled(m.fetching).Clicked() {
			s.fetchModels()
		}
		if m.fetching {
			ui.Spinner(c)
		}
	})
	if m.err != "" {
		ui.Text(c, m.err).FontSize(font2XS).TextColor(p.danger).Selectable()
	} else {
		ui.Text(c, locales.SettingsAIModelsHint()).FontSize(font2XS).TextColor(p.fgMuted)
	}
	if m.open {
		s.modelPanel(c, p)
	}
}

// modelCombo 是模型输入框：文字直接绑定 cfg.Model（手写什么就是什么），
// 点击或按 ↓ 弹出已保存的模型，输入时按文字筛选，选中后填入。
func (s *Shell) modelCombo(c *ui.Context, p tokens) {
	d := &s.set
	var cb ui.ComboboxParts
	box := ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn).Radius(radiusBtn).
		Background(p.surface).Border(1, p.border).Grow(1).MinWidth(0)
	box.Children(func() {
		cb = ui.ComboboxBase(c.Key("ai-model"), &d.cfg.Model)
		cb.Input.Grow(1).Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false)
		ui.Icon(c, icChevronD).FontSize(13).TextColor(p.fgMuted)
	})
	opts := d.cfg.Models
	if cb.Filtering() {
		opts = modelRows(d.cfg.Models, nil, d.cfg.Model)
	}
	cb.Popup(func(panel ui.Element) {
		panel.Background(p.surfaceElev).Border(1, p.border).Radius(8).Padding(4).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		scrollbars(c, ui.Scroll(c.Key("ai-model-opts")).FillWidth().MaxHeight(240)).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for _, id := range opts {
					item := cb.Item(id).Padding(5, 8).Radius(radiusBtn).FillWidth()
					if item.Highlighted() {
						item.Background(p.accent).TextColor(ui.Hex("#ffffff"))
					}
					item.Children(func() { ui.Text(c, id).FontSize(font2XS + 1).SingleLine() })
				}
			})
		})
	})
	if id, ok := cb.Chosen(); ok {
		d.cfg.Model = id
	}
	if box.Clicked() {
		cb.Input.Focus()
	}
}

// modelPanel 是勾选面板：筛选框加一列带复选框的模型，勾上就加入下拉列表（保存后生效）。
func (s *Shell) modelPanel(c *ui.Context, p tokens) {
	d := &s.set
	m := &d.aiForm.models
	ui.Column(c).FillWidth().Gap(6).Padding(8).Radius(radiusBtn).Background(p.surface).Border(1, p.borderSubtle).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.TextInput(c.Key("ai-mfilter"), &m.filter).Placeholder(locales.SettingsAIModelsFilter()).Grow(1).Height(sizeBtn)
			if footBtn(c.Key("ai-mclose"), locales.SettingsAIModelsClose()).Clicked() {
				m.open = false
			}
		})
		rows := modelRows(d.cfg.Models, m.fetched, m.filter)
		scrollbars(c, ui.Scroll(c.Key("ai-mlist")).FillWidth().MaxHeight(220)).Children(func() {
			ui.Column(c).FillWidth().Gap(1).Children(func() {
				for _, id := range rows {
					on := slices.Contains(d.cfg.Models, id)
					if ui.Checkbox(c.Key("ai-mc-"+id), &on, id).Changed() {
						toggleModel(&d.cfg, id, on)
					}
				}
			})
		})
		total := len(modelRows(d.cfg.Models, m.fetched, ""))
		ui.Text(c, locales.SettingsAIModelsFound(total, len(d.cfg.Models))).FontSize(font2XS).TextColor(p.fgMuted)
	})
}
