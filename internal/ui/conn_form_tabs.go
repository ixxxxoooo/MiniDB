package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 配置页的标签：常规（名称、环境、服务器、开关）、SSH 隧道、SSL、高级（文件夹）。
// 本地文件库（SQLite）只有「常规」；没有文件夹时不显示「高级」。

// connTab 是配置页的一个标签：key 区分内容区，draw 画内容。
type connTab struct {
	key, label string
	draw       func(s *Shell, c *ui.Context, p tokens, submit *bool)
}

// tabs 是当前类型下可用的标签，顺序固定：常规、SSH 隧道、SSL、高级。
func (d *connDialog) tabs() []connTab {
	out := []connTab{{"general", locales.ConnTabGeneral(), (*Shell).connGeneralTab}}
	if app.CapsOf(d.cfg.Type).Network {
		out = append(out,
			connTab{"ssh", locales.ConnTabSSH(), (*Shell).connSSHTab},
			connTab{"ssl", locales.ConnTabSSL(), (*Shell).connSSLTab})
	}
	if len(d.folders) > 0 {
		out = append(out, connTab{"adv", locales.ConnTabAdvanced(), (*Shell).connAdvancedTab})
	}
	return out
}

// connGeneralTab 是「常规」：名称、环境、服务器字段，下面是只读 / AI / 安全模式三个开关。
func (s *Shell) connGeneralTab(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	connField(c, p, locales.ConnName(), func() {
		if d.draft { // 新建时名称框直接拿到键盘
			connInput(c, "name-new", &d.cfg.Name, submit).AutoFocus()
		} else {
			connInput(c, "name", &d.cfg.Name, submit)
		}
	})
	s.connTagAndColor(c, p)
	s.connServerFields(c, p, submit)
	hairline(c, p.borderSubtle)
	ui.Row(c).Gap(spaceLG+spaceSM).Padding(spaceXS, 0, 0, 88+spaceMD).AlignItems(ui.Center).FillWidth().Children(func() {
		connSwitch(c, p, &d.cfg.ReadOnly, locales.ConnReadOnly())
		connSwitch(c, p, &d.cfg.AIMetadataOnly, locales.ConnAiMetadataOnly())
	})
	connField(c, p, locales.ConnSafeMode(), func() {
		if safeSelect(c.Key("safe"), p, &d.safeSel, 160) {
			d.safeSet = true
		}
		ui.Text(c, safeModeDesc(safeModeFromLabel(d.safeSel))).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
	})
}

// connSwitch 是「开关 + 文字」，用在连接表单里的各个布尔选项。
func connSwitch(c *ui.Context, p tokens, on *bool, label string) {
	ui.Row(c.Key("sw-" + label)).Gap(spaceSM).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Switch(c, on)
		ui.Text(c, label).FontSize(fontDense).SingleLine()
	})
}

// connSSHTab 是「SSH 隧道」：一个开关，打开后是主机、账号、认证方式。
func (s *Shell) connSSHTab(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	connField(c, p, "", func() { connSwitch(c, p, &d.cfg.SSH.Enabled, locales.ConnSSH()) })
	if d.cfg.SSH.Enabled {
		s.connSSHFields(c, p, submit)
	}
}

// connSSLTab 是「SSL」：SSL 模式；这种数据库没有选项时给一句说明。
func (s *Shell) connSSLTab(c *ui.Context, p tokens, _ *bool) {
	d := &s.dlg
	modes := app.SSLModesOf(d.cfg.Type)
	if len(modes) == 0 {
		ui.Text(c, locales.ConnSSLNone()).FontSize(fontDense).TextColor(p.fgMuted)
		return
	}
	connField(c, p, locales.ConnSSLMode(), func() {
		ui.Select(c, &d.cfg.SSLMode, modes).MinWidth(0).Width(180).Height(sizeInputLg)
	})
}

// connAdvancedTab 是「高级」：连接所在的文件夹。
func (s *Shell) connAdvancedTab(c *ui.Context, p tokens, _ *bool) {
	s.connFolderField(c, p)
}

// connTagAndColor 画环境（状态）一行：一条分段开关（LOCAL / DEV / TEST / STG / PROD），选中哪个由环境决定，不再单独选颜色。
func (s *Shell) connTagAndColor(c *ui.Context, p tokens) {
	d := &s.dlg
	connField(c, p, locales.ConnStatus(), func() {
		labels, sel := make([]string, len(tagOrder)), 0
		for i, tag := range tagOrder {
			labels[i] = tagShort(tag)
			if d.cfg.Tag == tag || (d.cfg.Tag == "" && tag == "local") {
				sel = i
			}
		}
		if segmented(c.Key("conn-tag"), &sel, labels...) {
			d.selectTag(tagOrder[sel])
		}
		ui.Text(c, tagLabel(tagOrder[sel])).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	})
}
