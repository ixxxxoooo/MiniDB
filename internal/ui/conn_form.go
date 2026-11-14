package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接管理器的配置页（设计稿 08）：头部（标题 + 说明 + 关闭）、数据库类型网格、
// 下划线标签（常规 / SSH 隧道 / SSL / 高级）、标签内容、底部的测试 / 取消 / 保存 / 保存并连接。
// 各标签的内容在 conn_form_tabs.go，服务器与 SSH 字段在 conn_server.go。

// connField is a row of the form: its label at the left, right-aligned.
func connField(c *ui.Context, p tokens, label string, fn func()) {
	ui.Row(c).Gap(spaceMD).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(fontDense).TextColor(p.fgSecondary).Width(88).Shrink(0).TextAlign(ui.End)
		fn()
	})
}

// selectTag sets the environment (tag); its colour is fixed by the tag (tagColors).
func (d *connDialog) selectTag(tag string) {
	d.cfg.Tag, d.tagSel = tag, tagLabel(tag)
	if !d.safeSet { // a production connection starts out guarded
		d.safeSel = safeModeLabel((model.ConnectionConfig{Tag: tag}).EffectiveSafeMode())
	}
}

// connInput 是表单里的文本框（对话框里的大号 32px）：回车等于"保存并连接"。
// 参数 key 区分各个输入框，v 是绑定的字符串，submit 在回车时置真。
func connInput(c *ui.Context, key string, v *string, submit *bool) ui.Element {
	in := ui.TextInput(c.Key(key), v).Grow(1).Height(sizeInputLg)
	if in.Submitted() {
		*submit = true
	}
	return in
}

// connForm 画配置页。参数 c 为界面上下文，p 为主题色板，submit 在回车或点"保存并连接"时被置为真，
// back 在点返回时被置为真（由调用方在这一帧画完后再离开配置页，因为离开会清掉表单）。
func (s *Shell) connForm(c *ui.Context, p tokens, submit, back *bool) {
	d := &s.dlg
	s.connFormHeader(c, p, back)
	if d.urlOpen {
		ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Padding(0, spaceLG, spaceSM, spaceLG).Shrink(0).Children(func() {
			in := ui.TextInput(c.Key("url"), &d.urlText).Placeholder(locales.ConnURLHint()).Font(mono).FontSize(fontDense).Grow(1).Height(sizeInputLg).AutoFocus()
			if in.Submitted() || fieldBtn(c, icCheck, locales.CommonApply()).Clicked() {
				if !d.applyURL(d.urlText) {
					d.msg, d.msgOK = locales.ConnURLBad(), false
				} else {
					d.urlOpen, d.urlText, d.msg = false, "", ""
				}
			}
		})
	}
	s.typeGrid(c, p)
	tabs := d.tabs()
	d.tab = min(d.tab, len(tabs)-1)
	ui.Column(c.Key("conn-tabs")).FillWidth().Padding(0, spaceLG).Shrink(0).Children(func() {
		labels := make([]string, len(tabs))
		for i, t := range tabs {
			labels[i] = t.label
		}
		if i := ulTabs(c, p, "conn-tab", labels, nil, d.tab); i >= 0 {
			d.tab = i
		}
	})
	scrollbars(c, ui.Scroll(c.Key("connform")).FillWidth().Grow(1)).Children(func() {
		ui.Column(c.Key("connform-"+tabs[d.tab].key)).Gap(spaceMD-2).FillWidth().Padding(spaceMD+2, spaceLG, spaceMD, spaceLG-spaceSM).Children(func() {
			tabs[d.tab].draw(s, c, p, submit)
		})
	})
	hairline(c, p.border)
	s.connFormFooter(c, p, submit)
}

// connFormHeader 是配置页的头部：返回（有列表可回时）、标题与说明、从 URL 导入、关闭。
// 点返回只置 *back，不在这里直接离开。
func (s *Shell) connFormHeader(c *ui.Context, p tokens, back *bool) {
	d := &s.dlg
	title, sub := locales.ConnNew(), locales.ConnTypeHint()
	if !d.draft {
		title, sub = d.cfg.Name, connSubtitle(d.cfg)
		if strings.TrimSpace(title) == "" {
			title = locales.ConnEditTitle()
		}
	}
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceMD, spaceMD, spaceSM, spaceLG).FillWidth().Shrink(0).Children(func() {
		if s.canBackToList() {
			if tipBtn(c, icBack, locales.ConnBackToList()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				*back = true
			}
		}
		ui.Text(c, title).FontWeight(600).FontSize(fontTitle).SingleLine().TruncateMiddle().Shrink(0).MaxWidth(280)
		ui.Text(c, sub).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().TruncateMiddle().Grow(1).MinWidth(0)
		if d.draft && app.CapsOf(d.cfg.Type).Network {
			if toggleBtn(c, icLink, locales.ConnImportURL(), d.urlOpen).Clicked() {
				d.urlOpen = !d.urlOpen
			}
		}
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			d.open = false
		}
	})
}

// connFormFooter 是配置页底部：测试连接（结果跟在后面）、删除（仅编辑时）、取消、保存、保存并连接。
func (s *Shell) connFormFooter(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceSM+2, spaceLG).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
		if tipOn(c, footBtn(c, locales.ConnTest()), locales.ConnTest()).Disabled(d.testing).Clicked() {
			s.testConnection()
		}
		switch {
		case d.testing:
			ui.Spinner(c)
			ui.Spacer(c)
		case d.msg != "":
			k := chipErr
			if d.msgOK {
				k = chipOK
			}
			tipOn(c, chip(c, k, d.msg, true), d.msg)
			ui.Spacer(c)
		default:
			ui.Spacer(c)
		}
		if !d.draft {
			if dangerSoftBtn(c, icTrash, locales.CommonDelete()).Clicked() {
				s.deleteConnection(d.cfg)
			}
		}
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		if tipOn(c, footBtn(c, locales.CommonSave()), locales.CommonSave()+" (⌘S)").Clicked() {
			s.saveConnection(false)
		}
		if tipOn(c, footPrimary(c, locales.ConnSaveConnect()), locales.ConnSaveConnect()+" (↵)").Clicked() {
			*submit = true
		}
	})
}
