package ui

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 已打开的连接浮层（设计稿 connpop）：点标题栏左边的连接胶囊，从胶囊下面弹出一张 360px 的浮层：
// 标题「已打开的连接 · N」，每个连接一行（头像、名称 + 环境标记、类型版本 · 地址、绿色状态点、× 断开），
// 底部是「打开连接…」「新建连接… ⌘N」。键盘：↑↓ 移动高亮，回车切到该连接，Esc 或点外面收起。
// 库的切换是另一张浮层（db_popover.go），挂在侧栏顶部的库名上。

// connHost 是连接行第二行的地址：网络库是主机，本地文件库是文件名。
func connHost(w *Workspace) string {
	if !app.CapsOf(w.cfg.Type).Network {
		return filepath.Base(w.cfg.Database)
	}
	return w.cfg.Host
}

// connPopover 画浮层。必须在连接胶囊画出来的同一帧里、胶囊之后调用（锚点 s.wsPop.anchor）。
func (s *Shell) connPopover(c *ui.Context) {
	d := &s.wsPop
	p := s.tokens(c)
	n := len(s.spaces)
	d.sel = min(max(d.sel, 0), max(n-1, 0))
	choose := func(i int) {
		if i >= 0 && i < n {
			s.active = i
			d.open = false
		}
	}
	switch {
	case c.Shortcut(0, ui.KeyDown):
		d.sel = min(d.sel+1, n-1)
	case c.Shortcut(0, ui.KeyUp):
		d.sel = max(d.sel-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		choose(d.sel)
	}
	anchor := c.Resolve(d.anchor)
	var closing *Workspace // 点了 × 的连接：等这一帧画完再断开，免得遍历到一半 s.spaces 变短
	ui.PopoverBase(c, anchor, &d.open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft).Margin(spaceXS, 0, 0, 0).Width(sizeConnPop).MaxHeight(520).
			Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		enterPose(c, panel, posePop, ui.Snappy)
		ui.Column(c).FillWidth().Padding(spaceSM - 2).Gap(1).Children(func() {
			popupHeading(c, p, locales.WsOpenedN(n))
			for i := range s.spaces {
				i := i
				if s.connPopConn(c, p, i, i == d.sel, func() { d.sel = i; choose(i) }) {
					closing = s.spaces[i]
				}
			}
			hairline(c, p.borderSubtle).Margin(spaceXS, spaceSM)
			s.connPopActions(c, p)
		})
	})
	if closing != nil {
		d.open = false
		s.closeSpaceGuarded(closing)
	}
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

// connPopConn 画一个已打开的连接：头像、名称 + 环境标记、类型版本 · 地址，右端是状态点和断开的 ×。
// 当前连接整行淡强调底；断开的连接状态点是红色。点了 × 返回 true（由调用方断开）。
func (s *Shell) connPopConn(c *ui.Context, p tokens, i int, on bool, pick func()) (closed bool) {
	w := s.spaces[i]
	row := connPopRow(c, p, w.id(), sizeConnRow, on || s.active == i)
	row.Children(func() {
		dbTile(c, w.cfg.Type, sizeAvatarLg, connColor(c, cfgColor(w.cfg)))
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(spaceSM - 2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(600).SingleLine().Shrink(1)
				if w.cfg.Tag != "" && w.cfg.Tag != "local" {
					sqChip(c, tagChipKind(w.cfg.Tag), tagShort(w.cfg.Tag))
				}
			})
			ui.Text(c, cardSubtitle(w)).FontSize(fontCaption - 0.5).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		dot := p.success
		if s.lost[w.cfg.ID] {
			dot = p.danger
		}
		ui.Box(c.Key("st"+w.id())).Size(sizeDot+2, sizeDot+2).Shrink(0).Radius((sizeDot + 2) / 2).Background(dot)
		if tipBtn(c.Key("x"+w.id()), icX, locales.WsDisconnectNamed(w.cfg.Name)).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			closed = true
		}
	})
	if closed {
		return true
	}
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
	return false
}

// connPopActions 是浮层底部的两个动作：打开连接…（回到欢迎页的连接列表，已打开的都还在）、新建连接…（带 ⌘N 键帽）。
func (s *Shell) connPopActions(c *ui.Context, p tokens) {
	act := func(key string, ic *ui.SVG, label, hint string) bool {
		row := connPopRow(c, p, key, sizeRow+2, false)
		row.Children(func() {
			ui.Icon(c, ic).FontSize(btnFieldIcon).TextColor(p.fgSecondary)
			ui.Text(c, label).FontSize(fontCtl).SingleLine().Grow(1)
			if hint != "" {
				kbd(c, hint)
			}
		})
		return row.Clicked()
	}
	if act("cp-open", icPlug, locales.WsOpenConn(), "") {
		s.wsPop.open = false
		s.active = -1
	}
	if act("cp-new", icPlus, locales.WsNewConnection(), s.cmdKeys("new-conn")) {
		s.wsPop.open = false
		s.newConnectionDialog()
	}
}
