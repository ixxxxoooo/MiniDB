package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 侧栏顶部的「连接上下文」（设计稿 A 的 conn-h / dbsel / search）：
//   - 第一行是连接头：22px 头像（缩写 + 连接色渐变）、连接名、环境标记（非本地才显示）、
//     数据库类型与版本、地址、右端是切换图标；整行没有边框和底色，点它列出所有打开的连接；
//   - 第二行是一个带边框的库选择框：库名、安全模式芯片、下拉箭头，点库名切库，点芯片选安全级别；
//   - 第三行是筛选框，随时都在，不用先呼出。
//
// 侧栏收起时它们也一起收起。

// sideHeader 画侧栏顶部的整块：连接头、库与安全模式、筛选框。
func (w *Workspace) sideHeader(c *ui.Context, p tokens) {
	s := w.sh
	ui.Column(c.Key("side-head")).Padding(0, spaceSM+2, 0, spaceSM+2).FillWidth().Shrink(0).Children(func() {
		s.connectionCard(c, p, w)
		s.dbSelect(c, p, w)
	})
	w.sideFilter(c)
}

// cardSubtitle 是连接卡第二行：数据库类型（带版本）和地址；本地文件库显示文件名。
func cardSubtitle(w *Workspace) string {
	ver := typeLabel(w.cfg.Type)
	if w.version != "" {
		if strings.HasPrefix(strings.ToLower(w.version), strings.ToLower(ver)) {
			ver = w.version
		} else {
			ver += " " + w.version
		}
	}
	if !app.CapsOf(w.cfg.Type).Network {
		return ver + " · " + filepath.Base(w.cfg.Database)
	}
	return ver + " · " + w.cfg.Host + ":" + itoa(w.cfg.Port)
}

// connectionCard 画连接头。点它打开「所有连接」列表（openWorkspaces）。
func (s *Shell) connectionCard(c *ui.Context, p tokens, w *Workspace) {
	lost := s.lost[w.cfg.ID]
	card := ui.Row(c.Key("conn-card")).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceSM+1, 0, spaceSM-2, 0).FillWidth().Radius(radiusBtn)
	tipOn(c, card, s.pillDetails(w)).Children(func() {
		ui.Box(c.Key("avatar")).Size(sizeAvatar, sizeAvatar).Shrink(0).Children(func() {
			dbTile(c, w.cfg.Type, sizeAvatar, connColor(c, cfgColor(w.cfg)))
			if lost || len(s.connecting) > 0 { // 正常时不画状态点（状态栏有「已连接」）；断开、连接中才在头像上提示
				dot := p.warning
				if lost {
					dot = p.danger
				}
				ui.Box(c.Key("state")).Absolute().Right(-2).Bottom(-2).Size(sizeDot+2, sizeDot+2).Radius((sizeDot+2)/2).Background(dot).Border(1.5, p.surfaceSecondary)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(spaceSM - 2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(650).SingleLine().Shrink(1)
				if tag := w.cfg.Tag; tag != "" && tag != "local" { // 环境标记：生产红、测试橙…
					sqChip(c, tagChipKind(tag), tagShort(tag))
				}
			})
			ui.Text(c, cardSubtitle(w)).FontSize(fontCaption - 0.5).TextColor(p.fgMuted).SingleLine()
		})
		swap := ui.Row(c.Key("conn-swap")).Size(sizeBtnSm, sizeBtnSm).Shrink(0).Radius(radiusBtn).Center()
		if swap.Hovered() || card.Hovered() {
			swap.Background(p.hover)
		}
		swap.Children(func() { ui.Icon(c, icSwap).FontSize(fontCtl).TextColor(p.fgMuted) })
	})
	if card.Clicked() {
		s.openWorkspaces()
	}
}

// tagChipKind 是环境标签对应的芯片语义：生产是失败色，测试和预发是警告色，其余是成功色。
func tagChipKind(tag string) chipKind {
	switch tag {
	case "production":
		return chipErr
	case "test", "staging":
		return chipWarn
	}
	return chipOK
}

// dbSelect 画库选择框：一个 26px 的带边框条，里面是库名、安全模式芯片和下拉箭头。
// 点库名或箭头打开切库面板（⌘P 同款），点芯片选安全级别。
func (s *Shell) dbSelect(c *ui.Context, p tokens, w *Workspace) {
	tip := locales.ToolbarSwitchDatabase() + s.keyHint("switcher")
	box := ui.Row(c.Key("side-db")).Height(sizeSideSel).FillWidth().Padding(0, spaceSM, 0, 0).Gap(spaceSM-2).AlignItems(ui.Center).Radius(radiusBtn).Border(1, p.border).Background(p.surface)
	box.Children(func() {
		b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).FillHeight().Grow(1).MinWidth(0).Padding(0, 0, 0, spaceSM).Gap(spaceSM - 2).
			Radius(radiusBtn).BorderWidth(0).Background(ui.Color{}).Justify(ui.Start).AlignItems(ui.Center)
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Icon(c, icDatabase).FontSize(btnActIcon).TextColor(p.fgSecondary).Shrink(0)
			name := w.dbLabel()
			if name == "" {
				name = locales.SidebarNoDatabase()
			}
			ui.Text(c, name).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine().Grow(1).MinWidth(0)
		})
		s.safeButton(c, p, w)
		ui.Icon(c, icChevronD).FontSize(fontDense).TextColor(p.fgMuted).Shrink(0)
		if b.Clicked() {
			s.openSwitcher()
		}
	})
}
