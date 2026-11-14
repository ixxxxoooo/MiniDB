package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接管理器的配置页：头部（返回、图标、名称、从 URL 导入、关闭）、表单、
// 底部的测试 / 保存 / 连接。服务器与 SSH 部分在 conn_server.go。

// connField is a row of the form: its label at the left, right-aligned.
func connField(c *ui.Context, p tokens, label string, fn func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(font2XS).TextColor(p.fgSecondary).Width(88).Shrink(0).TextAlign(ui.End)
		fn()
	})
}

// connSection heads a group of fields of the connection form.
func connSection(c *ui.Context, p tokens, title string) {
	ui.Row(c.Key("sec-"+title)).Gap(8).AlignItems(ui.Center).FillWidth().Padding(4, 0, 0, 0).Children(func() {
		ui.Text(c, strings.ToUpper(title)).FontSize(10).FontWeight(600).TextColor(p.fgMuted).Shrink(0)
		ui.Box(c).Height(1).Grow(1).Background(p.borderSubtle)
	})
}

// selectTag sets the environment and, unless the user chose a colour of their
// own, the status colour that goes with it.
func (d *connDialog) selectTag(tag string) {
	old := d.cfg.Tag
	d.cfg.Tag, d.tagSel = tag, tagLabel(tag)
	if !d.safeSet { // a production connection starts out guarded
		d.safeSel = safeModeLabel((model.ConnectionConfig{Tag: tag}).EffectiveSafeMode())
	}
	if d.cfg.Color == "" || d.cfg.Color == tagColors[old] {
		d.cfg.Color = tagColors[tag]
	}
}

// connInput 是表单里的文本框：回车等于"连接"。参数 key 区分各个输入框，v 是绑定的字符串，submit 在回车时置真。
func connInput(c *ui.Context, key string, v *string, submit *bool) ui.Element {
	in := ui.TextInput(c.Key(key), v).Grow(1).Height(sizeBtn)
	if in.Submitted() {
		*submit = true
	}
	return in
}

// connForm 画配置页。参数 c 为界面上下文，p 为主题色板，submit 在回车或点"连接"时被置为真，
// back 在点返回时被置为真（由调用方在这一帧画完后再离开配置页，因为离开会清掉表单）。
func (s *Shell) connForm(c *ui.Context, p tokens, submit, back *bool) {
	d := &s.dlg
	s.connFormHeader(c, p, back)
	hairline(c, p.borderSubtle)
	if d.urlOpen {
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Padding(6, 14, 0, 14).Shrink(0).Children(func() {
			in := ui.TextInput(c.Key("url"), &d.urlText).Placeholder(locales.ConnURLHint()).Font(mono).FontSize(font2XS).Grow(1).Height(sizeBtn).AutoFocus()
			if in.Submitted() || fieldBtn(c, icCheck, locales.CommonApply()).Clicked() {
				if !d.applyURL(d.urlText) {
					d.msg, d.msgOK = locales.ConnURLBad(), false
				} else {
					d.urlOpen, d.urlText, d.msg = false, "", ""
				}
			}
		})
	}
	scrollbars(c, ui.Scroll(c.Key("connform")).FillWidth().Grow(1)).Children(func() {
		ui.Column(c).Gap(8).FillWidth().Padding(8, 18, 12, 14).Children(func() {
			connSection(c, p, locales.ConnSectionGeneral())
			// 名称在前、数据库类型在后，同一排
			connField(c, p, locales.ConnName(), func() {
				if d.draft { // 新建时名称框直接拿到键盘
					connInput(c, "name-new", &d.cfg.Name, submit).AutoFocus()
				} else {
					connInput(c, "name", &d.cfg.Name, submit)
				}
				ui.Text(c, locales.ConnPickType()).FontSize(font2XS).TextColor(p.fgSecondary).Shrink(0).SingleLine()
				s.typeSelect(c, p)
			})
			s.connTagAndColor(c, p)
			connSection(c, p, locales.ConnSectionServer())
			s.connServerFields(c, p, submit)
		})
	})
	hairline(c, p.borderSubtle)
	s.connFormFooter(c, p, submit)
}

// connFormHeader 是配置页的头部：返回（有列表可回时）、类型图标、标题、从 URL 导入、关闭。
// 点返回只置 *back，不在这里直接离开。
func (s *Shell) connFormHeader(c *ui.Context, p tokens, back *bool) {
	d := &s.dlg
	title, sub := locales.ConnNewTitle(typeLabel(d.cfg.Type)), locales.ConnDraftHint()
	if !d.draft {
		title, sub = d.cfg.Name, connSubtitle(d.cfg)
		if strings.TrimSpace(title) == "" {
			title = locales.ConnEditTitle()
		}
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 8, 8, 8).FillWidth().Shrink(0).Children(func() {
		if s.canBackToList() {
			if tipBtn(c, icBack, locales.ConnBackToList()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				*back = true
			}
		}
		dbTile(c, d.cfg.Type, 28, connColor(c, d.cfg.Color))
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontWeight(600).FontSize(14).SingleLine().TruncateMiddle()
			ui.Text(c, sub).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		if d.draft && database.CapsOf(d.cfg.Type).Network {
			if toggleBtn(c, icLink, locales.ConnImportURL(), d.urlOpen).Clicked() {
				d.urlOpen = !d.urlOpen
			}
		}
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			d.open = false
		}
	})
}

// connTagAndColor 画环境（状态）和颜色两行。
func (s *Shell) connTagAndColor(c *ui.Context, p tokens) {
	d := &s.dlg
	t := c.Theme()
	connField(c, p, locales.ConnStatus(), func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			for _, tag := range tagOrder {
				tag := tag
				on := d.cfg.Tag == tag || (d.cfg.Tag == "" && tag == "local")
				col := ui.Hex(tagColors[tag])
				chip := ui.Row(c.Key("tag-"+tag)).Gap(6).AlignItems(ui.Center).Padding(0, 10).Height(sizeBtn-4).Radius(14).Border(1, p.borderSubtle)
				switch {
				case on:
					chip.Background(col.Alpha(0.14)).BorderColor(col)
				case chip.Hovered():
					chip.Background(p.hover)
				}
				chip.Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(col)
					ui.Text(c, tagLabel(tag)).FontSize(font2XS)
				})
				if chip.Clicked() {
					d.selectTag(tag)
				}
			}
		})
	})
	connField(c, p, locales.ConnColor(), func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			for _, col := range colorChoices {
				on := d.cfg.Color == col
				sw := ui.Button(c.Key(col), "").Label(col).Size(18, 18).Padding(0).Radius(9).Background(ui.Hex(col)).BorderWidth(0)
				if on {
					sw.BorderWidth(2).BorderColor(t.Surface)
					sw.Shadow(0, 0, 0, 2, ui.Hex(col))
				}
				if sw.Clicked() {
					d.cfg.Color = col
				}
			}
		})
	})
}

// connFormFooter 是配置页底部：测试连接（结果在中间）、删除（仅编辑时）、保存、连接。
func (s *Shell) connFormFooter(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(5, 8).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
		if tipBtn(c, icActivity, locales.ConnTest()).Disabled(d.testing).Clicked() {
			s.testConnection()
		}
		switch {
		case d.testing:
			ui.Spinner(c)
			ui.Spacer(c)
		case d.msg != "":
			col := p.danger
			if d.msgOK {
				col = p.success
			}
			tipOn(c, ui.Text(c, d.msg).FontSize(font2XS).TextColor(col).SingleLine().Grow(1).TruncateMiddle(), d.msg).Padding(0, 6)
		default:
			ui.Spacer(c)
		}
		if !d.draft {
			if tipBtn(c, icTrash, locales.CommonDelete()).Clicked() {
				s.deleteConnection(d.cfg)
			}
		}
		if tipBtn(c, icSave, locales.CommonSave()+" (⌘S)").Clicked() {
			s.saveConnection(false)
		}
		if tipOn(c, fieldPrimary(c, icPlug, locales.ConnConnect()), locales.ConnConnect()+" (↵)").Clicked() {
			*submit = true
		}
	})
}
