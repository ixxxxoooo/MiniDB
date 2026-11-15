package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 侧栏顶部的「当前库」一行（设计稿 dbrow）：库图标 + 库名 + 上下箭头，点它弹出切库浮层（db_popover.go）；
// 右边是「新建数据库」和「更多」。更多里是刷新、切库、新建库，以及安全模式的各级别。

// icDatabasePlus 是「新建数据库」。
var (
	icDatabasePlus = svg(`<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.7 4 3 9 3"/><path d="M3 12c0 1.7 4 3 9 3"/><path d="M21 12V5"/><path d="M16 19h6"/><path d="M19 16v6"/>`)
)

// dbHeader 画当前库一行和下面可能出现的安全模式芯片。参数：w 是当前工作区。
func (s *Shell) dbHeader(c *ui.Context, p tokens, w *Workspace) {
	ui.Row(c.Key("db-head")).Height(sizeBtnLg + spaceSM).Gap(spaceXS).AlignItems(ui.Center).FillWidth().Children(func() {
		tip := locales.ToolbarSwitchDatabase() + s.keyHint("switcher")
		name := tipOn(c, ui.Button(c.Key("db-name"), "").Label(tip), tip).Grow(1).MinWidth(0).Height(sizeBtn).Padding(0, spaceSM).Gap(spaceSM).
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
			s.toggleSwitcher(false) // 再点一次收起
		}
		caps := app.CapsOf(w.cfg.Type)
		if caps.CreateDatabase {
			if tipBtn(c.Key("db-new"), icDatabasePlus, locales.DbNew()).Size(sizeBtnSm+spaceXS, sizeBtnSm+spaceXS).Disabled(w.cfg.ReadOnly).Clicked() {
				s.newDatabase(w)
			}
		}
		tipBtn(c.Key("db-more"), icMore, locales.ConnMore()).Size(sizeBtnSm+spaceXS, sizeBtnSm+spaceXS).PopoverMenuButton(func(m *ui.Menu) { s.dbMoreMenu(m, w) }, ui.PopoverMenuButtonOptions{})
	})
	if cur := w.cfg.EffectiveSafeMode(); cur != model.SafeModeSilent || w.cfg.ReadOnly {
		ui.Row(c.Key("db-chips")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Padding(0, spaceSM, spaceXS, spaceSM).Children(func() {
			s.safeButton(c, p, w)
			if w.cfg.ReadOnly {
				chip(c, chipWarn, locales.ConnReadOnlyShort(), false)
			}
		})
	}
}

// dbMoreMenu 是当前库一行右边「更多」的菜单：刷新、切库、新建库，然后是安全模式的各级别（单选）。
func (s *Shell) dbMoreMenu(m *ui.Menu, w *Workspace) {
	if m.Item(locales.CommonRefresh()).Shortcut(ui.Cmd, ui.KeyR).Chosen() {
		defer w.loadTables()
	}
	if m.Item(locales.ToolbarSwitchDatabase()).Chosen() {
		defer s.openSwitcher()
	}
	if app.CapsOf(w.cfg.Type).CreateDatabase && m.Item(locales.DbNew()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer func() { s.newDatabase(w) }()
	}
	m.Separator()
	cur := w.cfg.EffectiveSafeMode()
	for _, mode := range model.SafeModes {
		if safeModeGroup(mode) != "" {
			m.Separator()
		}
		if m.Item(locales.SafeTitle() + " · " + safeModeLabel(mode)).Radio(mode == cur).Subtitle(safeModeDesc(mode)).Chosen() {
			defer s.setSafeMode(w, mode)
		}
	}
}
