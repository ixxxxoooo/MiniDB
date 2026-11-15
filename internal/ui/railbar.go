package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 活动栏：窗口最左边 44px 宽的一条，把原来藏在「…」菜单里的功能外露出来。
// 从上到下：侧栏的三个页（项目 / 查询 / 历史），一条分隔线，项目概览、ER 图、监控、用户与权限、
// 导入导出；最下面是侧栏开关、主题切换和设置。选中的页用强调淡底，左边缘一条 3px 的强调色竖线。
// 它取代了原来 60px 宽的「工作区竖条」：在多个连接之间切换改在侧栏顶部的连接卡里做。

// railBtn 画活动栏上的一个图标按钮：on 为真是选中态；tip 同时是无障碍标签和悬浮提示。
func railBtn(c *ui.Context, p tokens, key string, ic *ui.SVG, tip string, on bool) ui.Element {
	b := tipOn(c, ui.Button(c.Key(key), "").Label(tip), tip).Size(sizeRailBtn, sizeRailBtn).Padding(0).
		Radius(radiusRail).BorderWidth(0).Background(ui.Color{}).Shrink(0)
	col := p.fgMuted
	switch {
	case on:
		b.Background(p.accentSoft)
		col = p.accentText
	case b.Hovered():
		b.Background(p.hover)
		col = p.fg
	}
	return b.Children(func() {
		ui.Icon(c, ic).FontSize(sizeRailIcon).TextColor(col)
		if on { // 竖线贴在活动栏的左边缘：按钮在 44px 里居中，左边留了 6px
			ui.Box(c.Key("mark")).Absolute().Left(-(sizeRail - sizeRailBtn) / 2).Top(8).Bottom(8).Width(3).Radius(1.5).Background(p.accent)
		}
	})
}

// railSep 是活动栏里两组按钮之间的短分隔线。
func railSep(c *ui.Context, p tokens, key string) {
	ui.Box(c.Key(key)).Width(18).Height(1).Background(p.border).Margin(4, 0)
}

// activityRail 画活动栏。没有打开的连接时只剩新建连接、主题和设置。
func (s *Shell) activityRail(c *ui.Context, p tokens) {
	w := s.activeWorkspace()
	ui.Row(c.Key("rail")).FillHeight().Shrink(0).Children(func() {
		ui.Column(c).Width(sizeRail).FillHeight().Padding(8, 0).Gap(3).AlignItems(ui.Center).Background(p.surfaceTertiary).Shrink(0).Children(func() {
			if w == nil {
				if railBtn(c, p, "r-new", icPlug, locales.ToolbarNewConnection()+s.keyHint("new-conn"), false).Clicked() {
					s.newConnectionDialog()
				}
			} else {
				s.railProject(c, p, w)
			}
			ui.Spacer(c)
			if w != nil && railBtn(c, p, "r-sidebar", icSidebar, locales.ToolbarSidebar()+s.keyHint("sidebar"), !w.sidebarHidden).Clicked() {
				w.sidebarHidden = !w.sidebarHidden // 设计稿：侧栏开关在活动栏底部，主题和设置上面
			}
			themeTip, themeIcon := locales.ToolbarToDark(), icMoon
			if c.Theme().Dark {
				themeTip, themeIcon = locales.ToolbarToLight(), icSun
			}
			if railBtn(c, p, "r-theme", themeIcon, themeTip, false).Clicked() {
				s.toggleTheme(c.Theme().Dark)
			}
			if railBtn(c, p, "r-settings", icSettings, locales.ToolbarSettings()+s.keyHint("settings"), false).Clicked() {
				s.openSettings(0)
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(p.border)
	})
}

// railProject 画活动栏上跟连接有关的那一组：侧栏三个页，然后是各个功能入口。
func (s *Shell) railProject(c *ui.Context, p tokens, w *Workspace) {
	pages := []struct {
		ic  *ui.SVG
		tip string
	}{{icDatabase, locales.SidebarItems()}, {icFileSQL, locales.SidebarQueries()}, {icHistory, locales.SidebarHistory()}}
	for i, pg := range pages {
		// 当前页再点一次：收起侧栏（和编辑器类应用的活动栏一样）；点别的页：展开并切过去
		on := !w.sidebarHidden && w.side.seg == i
		if railBtn(c, p, "r-page"+itoa(i), pg.ic, pg.tip, on).Clicked() {
			w.showSidePage(i, on)
		}
	}
	railSep(c, p, "r-sep1")
	active := w.activeTab()
	_, onOverview := active.(*overviewTab)
	_, onER := active.(*erTab)
	_, onMonitor := active.(*monitorTab)
	if railBtn(c, p, "r-overview", icGrid, locales.OverviewTitle(), onOverview).Clicked() {
		w.openOverview()
	}
	if app.CanER(w.cfg.Type) && railBtn(c, p, "r-er", icER, locales.ErOpenMenu(), onER).Clicked() {
		w.openER("")
	}
	if len(monitorTabs(w)) > 0 && railBtn(c, p, "r-monitor", icActivity, locales.MonitorTitle()+s.keyHint("monitor"), onMonitor).Clicked() {
		s.cmdMonitor()
	}
	if app.SupportsUsers(w.cfg.Type) && railBtn(c, p, "r-users", icUsers, locales.UsersMenu(), s.usr.open).Clicked() {
		s.openUsers()
	}
	railBtn(c, p, "r-transfer", icImpExp, locales.RailImportExport(), s.bak.open || s.rst.open || s.xf.open).PopoverMenuButton(func(m *ui.Menu) {
		w.transferMenu(m)
	}, ui.PopoverMenuButtonOptions{})
}

// showSidePage 切到侧栏的第 i 页；on 为真表示它已经是当前页，这时再点就收起侧栏。
func (w *Workspace) showSidePage(i int, on bool) {
	if on {
		w.sidebarHidden = true
		return
	}
	w.sidebarHidden = false
	if w.side.seg != i {
		w.side.seg = i
		w.loadSideQueries()
	}
}

// activeTab 是当前标签，没有标签时为 nil。
func (w *Workspace) activeTab() tab {
	if w.active < 0 || w.active >= len(w.tabs) {
		return nil
	}
	return w.tabs[w.active]
}

// transferMenu 是「导入导出」入口的弹出菜单：导出表、导入文件、备份、恢复。
func (w *Workspace) transferMenu(m *ui.Menu) {
	if m.Item(locales.ExportTablesMenu()).Chosen() {
		defer func() { w.sh.exportTables(w) }()
	}
	if m.Item(locales.ImportFileMenu()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer func() { w.sh.importFile(w) }()
	}
	m.Separator()
	if m.Item(locales.DumpBackupMenu()).Chosen() {
		defer func() { w.sh.openBackup() }()
	}
	if app.CapsOf(w.cfg.Type).Network && m.Item(locales.DumpRestoreMenu()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer func() { w.sh.openRestore() }()
	}
}
