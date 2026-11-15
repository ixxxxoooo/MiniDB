package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"

	"minidb/locales"
)

// usersDialog 是「用户与权限」的状态：页面（users_page.go）左边是账号列表，右边是现有权限；
// 新建 / 编辑时弹出编辑器对话框（usersView），改动先生成 SQL 预览，确认后才执行。open 表示页面开着。
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
	query   string // 左边列表的搜索词
	pageTab int    // 右边的标签：0 权限矩阵，1 SQL 预览
}

func (s *Shell) openUsers() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.usr = usersDialog{open: true, w: w}
	w.showUsersTab()
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

// usersView 画编辑器对话框：新建 / 编辑账号时才有，页面本身是工作区里的标签（users_page.go）。
func (s *Shell) usersView(c *ui.Context) {
	d := &s.usr
	if !d.editing {
		return
	}
	p := s.tokens(c)
	open := true
	s.dialog(c, &open, dlgOpts{title: locales.UsersTitle(), sub: d.w.cfg.Name, width: 760, height: 560}, func() {
		if d.err != "" {
			ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
		}
		s.userEditorView(c, p)
	}, func() { s.userEditorFooter(c) })
	if !open {
		d.editing = false
	}
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
