package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 配置页的"服务器"部分：主机、账号、密码、安全模式，以及 SSH 隧道。

// connServerFields 画服务器分组的各行。sqlite 只有一个文件；其余类型有主机、端口、账号和可选的 SSH。
// 参数 c 为界面上下文，p 为主题色板，submit 在文本框回车时被置为真。
func (s *Shell) connServerFields(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	if !app.CapsOf(d.cfg.Type).Network {
		connField(c, p, locales.ConnFile(), func() {
			connInput(c, "file", &d.cfg.Database, submit).Placeholder("/path/to/file.db")
			if fieldBtn(c, icFolder, locales.ConnSSHChoose()).Clicked() {
				s.pickSQLiteFile()
			}
		})
	} else {
		connField(c, p, locales.ConnHost(), func() {
			connInput(c, "host", &d.cfg.Host, submit)
			ui.Text(c, locales.ConnPort()).FontSize(font2XS).TextColor(p.fgSecondary)
			connInput(c, "port", &d.port, submit).Grow(0).Width(84)
		})
		connField(c, p, locales.ConnUser(), func() { connInput(c, "user", &d.cfg.User, submit) })
		connField(c, p, locales.ConnPasswordMode(), func() {
			ui.Segmented(c, &d.pwMode, locales.ConnPasswordKeychain(), locales.ConnPasswordAsk(), locales.ConnPasswordNone())
		})
		if d.pwMode != 2 {
			connField(c, p, locales.ConnPassword(), func() {
				in := ui.TextInput(c.Key("pass"), &d.password).Password().Grow(1).Height(sizeBtn)
				if in.Submitted() {
					*submit = true
				}
				switch {
				case d.pwMode == 1:
					in.Placeholder(locales.ConnPasswordTestOnly())
				case d.editing && d.cfg.HasPassword:
					in.Placeholder(locales.ConnPasswordSaved())
				}
			})
		}
		if d.editing && d.cfg.HasPassword && d.pwMode == 0 {
			connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.ClearPassword, locales.ConnClearPassword()) })
		}
		connField(c, p, locales.ConnDatabase(), func() { connInput(c, "db", &d.cfg.Database, submit) })
		if modes := app.SSLModesOf(d.cfg.Type); len(modes) > 0 {
			connField(c, p, locales.ConnSSLMode(), func() {
				ui.Select(c, &d.cfg.SSLMode, modes).MinWidth(0).Width(160).Height(sizeBtn)
			})
		}
	}
	connField(c, p, locales.ConnSafeMode(), func() {
		if safeSelect(c.Key("safe"), p, &d.safeSel, 160) {
			d.safeSet = true
		}
		ui.Text(c, safeModeDesc(safeModeFromLabel(d.safeSel))).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
	})
	connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.ReadOnly, locales.ConnReadOnly()) })
	connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.AIMetadataOnly, locales.ConnAiMetadataOnly()) })
	if app.CapsOf(d.cfg.Type).Network {
		connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.SSH.Enabled, locales.ConnSSH()) })
		if d.cfg.SSH.Enabled {
			s.connSSHFields(c, p, submit)
		}
	}
}

// connSSHFields 画 SSH 隧道的一块：主机、端口、用户、认证方式、私钥、口令。
func (s *Shell) connSSHFields(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	ui.Column(c).Gap(10).Padding(12).FillWidth().Radius(radiusPanel).Background(p.surfaceSecondary).Border(1, p.borderSubtle).Children(func() {
		connField(c, p, locales.ConnSSHHost(), func() {
			connInput(c, "sshhost", &d.cfg.SSH.Host, submit)
			ui.Text(c, locales.ConnSSHPort()).FontSize(font2XS).TextColor(p.fgSecondary)
			connInput(c, "sshport", &d.sshPort, submit).Grow(0).Width(84)
		})
		connField(c, p, locales.ConnSSHUser(), func() { connInput(c, "sshuser", &d.cfg.SSH.User, submit) })
		connField(c, p, locales.ConnSSHAuth(), func() {
			auth := 0
			if d.sshAuth == "key" {
				auth = 1
			}
			if ui.Segmented(c, &auth, locales.ConnSSHPassword(), locales.ConnSSHKey()).Changed() {
				d.sshAuth = []string{"password", "key"}[auth]
			}
		})
		if d.sshAuth == "key" {
			connField(c, p, locales.ConnSSHKeyFile(), func() {
				connInput(c, "sshkey", &d.cfg.SSH.KeyPath, submit)
				if fieldBtn(c, icFolder, locales.ConnSSHChoose()).Clicked() {
					s.pickSSHKey()
				}
			})
		}
		connField(c, p, locales.ConnSSHSecret(), func() {
			in := ui.TextInput(c.Key("sshsecret"), &d.secret).Password().Grow(1).Height(sizeBtn)
			if in.Submitted() {
				*submit = true
			}
			if d.editing && d.cfg.SSH.HasSecret {
				in.Placeholder(locales.ConnPasswordSaved())
			}
		})
	})
}
