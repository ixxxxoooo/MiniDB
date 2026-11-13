package ui

import (
	"fmt"
	"strings"
	"time"

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
				if len(s.spaces) > 1 {
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
					if toggleBtn(c, icSidebar, locales.ToolbarSidebar()+s.keyHint("sidebar"), !w.sidebarHidden).Clicked() {
						w.sidebarHidden = !w.sidebarHidden
					}
				}
				if tipBtn(c, icPlug, locales.ToolbarNewConnection()+s.keyHint("new-conn")).Clicked() {
					s.newConnectionDialog()
				}
				if tipBtn(c, icUnplug, locales.ToolbarDisconnect()).Disabled(w == nil).Clicked() && w != nil {
					s.closeSpaceGuarded(w) // 只断开当前这个工作区；连接在它的最后一个工作区关掉时才真正断开
				}
				vline(c, p.border, 12)
				if w != nil {
					if tipBtn(c, icDatabase, locales.ToolbarSwitchDatabase()+s.keyHint("switcher")).Clicked() {
						s.openSwitcher()
					}
					s.safeButton(c, p, w)
					sql := tipOn(c, ui.Button(c, "SQL"), locales.ToolbarSQL()+s.keyHint("new-query")).Font(mono).FontSize(font2XS).Height(sizeBtn)
					if sql.Clicked() {
						s.cmdNewQuery()
					}
				}
				ui.Spacer(c)
				s.taskBar(c)
				s.pendingGroup(c, p)
				if w != nil {
					s.indexChip(c, p, w)
					vline(c, p.border, 12)
				}
				if toggleBtn(c, icConsole, locales.ToolbarConsole()+s.keyHint("console"), s.consoleOpen).Clicked() {
					s.consoleOpen = !s.consoleOpen
				}
				s.bellButton(c, p)
				vline(c, p.border, 12)
				if tipBtn(c, icSearch, locales.ToolbarSearch()+s.keyHint("palette")).Clicked() {
					s.cmdPalette()
				}
				if toggleBtn(c, icSparkle, locales.ToolbarAI()+s.keyHint("ai"), w != nil && w.panel == 2).Disabled(w == nil).Clicked() {
					s.cmdToggleAI()
				}
				vline(c, p.border, 12)
				themeTip, themeIcon := locales.ToolbarToDark(), icMoon
				if c.Theme().Dark {
					themeTip, themeIcon = locales.ToolbarToLight(), icSun
				}
				if tipBtn(c, themeIcon, themeTip).Clicked() {
					s.toggleTheme(c.Theme().Dark)
				}
				if tipBtn(c, icSettings, locales.ToolbarSettings()+s.keyHint("settings")).Clicked() {
					s.openSettings(0)
				}
			})
		}).Padding(0)
	})
}

// pill 是顶栏中间的连接胶囊：状态点、连接名、环境、只读、「/库名」、数据库类型与版本，
// 以及一直显示的重连按钮。点胶囊本身打开连接列表；切库用左侧的数据库图标。
func (s *Shell) pill(c *ui.Context, p tokens, w *Workspace) {
	pill := ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 12).Height(sizeBtn).Radius(999).Background(p.surfaceSecondary).Border(1, p.border)
	if pill.Hovered() {
		pill.Background(p.hover)
	}
	reconnect := false
	lost := s.lost[w.cfg.ID]
	tipOn(c, pill, s.pillDetails(w)).Children(func() {
		dot := p.success
		switch {
		case lost:
			dot = p.danger
		case len(s.connecting) > 0:
			dot = p.warning
		}
		// 查询或翻页要超过 0.4 秒才转圈：一次排序、翻页只要几毫秒，状态点一闪变成转圈再变回来，标题栏就像在闪
		if w.busyFor(c, 400*time.Millisecond) {
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
		col := p.fgSecondary
		if lost {
			col = p.danger
		}
		// 胶囊里的小图标按钮：无边框无底色，悬停时才浮出一个圆形底
		rb := tipOn(c, ui.Button(c.Key("pill-reconnect"), "").Label(locales.ToolbarReconnect()), locales.ToolbarReconnect()).
			Size(20, 20).Padding(0).Radius(10).BorderWidth(0).Background(ui.Color{}).Shrink(0)
		if rb.Hovered() {
			rb.Background(p.border)
		}
		if rb.Children(func() {
			if s.reconnecting[w.cfg.ID] {
				ui.Spinner(c).Size(12, 12)
			} else {
				ui.Icon(c, icRefresh).FontSize(12).TextColor(col)
			}
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

// busyFor 报告 w 里有没有查询或表格翻页已经加载了超过 d：再短的加载不值得在标题栏上显示。
func (w *Workspace) busyFor(c *ui.Context, d time.Duration) bool {
	for _, tb := range w.tabs {
		switch t := tb.(type) {
		case *queryTab:
			if t.running && delayed(c, t.started, d) {
				return true
			}
		case *tableTab:
			if t.loading && delayed(c, t.loadStart, d) {
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
	lines = append(lines, locales.WelcomeTitle()+s.keyHint("open-conn"))
	return strings.Join(lines, "\n")
}

// reconnect 断开再重连 w 的连接。过程中胶囊里的按钮转圈，结束后弹提示告诉用户成功还是失败：
// 之前它静悄悄地跑完，用户点了看不到任何变化，就像没点中一样。
func (s *Shell) reconnect(w *Workspace) {
	id := w.cfg.ID
	if s.reconnecting[id] {
		return
	}
	s.reconnects++
	s.reconnecting[id] = true
	var ok bool
	var msg string
	s.up.bg(func() {
		start := time.Now()
		_ = s.app.Connections.Disconnect(id)
		ok, msg = s.app.Connections.Connect(id)
		// 本地库一瞬间就连好了，转圈至少停留一会儿，否则看起来仍然没反应
		if d := 700*time.Millisecond - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}, func() {
		delete(s.reconnecting, id)
		if !ok {
			s.lost[id] = true
			s.toastErr(msg)
			return
		}
		delete(s.lost, id)
		s.toastOK(locales.ConnRestored(w.cfg.Name))
		w.loadDatabases()
	})
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
				tipOn(c, item, w.cfg.Name+" - "+w.db).Children(func() {
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
	// 底边那条分隔线先画、横贯整条标签栏；选中的标签带着和内容区同色的底盖在它上面，
	// 线就在它下面断开，标签和下面的内容看起来是连成一体的。
	ui.Row(c).FillWidth().Height(sizeTab).Shrink(0).Background(p.toolbar).Children(func() {
		ui.Box(c.Key("tab-baseline")).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
		ui.ScrollHorizontal(c.Key("tabstrip")).FillWidth().Shrink(0).Scrollbars(false).Children(func() {
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
	})
	if !middleDown {
		w.midKey = ""
	}
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
	tip := locales.ToolbarNotes()
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtn, sizeBtn).Padding(0).BorderWidth(0).Background(ui.Color{})
	col := p.fgSecondary
	if b.Hovered() {
		col = p.fg
	}
	b.Children(func() {
		ui.Icon(c, icBell).FontSize(15).TextColor(col)
		if fresh > 0 { // 有没看过的通知：右上角一个红点，有失败的就更醒目
			dot := p.accent
			for _, n := range s.notes[s.notesSeen:] {
				if n.kind == kindError {
					dot = p.danger
				}
			}
			ui.Box(c).Absolute().Top(5).Right(5).Size(7, 7).Radius(3.5).Background(dot).Border(1.5, p.toolbar)
		}
	})
	if b.Clicked() {
		s.notesOpen = !s.notesOpen
		s.notesSeen = len(s.notes)
	}
	s.notesPopover(c, p, b)
}
