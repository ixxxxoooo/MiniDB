package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/locales"
)

// usersDialog 管理服务器上的账号和权限：左边是账号列表，右边是现有权限，
// 编辑时切到编辑器，改动先生成 SQL 预览，确认后才执行。
type usersDialog struct {
	open    bool
	w       *Workspace
	users   []database.UserInfo
	loading bool
	err     string
	sel     int
	grants  []string
	grantOf string // grants 属于哪个账号
	editing bool
	ed      userEditor
	busy    bool
}

// userEditor 是新建或修改账号时填的内容。
type userEditor struct {
	isNew    bool
	orig     database.UserInfo // 修改时的原账号
	name     string
	host     string
	password string
	again    string
	tab      int
	// PostgreSQL 角色属性
	login, super, createDB, createRole bool
	// 权限：键是 "g:权限"（全局）或 "s:权限"（库表范围）
	priv      map[string]bool
	revoke    bool // 勾选的权限是收回而不是授权
	withGrant bool
	scopeDB   string
	schema    string
	table     string
	limits    [4]string // 查询数、更新数、连接数、同时连接数
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
	var users []database.UserInfo
	var err error
	s.up.bg(func() { users, err = s.app.Databases.GetUsers(w.cfg.ID) }, func() {
		d.loading = false
		if err != nil {
			d.err = err.Error()
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
func userKey(u database.UserInfo) string {
	if u.Host != "" {
		return u.Name + "@" + u.Host
	}
	return u.Name
}

// selectedUser 返回列表里选中的账号。
func (d *usersDialog) selectedUser() (database.UserInfo, bool) {
	if d.sel < 0 || d.sel >= len(d.users) {
		return database.UserInfo{}, false
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
			d.grants = []string{err.Error()}
			return
		}
		d.grants = grants
	})
}

func (s *Shell) newUserEditor() {
	d := &s.usr
	d.ed = userEditor{isNew: true, host: "%", login: d.w.cfg.Type == model.Postgres, priv: map[string]bool{}, scopeDB: d.w.db}
	if d.w.cfg.Type == model.Postgres {
		d.ed.schema = "public"
	}
	d.editing = true
}

func (s *Shell) editUserEditor() {
	d := &s.usr
	u, ok := d.selectedUser()
	if !ok {
		return
	}
	d.ed = userEditor{orig: u, name: u.Name, host: u.Host, login: u.CanLogin, super: u.Superuser, createDB: u.CreateDB,
		createRole: u.CreateRole, priv: map[string]bool{}, scopeDB: d.w.db}
	if d.w.cfg.Type == model.Postgres {
		d.ed.schema = "public"
	}
	d.editing = true
}

// parseLimit 把输入框里的限制读成数字；空串表示不改。
func parseLimit(s string) (*int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("%q 不是有效的数字", s)
	}
	return &n, nil
}

// checked 返回勾选的、并且在当前范围里允许的权限，保持清单顺序。
func (e *userEditor) checked(prefix string, allowed []string) []string {
	var out []string
	for _, p := range allowed {
		if e.priv[prefix+p] {
			out = append(out, p)
		}
	}
	if e.priv[prefix+"ALL PRIVILEGES"] {
		return []string{"ALL PRIVILEGES"}
	}
	return out
}

// scope 是库表页的授权范围。
func (e *userEditor) scope(dbType model.DBType) database.PrivScope {
	sc := database.PrivScope{DB: strings.TrimSpace(e.scopeDB), Table: strings.TrimSpace(e.table)}
	if dbType == model.Postgres {
		sc.Schema = strings.TrimSpace(e.schema)
	}
	return sc
}

// statements 生成要执行的语句；mask 为真时密码写成 ***，用于预览。
func (e *userEditor) statements(dbType model.DBType, mask bool) ([]string, error) {
	pg := dbType == model.Postgres
	spec := database.UserSpec{Name: strings.TrimSpace(e.name), Host: strings.TrimSpace(e.host), Password: e.password}
	if mask && spec.Password != "" {
		spec.Password = "***"
	}
	if spec.Name == "" {
		return nil, fmt.Errorf("%s", locales.UsersNeedName())
	}
	if pg {
		// 修改时只写改动过的属性；新建时全写
		set := func(now, was bool) *bool {
			if e.isNew || now != was {
				v := now
				return &v
			}
			return nil
		}
		spec.Attrs = database.PGRoleAttrs{
			Login: set(e.login, e.orig.CanLogin), Superuser: set(e.super, e.orig.Superuser),
			CreateDB: set(e.createDB, e.orig.CreateDB), CreateRole: set(e.createRole, e.orig.CreateRole),
		}
	} else {
		var err error
		lim := make([]*int, 4)
		for i := range lim {
			if lim[i], err = parseLimit(e.limits[i]); err != nil {
				return nil, err
			}
		}
		spec.Limits = database.UserLimits{MaxQueries: lim[0], MaxUpdates: lim[1], MaxConnections: lim[2], MaxUserConns: lim[3]}
	}
	var out []string
	if e.isNew {
		st, err := database.BuildCreateUserSQL(dbType, spec)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	} else {
		st, err := database.BuildAlterUserSQL(dbType, spec)
		if err != nil {
			return nil, err
		}
		if st != "" {
			out = append(out, st)
		}
	}
	priv := func(privs []string, sc database.PrivScope) error {
		if len(privs) == 0 {
			return nil
		}
		var st string
		var err error
		if e.revoke && !e.isNew {
			st, err = database.BuildRevokeSQL(dbType, privs, sc, spec.Name, spec.Host)
		} else {
			st, err = database.BuildGrantSQL(dbType, privs, sc, spec.Name, spec.Host, e.withGrant)
		}
		if err != nil {
			return err
		}
		out = append(out, st)
		return nil
	}
	if !pg {
		if err := priv(e.checked("g:", database.MySQLGlobalPrivs), database.PrivScope{}); err != nil {
			return nil, err
		}
	}
	sc := e.scope(dbType)
	if sc.DB != "" {
		if err := priv(e.checked("s:", database.PrivilegesFor(dbType, sc)), sc); err != nil {
			return nil, err
		}
	}
	return out, nil
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

func (s *Shell) dropUser(u database.UserInfo) {
	d := &s.usr
	w := d.w
	stmt, err := database.BuildDropUserSQL(w.cfg.Type, u.Name, u.Host)
	if err != nil {
		d.err = err.Error()
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
			d.err = err.Error()
			s.reloadUsers()
			return
		}
		d.editing = false
		s.toastOK(locales.UsersApplied(n))
		s.reloadUsers()
	})
}

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

func (s *Shell) userEditorView(c *ui.Context, p tokens) {
	d := &s.usr
	e := &d.ed
	pg := d.w.cfg.Type == model.Postgres
	tabs := []string{locales.UsersTabGeneral(), locales.UsersTabGlobal(), locales.UsersTabScoped()}
	if database.CapsOf(d.w.cfg.Type).UserResourceLimits {
		tabs = append(tabs, locales.UsersTabLimits())
	}
	if pg {
		tabs = []string{locales.UsersTabGeneral(), locales.UsersTabScoped()}
	}
	if e.tab >= len(tabs) {
		e.tab = 0
	}
	ui.Segmented(c, &e.tab, tabs...)
	scrollbars(c, ui.Scroll(c.Key("ued")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).Gap(10).Padding(0, 2).FillWidth().Children(func() {
			switch tabs[e.tab] {
			case locales.UsersTabGeneral():
				s.userGeneralTab(c, p, pg)
			case locales.UsersTabGlobal():
				s.userPrivMode(c, p)
				privChecklist(c, "g", database.MySQLGlobalPrivs, e.priv, "g:")
			case locales.UsersTabScoped():
				s.userScopedTab(c, p, pg)
			default:
				s.userLimitsTab(c, p)
			}
		})
	})
	// SQL 预览
	ui.Text(c, locales.UsersPreview()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
	dlgBox(c, p).FillWidth().Height(110).Children(func() {
		scrollbars(c, ui.Scroll(c.Key("uprev")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Padding(8).Gap(2).FillWidth().Children(func() {
				stmts, err := e.statements(d.w.cfg.Type, true)
				switch {
				case err != nil:
					ui.Text(c, err.Error()).FontSize(font2XS).TextColor(p.danger)
				case len(stmts) == 0:
					ui.Text(c, locales.UsersNothing()).FontSize(font2XS).TextColor(p.fgMuted)
				}
				for i, st := range stmts {
					ui.Text(c.Key(fmt.Sprint(i)), st+";").FontSize(font2XS).Font(mono).Selectable()
				}
			})
		})
	})
}

func (s *Shell) userEditorFooter(c *ui.Context) {
	d := &s.usr
	e := &d.ed
	if d.busy {
		ui.Spinner(c)
	}
	if footBtn(c, locales.UsersBack()).Clicked() {
		d.editing = false
	}
	stmts, err := e.statements(d.w.cfg.Type, false)
	if footPrimary(c, locales.UsersApply()).Disabled(d.busy || err != nil || len(stmts) == 0).Clicked() {
		if e.password != e.again {
			d.err = locales.UsersPasswordMismatch()
			return
		}
		d.err = ""
		s.safeGate(d.w, stmts, func() { s.applyUserStatements(stmts) })
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
	privChecklist(c, "s", database.PrivilegesFor(d.w.cfg.Type, e.scope(d.w.cfg.Type)), e.priv, "s:")
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
