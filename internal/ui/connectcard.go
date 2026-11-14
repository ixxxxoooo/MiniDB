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
// 侧栏顶部的「连接上下文卡片」：连接、库、安全模式一处看全。
//   - 第一行是连接卡：头像（连接色，右下角是连接状态点）、连接名、环境标记（非本地才显示）、
//     数据库类型与版本、地址；点它列出所有打开的连接，可以在里面切换、新建、断开；
//   - 第二行是库选择（点开切库）和安全模式芯片（点开选级别）；
//   - 第三行是筛选框，随时都在，不用先呼出。
//
// 这几样原来分散在标题栏中间的连接胶囊和一排图标按钮里，现在跟着侧栏走，侧栏收起时它们也一起收起。

// sideHeader 画侧栏顶部的整块：连接卡、库与安全模式、筛选框。
func (w *Workspace) sideHeader(c *ui.Context, p tokens) {
	s := w.sh
	ui.Column(c.Key("side-head")).Gap(spaceSM-2).Padding(spaceSM+2, spaceSM+2, spaceSM-2, spaceSM+2).FillWidth().Shrink(0).Children(func() {
		s.connectionCard(c, p, w)
		ui.Row(c.Key("side-db")).Gap(spaceSM - 2).AlignItems(ui.Center).FillWidth().Children(func() {
			s.dbSelect(c, p, w)
			s.safeButton(c, p, w)
		})
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

// connectionCard 画连接卡。点它打开「所有连接」列表（openWorkspaces）。
func (s *Shell) connectionCard(c *ui.Context, p tokens, w *Workspace) {
	lost := s.lost[w.cfg.ID]
	dot := p.success
	switch {
	case lost:
		dot = p.danger
	case len(s.connecting) > 0:
		dot = p.warning
	}
	card := ui.Row(c.Key("conn-card")).Gap(spaceSM).AlignItems(ui.Center).Padding(7, spaceSM).FillWidth().Radius(radiusPanel).Background(p.surface).Border(1, p.border)
	if card.Hovered() {
		card.Background(p.hover)
	}
	tipOn(c, card, s.pillDetails(w)).Children(func() {
		ui.Box(c.Key("avatar")).Size(sizeAvatar, sizeAvatar).Shrink(0).Children(func() {
			dbTile(c, w.cfg.Type, sizeAvatar, connColor(c, cfgColor(w.cfg)))
			ui.Box(c.Key("state")).Absolute().Right(-2).Bottom(-2).Size(sizeDot+4, sizeDot+4).Radius((sizeDot+4)/2).Background(dot).Border(2, p.surface)
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(spaceSM - 2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(600).SingleLine().Shrink(1)
				if tag := w.cfg.Tag; tag != "" && tag != "local" { // 环境标记：生产红、测试橙…
					sqChip(c, tagChipKind(tag), strings.ToUpper(tagLabel(tag)))
				}
			})
			ui.Text(c, cardSubtitle(w)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		})
		ui.Icon(c, icChevUD).FontSize(btnActIcon).TextColor(p.fgMuted).Shrink(0)
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
	case "development":
		return chipInfo
	}
	return chipOK
}

// dbSelect 画库选择框：点一下打开切库面板（⌘P 同款）。
func (s *Shell) dbSelect(c *ui.Context, p tokens, w *Workspace) {
	tip := locales.ToolbarSwitchDatabase() + s.keyHint("switcher")
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Height(sizeSideSel).Grow(1).MinWidth(0).Padding(0, spaceSM).Gap(spaceSM-2).
		Radius(radiusBtn).Border(1, p.border).Background(p.surface).Justify(ui.Start).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, icDatabase).FontSize(btnActIcon).TextColor(p.fgSecondary).Shrink(0)
		name := w.dbLabel()
		if name == "" {
			name = locales.SidebarNoDatabase()
		}
		ui.Text(c, name).FontSize(fontDense).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, icChevronD).FontSize(fontCaption).TextColor(p.fgMuted).Shrink(0)
	})
	if b.Clicked() {
		s.openSwitcher()
	}
}
