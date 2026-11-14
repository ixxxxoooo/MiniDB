package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

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
					sql := tipOn(c, fieldBtn(c, icTerminal, "SQL"), locales.ToolbarSQL()+s.keyHint("new-query"))
					if sql.Clicked() {
						s.cmdNewQuery()
					}
				}
				ui.Spacer(c)
				s.updateChip(c, p)
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
		if w.db != "" && app.CapsOf(w.cfg.Type).MultiDatabase {
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
	var connErr error
	s.up.bg(func() {
		start := time.Now()
		_ = s.app.Connections.Disconnect(id)
		connErr = s.app.Connections.Connect(id)
		// 本地库一瞬间就连好了，转圈至少停留一会儿，否则看起来仍然没反应
		if d := 700*time.Millisecond - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}, func() {
		delete(s.reconnecting, id)
		if connErr != nil {
			s.lost[id] = true
			s.toastErr(errMessage(connErr))
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
