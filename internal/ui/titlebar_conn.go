package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 标题栏左边的连接胶囊（设计稿 cchip）：头像、连接名、环境标记、状态点、已打开连接数、下拉箭头。
// 点它从下面弹出「已打开的连接」浮层（conn_popover.go）。右边的搜索只剩一枚图标（searchIcon）。

// connChip 画连接胶囊，并记下它作锚点给连接浮层。参数：w 是当前工作区。
func (s *Shell) connChip(c *ui.Context, p tokens, w *Workspace) {
	lost := s.lost[w.cfg.ID]
	chip := ui.Row(c.Key("conn-chip")).Height(sizeBtn).Padding(0, spaceSM, 0, spaceXS+1).Gap(spaceSM - 1).AlignItems(ui.Center).Shrink(0).Radius(radiusBtn)
	switch {
	case s.wsPop.open:
		chip.Background(p.hover)
	case chip.Hovered():
		chip.Background(p.hover)
	}
	tipOn(c, chip, s.pillDetails(w)).Children(func() {
		dbTile(c, w.cfg.Type, sizeChipTile, connColor(c, cfgColor(w.cfg)))
		ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(600).SingleLine().MaxWidth(sizeChipName).Shrink(1)
		if tag := w.cfg.Tag; tag != "" && tag != "local" {
			sqChip(c, tagChipKind(tag), tagShort(tag))
		}
		dot := p.success
		switch {
		case lost:
			dot = p.danger
		case len(s.connecting) > 0:
			dot = p.warning
		}
		ui.Box(c.Key("chip-dot")).Size(sizeDot+1, sizeDot+1).Shrink(0).Radius((sizeDot + 1) / 2).Background(dot)
		ui.Row(c.Key("chip-n")).Height(sizeChip).MinWidth(sizeChip).Padding(0, spaceXS+1).Shrink(0).Center().Radius(sizeChip / 2).Background(p.segTrack).Children(func() {
			ui.Text(c, itoa(len(s.spaces))).FontSize(fontMicro).FontWeight(600).TextColor(p.fgSecondary).SingleLine()
		})
		ui.Icon(c, icChevronD).FontSize(fontDense).TextColor(p.fgMuted).Shrink(0)
	})
	chip.Bind(&s.wsPop.anchor)
	s.wsPop.anchored = true
	if chip.Clicked() {
		s.openWorkspaces()
	}
}

// searchIcon 是标题栏右边的搜索：只有一枚图标，点一下打开命令面板。
// 无障碍标签与悬浮提示沿用「快速搜索 + 快捷键」。
func (s *Shell) searchIcon(c *ui.Context) {
	tip := locales.ToolbarSearch() + s.keyHint("palette")
	if tipBtn(c, icSearch, tip).Size(sizeBtn, sizeBtn).Clicked() {
		s.cmdPalette()
	}
}
