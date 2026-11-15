package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 标题栏左边的连接胶囊（设计稿 cchip）：数据库图标、连接名、环境标记、下拉箭头，宽度随名称、最宽约 200。
// 已打开的连接数在悬停提示和浮层标题里，连接状态（连接中 / 掉线）看底部状态栏。
// 点它从下面弹出「已打开的连接」浮层（conn_popover.go）。右边的搜索只剩一枚图标（searchIcon）。

// connChip 画连接胶囊，并记下它作锚点给连接浮层。参数：w 是当前工作区。
func (s *Shell) connChip(c *ui.Context, p tokens, w *Workspace) {
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
		ui.Icon(c, icChevronD).FontSize(fontDense).TextColor(p.fgMuted).Shrink(0)
	})
	chip.Bind(&s.wsPop.anchor)
	s.wsPop.anchored = true
	if chip.Clicked() {
		s.toggleWorkspaces() // 再点一次收起
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
