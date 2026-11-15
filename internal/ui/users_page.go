package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 「用户与权限」页（设计稿 users 页）：和监控一样是工作区里的一个标签。
// 顶部条：标题、连接名、新建用户、刷新；左边是带搜索的账号列表（圆形头像 + 名字 / @主机 + 超级用户芯片）；
// 右边是选中账号的页头（头像、名字、编辑 / 删除）、「权限 n / SQL 预览」下划线标签，权限是只读的授权矩阵（users_matrix.go）。
// 新建 / 编辑仍是对话框（users_editor.go）：所有改动先出 SQL 预览、确认后才执行。

// usersTab 是「用户与权限」标签；状态都在 Shell 的 s.usr 里，标签本身只记属于哪个工作区。
type usersTab struct{ w *Workspace }

func (u *usersTab) title() string       { return locales.UsersTitle() }
func (u *usersTab) icon() *ui.SVG       { return icUsers }
func (u *usersTab) objKind() ObjectKind { return KindTable }
func (u *usersTab) pending() int        { return 0 }
func (u *usersTab) view(c *ui.Context, p tokens) {
	u.w.sh.usersPage(c, p, u.w)
}

// showUsersTab 把「用户与权限」标签切到前台；还没有就新开一个。
func (w *Workspace) showUsersTab() {
	for i, t := range w.tabs {
		if _, ok := t.(*usersTab); ok {
			w.active = i
			return
		}
	}
	w.tabs = append(w.tabs, &usersTab{w: w})
	w.active = len(w.tabs) - 1
}

// onUsersTab 报告当前前台标签是不是「用户与权限」（活动栏的按钮据此高亮）。
func (w *Workspace) onUsersTab() bool {
	_, ok := w.activeTab().(*usersTab)
	return ok
}

// usersPage 画整页。参数：w 是标签所属的工作区；状态在别的连接上时（切过连接）先换成这个连接并重新读账号。
func (s *Shell) usersPage(c *ui.Context, p tokens, w *Workspace) {
	d := &s.usr
	if d.w != w {
		s.usr = usersDialog{open: true, w: w}
		s.reloadUsers()
	}
	s.loadGrants()
	ui.Toolbar(c.Key("usr-bar"), func() {
		ui.Text(c, locales.UsersTitle()).FontSize(fontBody).FontWeight(600).SingleLine().Shrink(0)
		ui.Text(c, w.cfg.Name).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
		ui.Spacer(c)
		if d.loading {
			ui.Spinner(c)
		}
		if fieldPrimary(c, icPlus, locales.UsersNew()).Disabled(w.cfg.ReadOnly).Clicked() {
			s.newUserEditor()
		}
		if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			s.reloadUsers()
		}
	}).Gap(spaceSM).Padding(0, spaceSM+2, 0, spaceLG).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Background(p.surface)
	hairline(c, p.border)
	ui.Row(c.Key("usr-body")).Grow(1).FillWidth().MinHeight(0).Children(func() {
		s.usersList(c, p)
		ui.Box(c).Width(1).FillHeight().Shrink(0).Background(p.border)
		s.usersDetail(c, p)
	})
}

// visibleUsers 返回通过搜索词的账号在 d.users 里的下标（名字或主机包含搜索词，不分大小写）。
func (d *usersDialog) visibleUsers() []int {
	q := strings.ToLower(strings.TrimSpace(d.query))
	var out []int
	for i, u := range d.users {
		if q == "" || strings.Contains(strings.ToLower(userKey(u)), q) {
			out = append(out, i)
		}
	}
	return out
}

// usersList 是左边：搜索框 + 账号列表。
func (s *Shell) usersList(c *ui.Context, p tokens) {
	d := &s.usr
	ui.Column(c.Key("usr-list")).Width(sizeUsersList).FillHeight().Shrink(0).Gap(spaceSM).Padding(spaceMD).Background(p.surfaceSecondary).Children(func() {
		textField(c.Key("usr-q"), &d.query).Placeholder(locales.UsersSearch()).FillWidth().Height(sizeInput)
		scrollbars(c, ui.Scroll(c.Key("users")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				vis := d.visibleUsers()
				if len(vis) == 0 && !d.loading {
					ui.Text(c, locales.UsersNoMatch()).FontSize(font2XS).TextColor(p.fgMuted).Padding(spaceMD)
				}
				for _, i := range vis {
					i, u := i, d.users[i]
					row := ui.Row(c.Key(fmt.Sprint(i))).Gap(spaceSM-2).Padding(spaceSM-3, spaceSM).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
					switch {
					case d.sel == i:
						row.Background(p.accentSoft)
					case row.Hovered():
						row.Background(p.hover)
					}
					row.Children(func() { userLabel(c, p, u, false) })
					if row.Clicked() {
						d.sel = i
					}
				}
			})
		})
	})
}

// usersDetail 是右边：选中账号的页头、标签和内容。
func (s *Shell) usersDetail(c *ui.Context, p tokens) {
	d := &s.usr
	ui.Column(c.Key("usr-detail")).Grow(1).FillHeight().MinWidth(0).Children(func() {
		u, ok := d.selectedUser()
		if !ok {
			return
		}
		ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Padding(spaceMD, spaceLG, spaceSM, spaceLG).Shrink(0).Children(func() {
			userLabel(c, p, u, true)
			if actBtn(c, icPencil, locales.UsersEdit()).Disabled(d.w.cfg.ReadOnly).Clicked() {
				s.editUserEditor()
			}
			if dangerSoftBtn(c, icTrash, locales.UsersDrop()).Disabled(d.w.cfg.ReadOnly).Clicked() {
				s.dropUser(u)
			}
		})
		rows := parseGrants(d.grants, grantColumns(app.CapsOf(d.w.cfg.Type).RoleUsers))
		labels := []string{locales.UsersGrantsTab(), locales.UsersPreview()}
		if i := ulTabs(c, p, "usr-tabs", labels, []int{len(rows)}, d.pageTab); i >= 0 {
			d.pageTab = i
		}
		body := ui.Column(c.Key("usr-pane")).Grow(1).FillWidth().MinHeight(0).Gap(spaceSM).Padding(spaceMD, spaceLG)
		body.Children(func() {
			if d.pageTab == 1 {
				s.usersSQL(c, p)
				return
			}
			s.usersMatrix(c, p, rows)
		})
	})
}

// usersSQL 是「SQL 预览」标签：服务器上的原始授权语句。
func (s *Shell) usersSQL(c *ui.Context, p tokens) {
	d := &s.usr
	dlgBox(c, p).Grow(1).FillWidth().Children(func() {
		scrollbars(c, ui.Scroll(c.Key("grants")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Padding(spaceSM).Gap(spaceSM - 2).FillWidth().Children(func() {
				if len(d.grants) == 0 && d.grantOf != "" {
					ui.Text(c, locales.UsersNoGrants()).FontSize(font2XS).TextColor(p.fgMuted)
				}
				for i, g := range d.grants {
					ui.Text(c.Key(fmt.Sprint(i)), g).FontSize(font2XS).Font(mono).Selectable().TextColor(p.fgSecondary)
				}
			})
		})
	})
}

// usersMatrix 是「权限」标签：只读的授权矩阵 + 「允许其再授权」一行 + 只读说明。
func (s *Shell) usersMatrix(c *ui.Context, p tokens, rows []grantRow) {
	d := &s.usr
	cols := grantColumns(app.CapsOf(d.w.cfg.Type).RoleUsers)
	if len(rows) == 0 {
		if d.grantOf != "" {
			ui.Text(c, locales.UsersNoGrants()).FontSize(font2XS).TextColor(p.fgMuted)
		}
		return
	}
	option := false
	for _, r := range rows {
		option = option || r.option
	}
	dlgBox(c, p).FillWidth().Shrink(1).MinHeight(0).Children(func() {
		ui.Row(c).Gap(0).Padding(0, spaceMD).Height(sizeGrantRow).AlignItems(ui.Center).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
			ui.Text(c, locales.UsersObject()).FontSize(font2XS).FontWeight(600).TextColor(p.fgMuted).Grow(1).MinWidth(0)
			for _, col := range cols {
				ui.Text(c, col).Width(sizeGrantCol).FontSize(fontMicro).FontWeight(600).TextColor(p.fgMuted).TextAlign(ui.Center).Shrink(0)
			}
			ui.Text(c, locales.UsersOther()).Width(sizeGrantOther).FontSize(font2XS).FontWeight(600).TextColor(p.fgMuted).Shrink(0)
		})
		hairline(c, p.borderSubtle)
		scrollbars(c, ui.Scroll(c.Key("matrix")).FillWidth().Grow(1)).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for i, r := range rows {
					ui.Row(c.Key(fmt.Sprint("g", i))).Padding(0, spaceMD).Height(sizeGrantRow).AlignItems(ui.Center).FillWidth().Children(func() {
						ui.Text(c, r.object).FontSize(font2XS).Font(mono).FontWeight(600).SingleLine().Grow(1).MinWidth(0).TruncateMiddle()
						for _, col := range cols {
							ui.Row(c).Width(sizeGrantCol).Shrink(0).Center().Children(func() {
								if r.privs[col] {
									ui.Icon(c, icCheck).FontSize(btnFieldIcon).TextColor(p.accent)
								}
							})
						}
						ui.Text(c, strings.Join(r.extra, ", ")).Width(sizeGrantOther).FontSize(fontCaption).Font(mono).TextColor(p.fgSecondary).SingleLine().TruncateMiddle().Shrink(0)
					})
					hairline(c, p.borderSubtle)
				}
			})
		})
	})
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Shrink(0).Padding(spaceXS, 0).Children(func() {
		ui.Text(c, locales.UsersWithGrant()).FontSize(font2XS).Grow(1)
		if option {
			chip(c, chipOK, locales.UsersGranted(), false)
		} else {
			chip(c, chipNeutral, locales.UsersNotGranted(), false)
		}
	})
	ui.Text(c, locales.UsersMatrixHint()).FontSize(fontCaption).TextColor(p.fgMuted).FillWidth()
}
