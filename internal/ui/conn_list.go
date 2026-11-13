package ui

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接管理器的列表页：已保存的连接，列表下方是"再添加新的连接"。
// 点一行直接连接；悬停出现 ✎ 编辑和 🗑 删除；↑↓ 移动光标，↵ 连接；右键还有复制等操作。

// connList 画列表页。参数 c 为界面上下文，p 为主题色板；无返回值。
func (s *Shell) connList(c *ui.Context, p tokens) {
	d := &s.dlg
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(16, 12, 12, 22).FillWidth().Shrink(0).Children(func() {
		logo(c, 34)
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, locales.WelcomeTitle()).FontWeight(700).FontSize(16)
			sub := locales.ConnEmpty()
			if n := len(s.conns); n > 0 {
				sub = locales.ConnListSub(n)
			}
			ui.Text(c, sub).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
		})
		ui.MenuButton(c, locales.ConnMore(), func(m *ui.Menu) {
			if m.Item(locales.ConnRestore()).Chosen() {
				s.importConnections("")
			}
			if m.Item(locales.ConnBackup()).Chosen() {
				s.exportConnections("")
			}
		}).Height(sizeBtnSm).FontSize(font2XS)
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			d.open = false
		}
	})
	shown := s.filteredConns()
	cur := s.listCursor(shown)
	if len(s.conns) > 0 {
		s.connSearch(c, p, shown, cur)
	}
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
		scrollbars(c, ui.Scroll(c.Key("conns")).FillWidth().Shrink(1).MinHeight(0)).Children(func() {
			ui.Column(c).Gap(2).FillWidth().Padding(4, 14, 4, 14).Children(func() {
				if len(shown) == 0 {
					s.connEmptyState(c, p)
				}
				for i := range shown {
					s.connRow(c, p, shown[i], i == cur)
				}
			})
		})
		s.connAddRow(c, p)
	})
	hairline(c, p.borderSubtle)
	ui.Column(c).FillWidth().Padding(9, 22).Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Text(c, locales.ConnListHint()).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
	})
}

// filteredConns 是按搜索词筛过的连接（名称、主机、库名、环境）。
func (s *Shell) filteredConns() []database.ConnectionConfig {
	q := strings.ToLower(strings.TrimSpace(s.dlg.search))
	var shown []database.ConnectionConfig
	for _, cfg := range s.conns {
		if q == "" || strings.Contains(strings.ToLower(cfg.Name+" "+cfg.Host+" "+cfg.Database+" "+cfg.Tag), q) {
			shown = append(shown, cfg)
		}
	}
	return shown
}

// listCursor 是光标在 shown 里的下标：停在 s.selected 上，选中的连接不在列表里就落在第一条。
func (s *Shell) listCursor(shown []database.ConnectionConfig) int {
	for i, cfg := range shown {
		if cfg.ID == s.selected {
			return i
		}
	}
	return 0
}

// connSearch 是列表上方的搜索框：↑↓ 移动光标（搜索框有焦点时也一样），↵ 连接光标所在的那条。
func (s *Shell) connSearch(c *ui.Context, p tokens, shown []database.ConnectionConfig, cur int) {
	d := &s.dlg
	move := func(delta int) {
		if len(shown) > 0 {
			s.selected = shown[min(max(cur+delta, 0), len(shown)-1)].ID
		}
	}
	var search ui.Element
	ui.Column(c).FillWidth().Padding(0, 22, 8, 22).Shrink(0).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 10).Height(sizeBtn+2).Radius(radiusBtn).Background(p.surfaceSecondary).Border(1, p.borderSubtle).FillWidth().Children(func() {
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
	ui.Column(c).FillWidth().Padding(36, 20, 28, 20).Gap(6).AlignItems(ui.Center).Children(func() {
		if len(s.conns) > 0 {
			ui.Text(c, locales.ConnNoMatch()).FontSize(font2XS).TextColor(p.fgMuted)
			return
		}
		ui.Icon(c, icDatabase).FontSize(30).TextColor(p.fgMuted)
		ui.Text(c, locales.ConnEmpty()).FontSize(14).FontWeight(600)
		ui.Text(c, locales.ConnEmptyHint()).FontSize(font2XS).TextColor(p.fgMuted)
	})
}

// connAddRow 是列表下方的"再添加新的连接"：点一下进入配置页。
func (s *Shell) connAddRow(c *ui.Context, p tokens) {
	label := locales.ConnAddAnother()
	if len(s.conns) == 0 {
		label = locales.ConnAddFirst()
	}
	var row ui.Element
	ui.Column(c).FillWidth().Padding(4, 14, 12, 14).Shrink(0).Children(func() {
		row = ui.Row(c.Key("conn-add")).Gap(12).Padding(8, 12).Radius(radiusPanel).Border(1, p.accent.Alpha(0.35)).AlignItems(ui.Center).FillWidth()
		if row.Hovered() {
			row.Background(p.accent.Alpha(0.08)).BorderColor(p.accent.Alpha(0.6))
		}
		row.Children(func() {
			ui.Row(c).Size(36, 36).Shrink(0).Center().Radius(9).Background(p.accent.Alpha(0.12)).Children(func() {
				ui.Icon(c, icPlus).FontSize(17).TextColor(p.accent)
			})
			ui.Text(c, label).FontSize(font2XS + 1).FontWeight(600).TextColor(p.accent).Grow(1)
			ui.Icon(c, icChevronR).FontSize(14).TextColor(p.accent.Alpha(0.7))
		})
	})
	if row.Clicked() {
		s.startDraft("mysql")
	}
}

// connRow 是列表里的一条连接。cursor 表示光标停在这一行。
func (s *Shell) connRow(c *ui.Context, p tokens, cfg database.ConnectionConfig, cursor bool) {
	d := &s.dlg
	row := ui.Row(c.Key(cfg.ID)).Gap(12).Padding(8, 10, 8, 12).Radius(radiusPanel).AlignItems(ui.Center).FillWidth()
	hovered := row.Hovered()
	switch {
	case cursor:
		row.Background(p.accent.Alpha(0.12))
	case hovered:
		row.Background(p.hover)
	}
	var del, edit bool
	row.Children(func() {
		dbTile(c, cfg.Type, 36, connColor(c, cfg.Color))
		ui.Column(c).Grow(1).Gap(2).MinWidth(0).Children(func() {
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
func connURL(cfg database.ConnectionConfig) string {
	scheme := cfg.Type
	if scheme == "postgres" {
		scheme = "postgresql"
	}
	if cfg.Type == "sqlite" {
		return "sqlite://" + cfg.Database
	}
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), Path: "/" + cfg.Database}
	if cfg.User != "" {
		u.User = url.User(cfg.User)
	}
	return u.String()
}
