package ui

import (
	"net"
	"net/url"
	"strconv"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接管理器的列表页：已保存的连接，列表下方是"再添加新的连接"。
// 点一行直接连接；悬停出现 ✎ 编辑和 🗑 删除；↑↓ 移动光标，↵ 连接；右键还有复制等操作。

// connList 画列表页。参数 c 为界面上下文，p 为主题色板；无返回值。
func (s *Shell) connList(c *ui.Context, p tokens) {
	d := &s.dlg
	ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(8, 8, 6, 14).FillWidth().Shrink(0).Children(func() {
		logo(c, 24)
		ui.Text(c, locales.WelcomeTitle()).FontWeight(700).FontSize(14)
		if n := len(s.conns); n > 0 {
			ui.Text(c, locales.ConnListSub(n)).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
		}
		ui.Spacer(c)
		if tipBtn(c, icPlus, locales.ConnNew()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			s.startDraft(app.DefaultDBType())
		}
		tipBtn(c, icMore, locales.ConnMore()).Size(sizeBtnSm, sizeBtnSm).Menu(func(m *ui.Menu) {
			if m.Item(locales.ConnRestore()).Chosen() {
				s.importConnections("")
			}
			if m.Item(locales.ConnBackup()).Chosen() {
				s.exportConnections("")
			}
			m.Separator()
			if m.Item(locales.ConnFolderNew()).Chosen() {
				s.newFolder()
			}
		})
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			d.open = false
		}
	})
	shown := s.filteredConns()
	cur := s.listCursor(shown)
	if len(s.conns) > 0 {
		s.connSearch(c, p, shown, cur)
	}
	scrollbars(c, ui.Scroll(c.Key("conns")).FillWidth().Grow(1).MinHeight(0)).Children(func() {
		ui.Column(c).Gap(1).FillWidth().Padding(2, 8, 8, 8).Children(func() {
			if len(shown) == 0 {
				s.connEmptyState(c, p)
			}
			s.connGroupsView(c, p, cur)
		})
	})
	hairline(c, p.borderSubtle)
	ui.Column(c).FillWidth().Padding(6, 14).Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Text(c, locales.ConnListHint()).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
	})
}

// listCursor 是光标在 shown 里的下标：停在 s.selected 上，选中的连接不在列表里就落在第一条。
func (s *Shell) listCursor(shown []model.ConnectionConfig) int {
	for i, cfg := range shown {
		if cfg.ID == s.selected {
			return i
		}
	}
	return 0
}

// connSearch 是列表上方的搜索框：↑↓ 移动光标（搜索框有焦点时也一样），↵ 连接光标所在的那条。
func (s *Shell) connSearch(c *ui.Context, p tokens, shown []model.ConnectionConfig, cur int) {
	d := &s.dlg
	move := func(delta int) {
		if len(shown) > 0 {
			s.selected = shown[min(max(cur+delta, 0), len(shown)-1)].ID
		}
	}
	var search ui.Element
	ui.Column(c).FillWidth().Padding(0, 14, 6, 14).Shrink(0).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn).Radius(radiusBtn).Background(p.surfaceSecondary).Border(1, p.borderSubtle).FillWidth().Children(func() {
			ui.Icon(c, icSearch).FontSize(13).TextColor(p.fgMuted)
			search = ui.TextInput(c.Key("conn-search"), &d.search).Placeholder(locales.ConnSearchList()).FontSize(font2XS+1).Grow(1).Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Mods != 0:
						return false
					case k.Key == ui.KeyDown:
						move(1)
					case k.Key == ui.KeyUp:
						move(-1)
					default:
						return false
					}
					s.redraw()
					return true
				})
		})
	})
	if !d.focusSet { // 进入列表页时让搜索框拿到键盘：↑↓ 选择，↵ 连接
		d.focusSet = true
		search.Focus()
	}
	if search.Submitted() && cur < len(shown) {
		d.open = false
		s.openConnection(shown[cur])
	}
}

// connEmptyState 是列表没有内容时的提示：没有任何连接，或搜索没有命中。
func (s *Shell) connEmptyState(c *ui.Context, p tokens) {
	ui.Column(c).FillWidth().Padding(28, 20, 20, 20).Gap(6).AlignItems(ui.Center).Children(func() {
		if len(s.conns) > 0 {
			ui.Text(c, locales.ConnNoMatch()).FontSize(font2XS).TextColor(p.fgMuted)
			return
		}
		ui.Icon(c, icDatabase).FontSize(26).TextColor(p.fgMuted)
		ui.Text(c, locales.ConnEmpty()).FontSize(13).FontWeight(600)
		ui.Text(c, locales.ConnEmptyHint()).FontSize(font2XS).TextColor(p.fgMuted)
	})
}

// connRow 是列表里的一条连接。cursor 表示光标停在这一行。
func (s *Shell) connRow(c *ui.Context, p tokens, cfg model.ConnectionConfig, cursor bool) {
	d := &s.dlg
	row := ui.Row(c.Key(cfg.ID)).Gap(10).Padding(5, 6, 5, 8).Radius(radiusPanel).AlignItems(ui.Center).FillWidth()
	hovered := row.Hovered()
	row.Drag(connDrag{cfg.ID})
	if d, ok := ui.Drop[connDrag](row); ok {
		defer s.placeConn(d.id, cfg.FolderID, cfg.ID)
	}
	_, dropping := ui.DragOver[connDrag](row)
	switch {
	case dropping:
		row.Background(p.accent.Alpha(0.16))
	case cursor:
		row.Background(p.accent.Alpha(0.12))
	case hovered:
		row.Background(p.hover)
	}
	var del, edit bool
	row.Children(func() {
		dbTile(c, cfg.Type, 30, connColor(c, cfgColor(cfg)))
		ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, cfg.Name).FontSize(font2XS + 1).FontWeight(600).SingleLine()
				if cfg.Tag != "" && cfg.Tag != "local" {
					ui.Text(c, tagLabel(cfg.Tag)).FontSize(10).FontWeight(500).TextColor(tagColor(c, cfg.Tag)).Padding(0, 5).Radius(4).Background(tagColor(c, cfg.Tag).Alpha(0.14))
				}
				if cfg.ReadOnly {
					ui.Icon(c, icLock).FontSize(11).TextColor(p.fgMuted)
				}
			})
			ui.Text(c, connSubtitle(cfg)).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		switch {
		case s.connecting[cfg.ID]:
			ui.Spinner(c)
		case hovered:
			edit = tipBtn(c.Key("e"), icPencil, locales.CommonEdit()).Size(sizeBtnSm, sizeBtnSm).Clicked()
			del = tipBtn(c.Key("d"), icTrash, locales.CommonDelete()).Size(sizeBtnSm, sizeBtnSm).Clicked()
		case s.isOpen(cfg.ID):
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(p.success)
				ui.Text(c, locales.ConnOpened()).FontSize(font2XS - 1).TextColor(p.fgSecondary)
			})
		}
	})
	switch {
	case del:
		defer s.deleteConnection(cfg)
	case edit:
		defer s.editConnection(cfg)
	case row.Clicked():
		d.open = false
		s.openConnection(cfg)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.ConnOpen()).Chosen() {
			defer func() { d.open = false; s.openConnection(cfg) }()
		}
		if m.Item(locales.CommonEdit()).Chosen() {
			defer s.editConnection(cfg)
		}
		if m.Item(locales.ConnDuplicate()).Chosen() {
			defer s.duplicateConnection(cfg)
		}
		s.moveToMenu(m, cfg)
		if m.Item(locales.ConnCopyURL()).Chosen() {
			c.Services().WriteClipboard(connURL(cfg))
			defer s.toastOK(locales.ConnCopiedURL())
		}
		m.Separator()
		if m.Item(locales.CommonDelete()).Chosen() {
			defer s.deleteConnection(cfg)
		}
	})
}

// connURL is a connection's address, as mysql://user@host:3306/db; the
// password is never part of it.
func connURL(cfg model.ConnectionConfig) string {
	scheme := app.URLSchemeOf(cfg.Type)
	if !app.CapsOf(cfg.Type).Network {
		return scheme + "://" + cfg.Database
	}
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), Path: "/" + cfg.Database}
	if cfg.User != "" {
		u.User = url.User(cfg.User)
	}
	return u.String()
}
