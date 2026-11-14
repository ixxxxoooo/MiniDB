package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"

	"minidb/locales"
)

// usersDialog 管理服务器上的账号和权限：左边是账号列表，右边是现有权限，
// 编辑时切到编辑器，改动先生成 SQL 预览，确认后才执行。
type usersDialog struct {
	open    bool
	w       *Workspace
	users   []app.UserInfo
	loading bool
	err     string
	sel     int
	grants  []string
	grantOf string // grants 属于哪个账号
	editing bool
	ed      userEditor
	busy    bool
}

func (s *Shell) openUsers() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.usr = usersDialog{open: true, w: w}
	s.reloadUsers()
}

func (s *Shell) reloadUsers() {
	d := &s.usr
	w := d.w
	d.loading = true
	var users []app.UserInfo
	var err error
	s.up.bg(func() { users, err = s.app.Databases.GetUsers(w.cfg.ID) }, func() {
		d.loading = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.err, d.users = "", users
		if d.sel >= len(users) {
			d.sel = 0
		}
		d.grantOf = ""
	})
}

// userKey 是账号在列表里的显示名。
func userKey(u app.UserInfo) string {
	if u.Host != "" {
		return u.Name + "@" + u.Host
	}
	return u.Name
}

// selectedUser 返回列表里选中的账号。
func (d *usersDialog) selectedUser() (app.UserInfo, bool) {
	if d.sel < 0 || d.sel >= len(d.users) {
		return app.UserInfo{}, false
	}
	return d.users[d.sel], true
}

// loadGrants 读出选中账号的授权。
func (s *Shell) loadGrants() {
	d := &s.usr
	u, ok := d.selectedUser()
	if !ok || d.grantOf == userKey(u) {
		return
	}
	key := userKey(u)
	d.grantOf, d.grants = key, nil
	w := d.w
	var grants []string
	var err error
	s.up.bg(func() { grants, err = s.app.Databases.GetUserGrants(w.cfg.ID, u.Name, u.Host) }, func() {
		if d.grantOf != key {
			return
		}
		if err != nil {
			d.grants = []string{errMessage(err)}
			return
		}
		d.grants = grants
	})
}

func (s *Shell) usersView(c *ui.Context) {
	d := &s.usr
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &d.open, dlgOpts{title: locales.UsersTitle(), sub: d.w.cfg.Name, width: 760, height: 560, extra: func() {
		if d.loading {
			ui.Spinner(c)
		}
		if !d.editing && tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			s.reloadUsers()
		}
	}}, func() {
		if d.err != "" {
			ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
		}
		if d.editing {
			s.userEditorView(c, p)
			return
		}
		s.userListView(c, p)
	}, func() {
		if d.editing {
			s.userEditorFooter(c)
			return
		}
		ui.Text(c, locales.UsersCount(len(d.users))).FontSize(font2XS).TextColor(p.fgMuted)
		ui.Spacer(c)
		if fieldPrimary(c, icPlus, locales.UsersNew()).Disabled(d.w.cfg.ReadOnly).Clicked() {
			s.newUserEditor()
		}
	})
}

func (s *Shell) userListView(c *ui.Context, p tokens) {
	d := &s.usr
	s.loadGrants()
	ui.Row(c).Grow(1).FillWidth().Gap(12).Children(func() {
		dlgBox(c, p).Width(250).FillHeight().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("users")).Grow(1).FillWidth()).Children(func() {
				ui.Column(c).Padding(4).Gap(1).FillWidth().Children(func() {
					for i, u := range d.users {
						i, u := i, u
						row := ui.Row(c.Key(fmt.Sprint(i))).Gap(6).Padding(5, 8).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
						switch {
						case d.sel == i:
							row.Background(p.accent.Alpha(0.14))
						case row.Hovered():
							row.Background(p.hover)
						}
						row.Children(func() {
							ui.Icon(c, icUser).FontSize(13).TextColor(p.fgMuted)
							ui.Text(c, userKey(u)).FontSize(font2XS).SingleLine().Grow(1)
							if u.Superuser {
								ui.Text(c, locales.UsersSuperuser()).FontSize(10).TextColor(p.accent)
							}
						})
						if row.Clicked() {
							d.sel = i
						}
					}
				})
			})
		})
		ui.Column(c).Grow(1).FillHeight().Gap(8).Children(func() {
			u, ok := d.selectedUser()
			if !ok {
				return
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, userKey(u)).FontWeight(600).Grow(1).SingleLine()
				if actBtn(c, icPencil, locales.UsersEdit()).Disabled(d.w.cfg.ReadOnly).Clicked() {
					s.editUserEditor()
				}
				if actBtn(c, icTrash, locales.UsersDrop()).Disabled(d.w.cfg.ReadOnly).Clicked() {
					s.dropUser(u)
				}
			})
			ui.Text(c, locales.UsersGrants()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
			dlgBox(c, p).Grow(1).FillWidth().Children(func() {
				scrollbars(c, ui.Scroll(c.Key("grants")).Grow(1).FillWidth()).Children(func() {
					ui.Column(c).Padding(8).Gap(6).FillWidth().Children(func() {
						if len(d.grants) == 0 && d.grantOf != "" {
							ui.Text(c, locales.UsersNoGrants()).FontSize(font2XS).TextColor(p.fgMuted)
						}
						for i, g := range d.grants {
							ui.Text(c.Key(fmt.Sprint(i)), g).FontSize(font2XS).Font(mono).Selectable().TextColor(p.fgSecondary)
						}
					})
				})
			})
		})
	})
}

func (s *Shell) dropUser(u app.UserInfo) {
	d := &s.usr
	w := d.w
	stmt, err := app.BuildDropUserSQL(w.cfg.Type, u.Name, u.Host)
	if err != nil {
		d.err = errMessage(err)
		return
	}
	s.askDanger(locales.UsersDrop(), locales.UsersDropConfirm(userKey(u)), locales.UsersDrop(), func() {
		s.safeGate(w, []string{stmt}, func() { s.applyUserStatements([]string{stmt}) })
	})
}

// applyUserStatements 执行语句，完成后回到列表并刷新。
func (s *Shell) applyUserStatements(stmts []string) {
	d := &s.usr
	w := d.w
	d.busy, d.err = true, ""
	var n int
	var err error
	s.up.bg(func() { n, err = s.app.Databases.ApplyUserStatements(w.cfg.ID, stmts) }, func() {
		d.busy = false
		if err != nil {
			d.err = errMessage(err)
			s.reloadUsers()
			return
		}
		d.editing = false
		s.toastOK(locales.UsersApplied(n))
		s.reloadUsers()
	})
}
