package ui

import (
	"fmt"
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
	b := ui.Button(c.Key(tip), "").Label(tip).Tooltip(tip).Size(sizeBtn, sizeBtn).Padding(0).BorderWidth(0).Background(ui.Color{})
	p := baseTokens(c)
	col := p.fgSecondary
	if b.Hovered() {
		col = p.fg
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(15).TextColor(col) })
}

// toggleBtn is tipBtn for a switch: its icon takes the accent while it is on.
func toggleBtn(c *ui.Context, ic *ui.SVG, tip string, on bool) ui.Element {
	b := ui.Button(c.Key(tip), "").Label(tip).Tooltip(tip).Size(sizeBtnSm, sizeBtnSm).Padding(0).BorderWidth(0).Background(ui.Color{})
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
	for _, m := range s.toasts {
		c.Toast(m)
	}
	s.toasts = s.toasts[:0]
	if s.clip != "" {
		c.Services().WriteClipboard(s.clip)
		s.clip = ""
	}
	for _, a := range s.toastActs {
		c.ToastAction(a.msg, a.label, a.fn)
	}
	s.toastActs = s.toastActs[:0]

	ui.Column(c).Fill().Background(p.surface).Children(func() {
		s.toolbar(c, p)
		hairline(c, p.border)
		ui.Row(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
			if len(s.spaces) > 1 {
				s.rail(c, p)
			}
			if w := s.activeWorkspace(); w != nil {
				w.view(c, p)
			} else {
				s.emptyWorkspace(c, p)
			}
		})
		s.consolePanel(c, p)
	})
	s.shortcuts(c)
	s.notesDialog(c)
	s.workspacesPopup(c)
	s.connectionDialog(c)
	s.exportDialogView(c)
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
}

// toolbar is the 36px title bar: connection actions on the left, the current
// connection as a pill in the middle, global actions on the right.
func (s *Shell) toolbar(c *ui.Context, p tokens) {
	bar := c.TitleBar()
	w := s.activeWorkspace()
	left := max(bar.Left, 12) + 4
	ui.Box(c).Height(sizeToolbar).FillWidth().Background(p.toolbar).DragWindow().Children(func() {
		// centered pill, behind the buttons: passes clicks through
		ui.Row(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Justify(ui.Center).AlignItems(ui.Center).PassThrough().Children(func() {
			if w != nil {
				s.pill(c, p, w)
			}
		})
		ui.Toolbar(c, func() {
			ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(0, 8, 0, left).FillWidth().Height(sizeToolbar).Children(func() {
				if w != nil {
					if toggleBtn(c, icSidebar, locales.ToolbarSidebar()+" (⌘0)", !w.sidebarHidden).Clicked() {
						w.sidebarHidden = !w.sidebarHidden
					}
				}
				if tipBtn(c, icPlug, locales.ToolbarNewConnection()+" (⌘N)").Clicked() {
					s.newConnectionDialog()
				}
				if tipBtn(c, icUnplug, locales.ToolbarDisconnect()).Disabled(w == nil).Clicked() && w != nil {
					s.closeConnection(w.cfg.ID)
				}
				vline(c, p.border, 12)
				if w != nil {
					if tipBtn(c, icDatabase, locales.ToolbarSwitchDatabase()+" (⌘K)").Clicked() {
						s.openSwitcher()
					}
					s.safeButton(c, p, w)
					sql := ui.Button(c, "SQL").Tooltip(locales.ToolbarSQL() + " (⌘T)").Font(mono).FontSize(font2XS).Height(sizeBtn)
					if sql.Clicked() {
						s.cmdNewQuery()
					}
				}
				ui.Spacer(c)
				s.taskBar(c)
				s.pendingGroup(c, p)
				if toggleBtn(c, icConsole, locales.ToolbarConsole()+" (⌃⌘C)", s.consoleOpen).Clicked() {
					s.consoleOpen = !s.consoleOpen
				}
				s.bellButton(c, p)
				if tipBtn(c, icSearch, locales.ToolbarSearch()+" (⌘P)").Clicked() {
					s.cmdPalette()
				}
				if tipBtn(c, icSparkle, locales.ToolbarAI()+" (⌘I)").Disabled(w == nil).Clicked() {
					s.cmdToggleAI()
				}
				themeTip, themeIcon := locales.ToolbarToDark(), icMoon
				if c.Theme().Dark {
					themeTip, themeIcon = locales.ToolbarToLight(), icSun
				}
				if tipBtn(c, themeIcon, themeTip).Clicked() {
					s.toggleTheme(c.Theme().Dark)
				}
				if tipBtn(c, icSettings, locales.ToolbarSettings()+" (⌘,)").Clicked() {
					s.openSettings(0)
				}
			})
		}).Padding(0)
	})
}

// pill shows the current connection: status, name, database and server.
func (s *Shell) pill(c *ui.Context, p tokens, w *Workspace) {
	pill := ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 12).Height(sizeBtn).Radius(999).Background(p.surfaceSecondary).Border(1, p.border)
	if pill.Hovered() {
		pill.Background(p.hover)
	}
	reconnect := false
	pill.Tooltip(s.pillDetails(w)).Children(func() {
		dot := p.success
		switch {
		case s.lost[w.cfg.ID]:
			dot = p.danger
		case len(s.connecting) > 0:
			dot = p.warning
		}
		if w.busy() { // a query or a page of a table is on its way
			ui.Spinner(c).Size(10, 10)
		} else {
			ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		}
		ui.Text(c, w.cfg.Name).FontSize(font2XS).FontWeight(500).SingleLine().MaxWidth(160)
		if tag := w.cfg.Tag; tag != "" && tag != "local" { // the environment, in its colour
			col := tagColor(c, tag)
			ui.Text(c, tagLabel(tag)).FontSize(font2XS-1).FontWeight(600).TextColor(col).Padding(0, 5).Background(col.Alpha(0.15)).Radius(4)
		}
		if w.cfg.ReadOnly {
			ui.Text(c, locales.StatusReadOnly()).FontSize(font2XS).FontWeight(500).TextColor(p.warning).Padding(0, 4).Background(p.warning.Alpha(0.15)).Radius(4)
		}
		if w.db != "" && w.cfg.Type != "sqlite" {
			ui.Text(c, "/").FontSize(font2XS).TextColor(p.fgMuted)
			ui.Text(c, w.db).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine().MaxWidth(140)
		}
		ver := typeLabel(w.cfg.Type)
		if w.version != "" {
			if strings.HasPrefix(strings.ToLower(w.version), strings.ToLower(ver)) {
				ver = w.version
			} else {
				ver += " " + w.version
			}
		}
		ui.Text(c, "·").FontSize(font2XS).TextColor(p.fgMuted)
		ui.Text(c, ver).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().MaxWidth(200)
		if ui.Button(c, "").Label(locales.ToolbarReconnect()).Tooltip(locales.ToolbarReconnect()).Size(16, 16).Radius(8).Children(func() {
			ui.Icon(c, icRefresh).FontSize(10).TextColor(p.fgSecondary)
		}).Clicked() {
			reconnect = true
		}
	})
	switch {
	case reconnect:
		s.reconnect(w)
	case pill.Clicked(): // the pill opens the list of connections, as TablePlus's title bar does
		s.newConnectionDialog()
	}
}

// busy reports whether a query or a table page is loading in w.
func (w *Workspace) busy() bool {
	for _, tb := range w.tabs {
		switch t := tb.(type) {
		case *queryTab:
			if t.running {
				return true
			}
		case *tableTab:
			if t.loading {
				return true
			}
		}
	}
	return false
}

// pillDetails is the tooltip of the status pill: what the connection is.
func (s *Shell) pillDetails(w *Workspace) string {
	lines := []string{w.cfg.Name, connSubtitle(w.cfg)}
	if w.version != "" {
		lines = append(lines, w.version)
	}
	lines = append(lines, locales.SafeTitle()+": "+safeModeLabel(w.cfg.EffectiveSafeMode()))
	if s.lost[w.cfg.ID] {
		lines = append(lines, locales.ConnLost(w.cfg.Name))
	}
	lines = append(lines, locales.WelcomeTitle()+" (⌘O)")
	return strings.Join(lines, "\n")
}

func (s *Shell) reconnect(w *Workspace) {
	s.up.bg(func() {
		_ = s.app.Connections.Disconnect(w.cfg.ID)
		_, _ = s.app.Connections.Connect(w.cfg.ID)
	}, func() { w.loadDatabases() })
}

func (s *Shell) toggleTheme(dark bool) {
	name := "dark"
	if dark {
		name = "light"
	}
	applyTheme(name)
	s.prefs.Theme = name
	s.savePrefs()
}

// rail is the 60px workspace bar, shown with more than one open connection.
func (s *Shell) rail(c *ui.Context, p tokens) {
	ui.Row(c).FillHeight().Shrink(0).Children(func() {
		ui.Column(c).Width(60).FillHeight().Padding(12, 0).Gap(8).AlignItems(ui.Center).Background(p.toolbar).Shrink(0).Children(func() {
			for i, w := range s.spaces {
				i, w := i, w
				active := i == s.active
				name := w.db
				if name == "" || w.cfg.Type == "sqlite" {
					name = w.cfg.Name
				}
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
				item.Tooltip(w.cfg.Name + " - " + w.db).Children(func() {
					ui.Icon(c, icDatabase).FontSize(20).TextColor(col)
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
		})
		ui.Box(c).Width(1).FillHeight().Background(p.border)
	})
}

// emptyWorkspace is what shows without a connection.
func (s *Shell) emptyWorkspace(c *ui.Context, p tokens) {
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(260).FillHeight().Background(p.toolbar).Center().Gap(6).Padding(16).Children(func() {
			ui.Icon(c, icDatabase).FontSize(24).TextColor(p.fgMuted)
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
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icTable).FontSize(30).TextColor(p.fgMuted)
				ui.Text(c, locales.TabNone()).FontSize(font2XS + 1).TextColor(p.fgMuted)
			})
			return
		}
		if w.active >= len(w.tabs) {
			w.active = len(w.tabs) - 1
		}
		ui.Column(c.Key(fmt.Sprintf("tab%d", w.active))).Grow(1).FillWidth().MinWidth(0).Children(func() { w.tabs[w.active].view(c, p) })
	})
}

// tabBar is the strip of tabs, in the editor-tab style of VS Code: the active
// tab on the surface color under a line in the connection's color, the others
// recessed and divided by hairlines. A tab's close button shows only while the
// pointer is over it (or it is the active one, dimmed); a dot marks unsaved
// changes in its place. A middle click closes, and the strip scrolls sideways
// when the tabs outgrow it.
func (w *Workspace) tabBar(c *ui.Context, p tokens) {
	if len(w.tabs) == 0 {
		return
	}
	accent := connColor(c, w.cfg.Color)
	middleDown := false
	ui.ScrollHorizontal(c.Key("tabstrip")).FillWidth().Shrink(0).Scrollbars(false).Background(p.toolbar).Children(func() {
		ui.Row(c).Height(sizeTab).AlignItems(ui.Stretch).Children(func() {
			for i, tb := range w.tabs {
				i, tb := i, tb
				active := i == w.active
				label := tb.title()
				tab := ui.Row(c.Key(fmt.Sprintf("tab-%d-%s", i, label))).Gap(6).AlignItems(ui.Center).Padding(0, 6, 0, 12).Height(sizeTab).MinWidth(96).MaxWidth(200).Shrink(0)
				hovered := tab.Hovered()
				switch {
				case active:
					tab.Background(p.surface).BorderWidth(2, 0, 0, 0).BorderColor(accent)
				case hovered:
					tab.Background(p.surface.Alpha(0.6)).BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
				default:
					tab.BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
				}
				closeClicked := false
				tab.Children(func() {
					fg := p.fgSecondary
					if active {
						fg = p.fg
					}
					ui.Icon(c, tb.icon()).FontSize(11).TextColor(tabIconColor(p, tb, active, fg))
					lbl := ui.Text(c, label).FontSize(font2XS).SingleLine().TextColor(fg).Grow(1).TruncateMiddle()
					if isPreviewTab(tb) {
						lbl.Italic()
					}
					// the close slot keeps its width, so tabs never change size
					ui.Box(c.Key("slot")).Size(18, 18).Shrink(0).Radius(4).Children(func() {
						ui.Column(c).Fill().Center().Children(func() {
							switch {
							case hovered:
								x := iconButton(c.Key("x"), icX, locales.CommonClose()).Size(18, 18).Radius(4)
								if x.Hovered() {
									x.Background(p.hover)
								}
								if x.Clicked() {
									closeClicked = true
								}
							case tb.pending() > 0:
								ui.Box(c).Size(7, 7).Radius(4).Background(p.fgSecondary)
							}
						})
					})
				})
				if closeClicked {
					defer w.closeTab(i)
				} else if tab.DoubleClicked() {
					w.active = i
					if t, ok := tb.(*tableTab); ok {
						t.keep()
					}
				} else if tab.Clicked() {
					w.active = i
				}
				// A middle click closes the tab at once, on the press.
				if b, down := tab.PressedButton(); down && b == 2 {
					middleDown = true
					if key := fmt.Sprint(i, label); w.midKey != key {
						w.midKey = key
						defer func() { w.closeTab(i); w.sh.redraw() }()
					}
				}
				tab.ContextMenu(func(m *ui.Menu) {
					if t, ok := tb.(*tableTab); ok && t.preview && m.Item(locales.TabKeepOpen()).Chosen() {
						defer t.keep()
					}
					if m.Item(locales.CommonClose()).Chosen() {
						defer w.closeTab(i)
					}
					if m.Item(locales.TabCloseOthers()).Chosen() {
						defer w.closeOthers(i)
					}
				})
				ui.Box(c.Key(fmt.Sprint("sep", i))).Width(1).FillHeight().Background(p.border).Shrink(0)
			}
		})
	})
	if !middleDown {
		w.midKey = ""
	}
	hairline(c, p.border)
}

// tabIconColor colours a tab's icon by what the tab is; inactive tabs keep
// the kind's hue, a little recessed.
func tabIconColor(p tokens, tb tab, active bool, fallback ui.Color) ui.Color {
	var col ui.Color
	switch tb.(type) {
	case *tableTab:
		col = iconColor(p, "table")
	case *queryTab:
		col = iconColor(p, "query")
	default:
		return fallback
	}
	if !active {
		col = col.Alpha(0.8)
	}
	return col
}

// bellButton opens the notifications; a dot says there are new ones.
func (s *Shell) bellButton(c *ui.Context, p tokens) {
	fresh := len(s.notes) - s.notesSeen
	b := tipBtn(c, icBell, locales.ToolbarNotes())
	if fresh > 0 {
		b.Children(func() {
			ui.Icon(c, icBell).FontSize(15).TextColor(p.fgSecondary)
			ui.Box(c).Absolute().Top(6).Right(6).Size(6, 6).Radius(3).Background(p.danger)
		})
	}
	if b.Clicked() {
		s.notesOpen = true
		s.notesSeen = len(s.notes)
	}
}
