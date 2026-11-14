package ui

import (
	"fmt"
	"minidb/internal/database"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

func hairline(c *ui.Context, col ui.Color) ui.Element {
	return ui.Box(c).Height(1).Background(col).FillWidth()
}
func vline(c *ui.Context, col ui.Color, h float32) ui.Element {
	return ui.Box(c).Width(1).Height(h).Background(col).Margin(0, 2)
}

// tip returns a flat icon button of the toolbar (no face until hovered).
// Its icon is the secondary grey, darker under the pointer, as TablePlus's are.
func tipBtn(c *ui.Context, ic *ui.SVG, tip string) ui.Element {
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtn, sizeBtn).Padding(0).BorderWidth(0).Background(ui.Color{})
	p := baseTokens(c)
	col := p.fgSecondary
	if b.Hovered() {
		col = p.fg
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(15).TextColor(col) })
}

// toggleBtn is tipBtn for a switch: its icon takes the accent while it is on.
func toggleBtn(c *ui.Context, ic *ui.SVG, tip string, on bool) ui.Element {
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtnSm, sizeBtnSm).Padding(0).BorderWidth(0).Background(ui.Color{})
	p := baseTokens(c)
	col := p.fgSecondary
	switch {
	case on:
		col = p.accent
		b.Background(p.accent.Alpha(0.10))
	case b.Hovered():
		col = p.fg
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(14).TextColor(col) })
}

func (s *Shell) view(c *ui.Context) {
	p := s.applyLook(c)
	if s.probe != nil {
		s.probe(c)
	}
	s.flushToasts(c)
	if s.clip != "" {
		c.Services().WriteClipboard(s.clip)
		s.clip = ""
	}

	ui.Column(c).Fill().Background(p.surface).Children(func() {
		s.toolbar(c, p)
		hairline(c, p.border)
		body := func() {
			ui.Row(c).Fill().MinWidth(0).Children(func() {
				if len(s.spaces) > 1 { // 只有一个工作区（或没有）时不占地方；新建连接用工具栏的按钮或 ⌘N
					s.rail(c, p)
				}
				if w := s.activeWorkspace(); w != nil {
					w.view(c, p)
				} else {
					s.emptyWorkspace(c, p)
				}
			})
		}
		if !s.consoleOpen {
			ui.Column(c.Key("body")).Grow(1).FillWidth().MinWidth(0).Children(body)
			return
		}
		if s.consolePct <= 0 {
			s.consolePct = 74
		}
		// 分隔条可以拖：日志的高度随窗口按比例伸缩
		ui.SplitVerticalPercent(c, &s.consolePct, body, func() { s.consolePanel(c, p) }).Grow(1).FillWidth()
	})
	s.shortcuts(c)
	s.workspacesPopup(c)
	s.connectionDialog(c)
	s.exportDialogView(c)
	s.backupView(c)
	s.usersView(c)
	s.restoreView(c)
	s.importDialogView(c)
	s.settingsView(c)
	s.processView(c)
	s.switcherView(c)
	s.paletteView(c)
	s.promptView(c)
	s.editorOptionsDialog(c)
	s.searchView(c)
	s.confirmView(c)
	s.passwordView(c)
	s.toastViewport(c, p) // 最后建，盖在对话框之上
}

// rail is the 60px workspace bar, shown only with more than one open workspace.
func (s *Shell) rail(c *ui.Context, p tokens) {
	ui.Row(c).FillHeight().Shrink(0).Children(func() {
		ui.Column(c).Width(60).FillHeight().Padding(12, 0).Gap(8).AlignItems(ui.Center).Background(p.toolbar).Shrink(0).Children(func() {
			for i, w := range s.spaces {
				i, w := i, w
				active := i == s.active
				name := w.db
				if name == "" || !database.CapsOf(w.cfg.Type).MultiDatabase {
					name = w.cfg.Name
				}
				nth := w.ordinal() // 同一个连接、同一个库开了好几个时，用图标角上的序号区分
				col := connColor(c, w.cfg.Color)
				item := ui.Column(c.Key(w.id())).Width(48).Height(52).Radius(radiusPanel).AlignItems(ui.Center).Justify(ui.Center).Gap(4)
				hovered := item.Hovered()
				switch {
				case active:
					item.Background(p.accent.Alpha(0.12))
				case hovered:
					item.Background(p.hover)
				}
				closed := false
				tipOn(c, item, strings.TrimSpace(w.cfg.Name+" - "+w.db+map[bool]string{true: fmt.Sprintf(" #%d", nth)}[nth > 1])).Children(func() {
					ui.Icon(c, icDatabase).FontSize(20).TextColor(col)
					if nth > 1 {
						ui.Row(c.Key("nth")).Absolute().Top(6).Right(8).Size(14, 14).Radius(7).Background(col).Center().Children(func() {
							ui.Text(c, fmt.Sprint(nth)).FontSize(9).FontWeight(700).TextColor(ui.Hex("#ffffff"))
						})
					}
					ui.Text(c, name).FontSize(10).SingleLine().MaxWidth(44).TextAlign(ui.Center).TextColor(p.fgSecondary)
					if active { // the marker sits on the rail's left edge
						ui.Box(c).Absolute().Left(-6).Top(14).Size(3, 24).Radius(2).Background(col)
					}
					if hovered || active {
						x := iconButton(c.Key("close"), icX, locales.ToolbarDisconnect()).Absolute().Top(2).Right(2).Size(16, 16)
						if !hovered {
							x.Opacity(0)
						}
						if x.Clicked() {
							closed = true
						}
					}
				})
				switch {
				case closed:
					defer s.closeSpaceGuarded(w)
				case item.Clicked():
					s.active = i
				}
				item.ContextMenu(func(m *ui.Menu) {
					if m.Item(locales.CommonEdit()).Chosen() {
						defer s.editConnection(w.cfg)
					}
					if m.Item(locales.ToolbarDisconnect()).Chosen() {
						defer s.closeSpaceGuarded(w)
					}
					if m.Item(locales.WsCloseOthers()).Chosen() {
						defer s.closeOtherSpaces(w)
					}
				})
				if i < len(s.spaces)-1 {
					ui.Box(c).Width(32).Height(1).Background(p.border.Alpha(0.4))
				}
			}
			if tipBtn(c.Key("rail-new"), icPlus, locales.ToolbarNewConnection()+s.keyHint("new-conn")).Size(36, 36).Radius(radiusPanel).Clicked() {
				s.newConnectionDialog()
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(p.border)
	})
}

// emptyWorkspace is what shows without a connection.
func (s *Shell) emptyWorkspace(c *ui.Context, p tokens) {
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(260).FillHeight().Background(p.toolbar).Center().Gap(6).Padding(16).Children(func() {
			logo(c, 56)
			ui.Text(c, locales.SidebarNotConnected()).FontSize(font2XS + 1).FontWeight(500).TextColor(p.fgSecondary)
			ui.Text(c, locales.SidebarConnectHint()).FontSize(font2XS).TextColor(p.fgMuted).TextAlign(ui.Center)
		})
		ui.Box(c).Width(1).FillHeight().Background(p.border)
		ui.Column(c).Grow(1).FillHeight()
	})
}

// ---- workspace ----

func (w *Workspace) main(c *ui.Context, p tokens) {
	ui.Column(c).Fill().MinWidth(0).Background(p.surface).Children(func() {
		w.tabBar(c, p)
		if len(w.tabs) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				ui.Icon(c, icTable).FontSize(30).TextColor(p.fgMuted)
				ui.Text(c, locales.TabNone()).FontSize(font2XS + 1).TextColor(p.fgMuted)
				if actPrimary(c.Key("empty-new-query"), icPlus, locales.MenuNewQuery()+w.sh.keyHint("new-query")).Clicked() {
					w.newQuery("")
				}
			})
			return
		}
		if w.active >= len(w.tabs) {
			w.active = len(w.tabs) - 1
		}
		ui.Column(c.Key(fmt.Sprintf("tab%d", w.active))).Grow(1).FillWidth().MinWidth(0).Children(func() { w.tabs[w.active].view(c, p) })
	})
}
