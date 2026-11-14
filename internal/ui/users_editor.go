package ui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
	"strconv"
	"strings"
)

// userEditor 是新建或修改账号时填的内容。
type userEditor struct {
	isNew    bool
	orig     app.UserInfo // 修改时的原账号
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

func (s *Shell) newUserEditor() {
	d := &s.usr
	d.ed = userEditor{isNew: true, host: "%", login: app.CapsOf(d.w.cfg.Type).RoleUsers, priv: map[string]bool{}, scopeDB: d.w.db}
	d.ed.schema = app.CapsOf(d.w.cfg.Type).DefaultSchema
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
	d.ed.schema = app.CapsOf(d.w.cfg.Type).DefaultSchema
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
func (e *userEditor) scope(dbType model.DBType) app.PrivScope {
	sc := app.PrivScope{DB: strings.TrimSpace(e.scopeDB), Table: strings.TrimSpace(e.table)}
	if app.CapsOf(dbType).RoleUsers {
		sc.Schema = strings.TrimSpace(e.schema)
	}
	return sc
}

// statements 生成要执行的语句；mask 为真时密码写成 ***，用于预览。
func (e *userEditor) statements(dbType model.DBType, mask bool) ([]string, error) {
	pg := app.CapsOf(dbType).RoleUsers
	spec := app.UserSpec{Name: strings.TrimSpace(e.name), Host: strings.TrimSpace(e.host), Password: e.password}
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
		spec.Attrs = app.PGRoleAttrs{
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
		spec.Limits = app.UserLimits{MaxQueries: lim[0], MaxUpdates: lim[1], MaxConnections: lim[2], MaxUserConns: lim[3]}
	}
	var out []string
	if e.isNew {
		st, err := app.BuildCreateUserSQL(dbType, spec)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	} else {
		st, err := app.BuildAlterUserSQL(dbType, spec)
		if err != nil {
			return nil, err
		}
		if st != "" {
			out = append(out, st)
		}
	}
	priv := func(privs []string, sc app.PrivScope) error {
		if len(privs) == 0 {
			return nil
		}
		var st string
		var err error
		if e.revoke && !e.isNew {
			st, err = app.BuildRevokeSQL(dbType, privs, sc, spec.Name, spec.Host)
		} else {
			st, err = app.BuildGrantSQL(dbType, privs, sc, spec.Name, spec.Host, e.withGrant)
		}
		if err != nil {
			return err
		}
		out = append(out, st)
		return nil
	}
	if !pg {
		if err := priv(e.checked("g:", app.MySQLGlobalPrivs), app.PrivScope{}); err != nil {
			return nil, err
		}
	}
	sc := e.scope(dbType)
	if sc.DB != "" {
		if err := priv(e.checked("s:", app.PrivilegesFor(dbType, sc)), sc); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Shell) userEditorView(c *ui.Context, p tokens) {
	d := &s.usr
	e := &d.ed
	pg := app.CapsOf(d.w.cfg.Type).RoleUsers
	tabs := []string{locales.UsersTabGeneral(), locales.UsersTabGlobal(), locales.UsersTabScoped()}
	if app.CapsOf(d.w.cfg.Type).UserResourceLimits {
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
				privChecklist(c, "g", app.MySQLGlobalPrivs, e.priv, "g:")
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
					ui.Text(c, errMessage(err)).FontSize(font2XS).TextColor(p.danger)
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
