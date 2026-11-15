package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 侧栏顶部的「当前库」一行（设计稿 dbrow）：库图标 + 库名 + 上下箭头，点它弹出切库浮层（db_popover.go），
// 库名右边紧挨着安全级别小标签（只读的连接显示黄色「只读」）；最右是「新建数据库」和「更多」。
// 更多里是侧栏原来底栏的全部入口（新建表、导入导出备份、监控…），最后是安全级别的单选。

// icDatabasePlus 是「新建数据库」。
var (
	icDatabasePlus = svg(`<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.7 4 3 9 3"/><path d="M3 12c0 1.7 4 3 9 3"/><path d="M21 12V5"/><path d="M16 19h6"/><path d="M19 16v6"/>`)
)

// dbHeader 画当前库一行和下面可能出现的安全模式芯片。参数：w 是当前工作区。
func (s *Shell) dbHeader(c *ui.Context, p tokens, w *Workspace) {
	ui.Row(c.Key("db-head")).Height(sizeBtnLg + spaceSM).Gap(spaceXS).AlignItems(ui.Center).FillWidth().Children(func() {
		tip := locales.ToolbarSwitchDatabase() + s.keyHint("switcher")
		name := tipOn(c, ui.Button(c.Key("db-name"), "").Label(tip), tip).Shrink(1).MinWidth(0).MaxWidth(sizeDbName).Height(sizeBtn).Padding(0, spaceSM).Gap(spaceSM).
			Radius(radiusBtn).BorderWidth(0).Background(ui.Color{}).Justify(ui.Start).AlignItems(ui.Center)
		if name.Hovered() || s.sw.open {
			name.Background(p.hover)
		}
		name.Children(func() {
			ui.Icon(c, icDatabase).FontSize(btnFieldIcon).TextColor(p.fgSecondary).Shrink(0)
			label := w.dbLabel()
			if label == "" {
				label = locales.SidebarNoDatabase()
			}
			ui.Text(c, label).FontSize(fontBody).FontWeight(600).SingleLine().Shrink(1).MinWidth(0)
			ui.Icon(c, icChevUD).FontSize(fontDense).TextColor(p.fgMuted).Shrink(0)
		})
		name.Bind(&s.sw.anchor)
		s.sw.anchored = true
		if name.Clicked() {
			s.toggleSwitcher() // 再点一次收起
		}
		if w.cfg.ReadOnly {
			chip(c, chipWarn, locales.ConnReadOnlyShort(), false)
		} else {
			s.safeButton(c, p, w)
		}
		ui.Spacer(c)
		caps := app.CapsOf(w.cfg.Type)
		if caps.CreateDatabase {
			if tipBtn(c.Key("db-new"), icDatabasePlus, locales.DbNew()).Size(sizeBtnSm+spaceXS, sizeBtnSm+spaceXS).Disabled(w.cfg.ReadOnly).Clicked() {
				s.newDatabase(w)
			}
		}
		tipBtn(c.Key("db-more"), icMore, locales.ConnMore()).Size(sizeBtnSm+spaceXS, sizeBtnSm+spaceXS).PopoverMenuButton(func(m *ui.Menu) { s.dbMoreMenu(m, p, w) }, ui.PopoverMenuButtonOptions{})
	})
}

// dbMoreMenu 是当前库一行右边「更多」的菜单：切换数据库，侧栏原来底栏的全部入口（sideMoreMenu，每项带图标），
// 最后是「安全级别」一行子菜单（右边写着当前级别，展开才是各级别），不再把五级平铺在主菜单里。
func (s *Shell) dbMoreMenu(m *ui.Menu, p tokens, w *Workspace) {
	if menuItem(m, locales.ToolbarSwitchDatabase(), icDatabase).Chosen() {
		defer s.openSwitcher()
	}
	m.Separator()
	w.sideMoreMenu(m)
	m.Separator()
	s.safeLevelItem(m, p, w)
}
