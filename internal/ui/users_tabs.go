package ui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"

	"minidb/locales"
)

// privChecklist 把权限排成三列的复选框。
func privChecklist(c *ui.Context, key string, privs []string, sel map[string]bool, prefix string) {
	const cols = 3
	all := sel[prefix+"ALL PRIVILEGES"]
	if ui.Checkbox(c.Key(key+"all"), &all, locales.UsersAllPrivs()).Changed() {
		sel[prefix+"ALL PRIVILEGES"] = all
	}
	for i := 0; i < len(privs); i += cols {
		i := i
		ui.Row(c.Key(fmt.Sprintf("%s%d", key, i))).Gap(8).FillWidth().Children(func() {
			for j := i; j < i+cols && j < len(privs); j++ {
				name := privs[j]
				v := sel[prefix+name] || all
				if ui.Checkbox(c.Key(name), &v, name).Disabled(all).Width(200).Changed() {
					sel[prefix+name] = v
				}
			}
		})
	}
}

func (s *Shell) userGeneralTab(c *ui.Context, p tokens, pg bool) {
	e := &s.usr.ed
	ui.Row(c).Gap(12).FillWidth().Children(func() {
		dlgField(c, p, locales.UsersName(), func() {
			in := ui.TextInput(c.Key("un"), &e.name).FillWidth().Height(sizeBtn)
			if !e.isNew {
				in.Disabled(true)
			}
		})
		if !pg {
			dlgField(c, p, locales.UsersHost(), func() {
				in := ui.TextInput(c.Key("uh"), &e.host).Placeholder(locales.UsersHostHint()).FillWidth().Height(sizeBtn)
				if !e.isNew {
					in.Disabled(true)
				}
			})
		}
	})
	hint := locales.UsersPasswordEdit()
	if e.isNew {
		hint = locales.UsersPasswordNew()
	}
	ui.Row(c).Gap(12).FillWidth().Children(func() {
		dlgField(c, p, locales.UsersPassword(), func() {
			ui.TextInput(c.Key("up"), &e.password).Password().Placeholder(hint).FillWidth().Height(sizeBtn)
		})
		dlgField(c, p, locales.UsersPasswordAgain(), func() {
			ui.TextInput(c.Key("up2"), &e.again).Password().FillWidth().Height(sizeBtn)
		})
	})
	if pg {
		ui.Column(c).Gap(6).FillWidth().Children(func() {
			ui.Checkbox(c, &e.login, locales.UsersLogin())
			ui.Checkbox(c, &e.super, locales.UsersSuperuser())
			ui.Checkbox(c, &e.createDB, locales.UsersCreateDb())
			ui.Checkbox(c, &e.createRole, locales.UsersCreateRole())
		})
	}
}

// userPrivMode 是授权/收回的开关（新建账号时只能授权）。
func (s *Shell) userPrivMode(c *ui.Context, p tokens) {
	e := &s.usr.ed
	if !e.isNew {
		seg := 0
		if e.revoke {
			seg = 1
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, locales.UsersAction()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
			if ui.Segmented(c, &seg, locales.UsersGrant(), locales.UsersRevoke()).Changed() {
				e.revoke = seg == 1
			}
		})
	}
	if !e.revoke || e.isNew {
		ui.Checkbox(c, &e.withGrant, locales.UsersWithGrant())
	}
}

func (s *Shell) userScopedTab(c *ui.Context, p tokens, pg bool) {
	d := &s.usr
	e := &d.ed
	s.userPrivMode(c, p)
	ui.Row(c).Gap(12).FillWidth().Children(func() {
		dlgField(c, p, locales.UsersScopeDb(), func() {
			ui.TextInput(c.Key("usd"), &e.scopeDB).FillWidth().Height(sizeBtn)
		})
		if pg {
			dlgField(c, p, locales.UsersScopeSchema(), func() {
				ui.TextInput(c.Key("uss"), &e.schema).FillWidth().Height(sizeBtn)
			})
		}
		dlgField(c, p, locales.UsersScopeTable(), func() {
			ui.TextInput(c.Key("ust"), &e.table).Placeholder(locales.UsersScopeHint()).FillWidth().Height(sizeBtn)
		})
	})
	privChecklist(c, "s", app.PrivilegesFor(d.w.cfg.Type, e.scope(d.w.cfg.Type)), e.priv, "s:")
}

func (s *Shell) userLimitsTab(c *ui.Context, p tokens) {
	e := &s.usr.ed
	labels := []string{locales.UsersLimitQueries(), locales.UsersLimitUpdates(), locales.UsersLimitConns(), locales.UsersLimitUser()}
	for i, l := range labels {
		i, l := i, l
		dlgField(c, p, l, func() {
			ui.TextInput(c.Key(fmt.Sprint("ul", i)), &e.limits[i]).FillWidth().Height(sizeBtn)
		})
	}
	ui.Text(c, locales.UsersLimitHint()).FontSize(font2XS).TextColor(p.fgMuted)
}
