package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接 / 库浮层（设计稿 A 的 connpop）：点侧栏连接卡或库选择框，从连接卡下面弹出一张 340px 的浮层，
// 上面是搜索框，然后依次是「已打开」的连接（两行：名称 + 地址，带环境标记和当前勾）、
// 「xx 的数据库」（当前库打勾加粗，右端是表数）、新建连接、全部连接、断开当前连接。
// 键盘：↑↓ 在连接和库之间移动高亮，回车选中，Esc 或点外面收起。

// connPopItems 按搜索词过滤出连接和库，顺序就是屏幕上的顺序，高亮下标 sel 指向它。
func (s *Shell) connPopItems(w *Workspace, query string) (conns []int, dbs []model.DatabaseInfo) {
	q := strings.ToLower(strings.TrimSpace(query))
	for i, o := range s.spaces {
		if q == "" || strings.Contains(strings.ToLower(o.cfg.Name+" "+o.db), q) {
			conns = append(conns, i)
		}
	}
	if w != nil {
		for _, db := range w.dbs {
			if q == "" || strings.Contains(strings.ToLower(db.Name), q) {
				dbs = append(dbs, db)
			}
		}
	}
	return
}

// connHost 是连接行第二行的地址：网络库是主机，本地文件库是文件名。
func connHost(w *Workspace) string {
	if !app.CapsOf(w.cfg.Type).Network {
		return filepath.Base(w.cfg.Database)
	}
	return w.cfg.Host
}

// connPopover 画浮层。必须在连接卡画出来的同一帧里、连接卡之后调用（锚点 s.connAnchor）。
func (s *Shell) connPopover(c *ui.Context) {
	d := &s.wsPop
	p := s.tokens(c)
	cur := s.activeWorkspace()
	conns, dbs := s.connPopItems(cur, d.query)
	total := len(conns) + len(dbs)
	d.sel = min(max(d.sel, 0), max(total-1, 0))
	choose := func(i int) {
		switch {
		case i < len(conns):
			s.active = conns[i]
		case i-len(conns) < len(dbs) && cur != nil:
			cur.switchDatabase(dbs[i-len(conns)].Name)
		default:
			return
		}
		d.open = false
	}
	move := func(n int) { d.sel = min(max(d.sel+n, 0), max(total-1, 0)) }
	anchor := d.anchor
	ui.PopoverBase(c, anchor, &d.open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft).Margin(spaceXS, 0, 0, 0).Width(sizePopover).MaxHeight(520).
			Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		ui.Column(c).FillWidth().Padding(spaceSM - 2).Gap(1).Children(func() {
			s.connPopSearch(c, p, d, move, func() { choose(d.sel) })
			if len(conns) > 0 {
				popupHeading(c, p, locales.WsOpened())
			}
			for k, i := range conns {
				s.connPopConn(c, p, i, k == d.sel, func() { d.sel = k; choose(k) })
			}
			if cur != nil && len(dbs) > 0 {
				popupHeading(c, p, locales.DbSwitchOf(cur.cfg.Name))
			}
			for k, db := range dbs {
				idx := len(conns) + k
				s.connPopDB(c, p, cur, db, idx == d.sel, func() { d.sel = idx; choose(idx) })
			}
			if total == 0 {
				ui.Text(c, locales.PaletteNone()).TextColor(p.fgMuted).Padding(spaceLG).TextAlign(ui.Center).FillWidth()
			}
			hairline(c, p.borderSubtle).Margin(spaceXS, spaceSM)
			s.connPopActions(c, p, cur)
		})
	})
}

// connPopSearch 是浮层顶部的搜索框（26px，带边框）；↑↓ 移动高亮，回车选中。
func (s *Shell) connPopSearch(c *ui.Context, p tokens, d *wsPopup, move func(int), submit func()) {
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceSM+2).Height(sizeInput).FillWidth().Radius(radiusBtn).Border(1, p.border).Background(p.surface).Margin(2, 2, spaceSM-2, 2).Children(func() {
		ui.Icon(c, icSearch).FontSize(btnActIcon).TextColor(p.fgMuted)
		in := ui.TextInput(c.Key("connpop-q"), &d.query).Placeholder(locales.WsSearch()).FontSize(fontCtl).Grow(1).AutoFocus().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
			OnTextKey(func(k ui.TextKey) bool {
				if k.Mods != 0 {
					return false
				}
				switch k.Key {
				case ui.KeyDown:
					move(1)
				case ui.KeyUp:
					move(-1)
				default:
					return false
				}
				s.redraw()
				return true
			})
		if in.Changed() {
			d.sel = 0
		}
		if in.Submitted() {
			submit()
		}
	})
}

// connPopRow 画一行浮层列表项的底：高亮态用强调淡底，悬浮用 hover 底。
func connPopRow(c *ui.Context, p tokens, key string, h float32, on bool) ui.Element {
	row := ui.Row(c.Key(key)).Gap(spaceSM+2).Padding(0, spaceSM+2).Height(h).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
	switch {
	case on:
		row.Background(p.accentSoft2)
	case row.Hovered():
		row.Background(p.hover)
	}
	return row
}

// connPopConn 画一个已打开的连接：头像、名称 + 地址两行、环境标记，当前连接右端打勾。
func (s *Shell) connPopConn(c *ui.Context, p tokens, i int, on bool, pick func()) {
	w := s.spaces[i]
	row := connPopRow(c, p, w.id(), sizeBtnLg+2, on)
	row.Children(func() {
		dbTile(c, w.cfg.Type, sizeAvatar, connColor(c, cfgColor(w.cfg)))
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(600).SingleLine()
			ui.Text(c, connHost(w)).FontSize(fontCaption - 0.5).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		if w.cfg.Tag != "" && w.cfg.Tag != "local" {
			sqChip(c, tagChipKind(w.cfg.Tag), tagShort(w.cfg.Tag))
		}
		if i == s.active {
			ui.Icon(c, icCheck).FontSize(btnActIcon).TextColor(p.accent)
		}
	})
	if row.Clicked() {
		pick()
	}
	row.ContextMenu(func(m *ui.Menu) { // 原来的右键菜单照旧
		if m.Item(locales.CommonEdit()).Chosen() {
			defer s.editConnection(w.cfg)
		}
		if m.Item(locales.ToolbarDisconnect()).Chosen() {
			defer s.closeSpaceGuarded(w)
		}
		if len(s.spaces) > 1 && m.Item(locales.WsCloseOthers()).Chosen() {
			defer s.closeOtherSpaces(w)
		}
	})
}

// connPopDB 画一个库：当前库前面打勾、名字加粗，右端是表数；有建库能力的连接可以右键改名、删除。
func (s *Shell) connPopDB(c *ui.Context, p tokens, w *Workspace, db model.DatabaseInfo, on bool, pick func()) {
	row := connPopRow(c, p, "db-"+db.Name, sizeRow, on)
	isCur := w != nil && (db.Name == w.db || !app.CapsOf(w.cfg.Type).MultiDatabase) // 文件型库只有这一个库，就是当前库
	row.Children(func() {
		if isCur {
			ui.Icon(c, icCheck).FontSize(fontDense).TextColor(p.fgSecondary).Shrink(0)
		} else {
			ui.Box(c).Size(fontDense, fontDense).Shrink(0)
		}
		name := ui.Text(c, db.Name).FontSize(fontCtl).SingleLine().Grow(1).MinWidth(0)
		if isCur {
			name.FontWeight(600)
		}
		if db.TableCount > 0 {
			ui.Text(c, locales.DbSwitchTables(db.TableCount)).FontSize(fontCaption).TextColor(p.fgMuted).Shrink(0)
		}
	})
	if row.Clicked() {
		pick()
	}
	if w != nil && app.CapsOf(w.cfg.Type).CreateDatabase && !w.cfg.ReadOnly {
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item(locales.DbRename()).Chosen() {
				defer func() { s.wsPop.open = false; s.renameDatabase(w, db.Name) }()
			}
			if m.Item(locales.DbDrop()).Chosen() {
				defer func() { s.wsPop.open = false; s.dropDatabase(w, db.Name) }()
			}
		})
	}
}

// connPopActions 是浮层底部的三个动作：新建连接（带 ⌘N 键帽）、全部连接（回到欢迎页）、断开当前连接（红字）。
func (s *Shell) connPopActions(c *ui.Context, p tokens, cur *Workspace) {
	act := func(key string, ic *ui.SVG, label string, col ui.Color, hint string) bool {
		row := connPopRow(c, p, key, sizeRow+2, false)
		row.Children(func() {
			ui.Icon(c, ic).FontSize(btnActIcon).TextColor(col)
			ui.Text(c, label).FontSize(fontCtl).TextColor(col).SingleLine().Grow(1)
			if hint != "" {
				kbd(c, hint)
			}
		})
		return row.Clicked()
	}
	if act("cp-new", icPlus, locales.WsNewConnection(), p.fg, "⌘N") {
		s.wsPop.open = false
		s.newConnectionDialog()
	}
	if act("cp-all", icPlug, locales.WsAllConnections(), p.fg, "") {
		s.wsPop.open = false
		s.active = -1 // 回到欢迎页的连接列表，已打开的连接都还在
	}
	if cur != nil && act("cp-close", icUnplug, locales.WsDisconnectNamed(cur.cfg.Name), p.danger, "") {
		s.wsPop.open = false
		s.closeSpaceGuarded(cur)
	}
}
