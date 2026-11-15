package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 安全级别的两个入口（设计稿「安全级别列表」）：
//   - 库名右边的盾牌小标签（safeButton）：点开是自绘的下拉，和连接表单里的安全模式下拉（safeselect.go）是同一份内容；
//   - 「更多」菜单里的「安全级别」一行（safeLevelItem）：右边写着当前级别，展开是各级别的子菜单，不再把五级平铺在主菜单里。

// safeButton 画库名右边的盾牌小标签，点它弹出各级别列表。
// 参数：c 为界面上下文，p 为主题色板，w 为当前工作区。选了别的级别就交给 setSafeMode（可能要先输密码）。
func (s *Shell) safeButton(c *ui.Context, p tokens, w *Workspace) {
	cur := w.cfg.EffectiveSafeMode()
	col := safeModeColor(p, cur)
	bg := col.Alpha(0.13)
	if cur == model.SafeModeSilent {
		bg = p.surfaceTertiary
	}
	label := safeModeLabel(cur)
	sel := ui.SelectBase(c.Key("safe"), &label)
	tipOn(c, sel.Trigger, locales.SafeTitle()+": "+label+"\n"+safeModeTip(cur)).Height(sizeChipSq).Padding(0, spaceSM-2).Gap(spaceXS).AlignItems(ui.Center).Shrink(0).
		Radius(sizeChipSq / 2).Background(bg).Children(func() {
		ui.Icon(c, safeModeIcon(cur)).FontSize(fontCaption).TextColor(col)
		ui.Text(c, label).FontSize(fontCaption - 0.5).FontWeight(600).TextColor(col).SingleLine()
	})
	safePopup(c, p, sel, cur)
	if sel.Trigger.Changed() {
		s.setSafeMode(w, safeModeFromLabel(label))
	}
}

// safePopup 画安全级别的下拉内容：类别小标题 + 每级一行（盾牌图标 + 标题 + 一行说明），
// 当前级别淡强调底并带 ✓，悬停淡灰底，悬停久了浮出完整说明。参数：sel 为下拉，cur 为当前级别。
func safePopup(c *ui.Context, p tokens, sel ui.SelectParts[string], cur model.SafeMode) {
	sel.Popup(func(panel ui.Element) {
		panel.Padding(spaceXS).Gap(1).Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).MinWidth(sizeSafePop).
			Shadow(0, shadowY, shadowBlur, 0, p.shadow)
		for _, mode := range model.SafeModes {
			mode := mode
			if g := safeModeGroup(mode); g != "" {
				ui.Text(c, g).FontSize(font2XS).FontWeight(600).TextColor(p.fgMuted).Padding(spaceSM, spaceMD-2, spaceXS-1, spaceMD-2)
			}
			item := sel.Item(safeModeLabel(mode)).Padding(spaceSM-2, spaceMD-2).Gap(spaceSM).AlignItems(ui.Center).Radius(radiusBtn)
			tipOn(c, item, safeModeTip(mode))
			switch {
			case item.Highlighted():
				item.Background(p.hover)
			case mode == cur:
				item.Background(p.accentSoft)
			}
			item.Children(func() {
				ui.Icon(c, safeModeIcon(mode)).FontSize(btnFieldIcon).TextColor(safeModeColor(p, mode)).Shrink(0)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, safeModeLabel(mode)).FontSize(fontBase).FontWeight(500).SingleLine()
					ui.Text(c, safeModeDesc(mode)).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
				})
				if mode == cur {
					ui.Icon(c, icCheck).FontSize(btnFieldIcon).TextColor(p.accentText).Shrink(0)
				}
			})
		}
	})
}

// safeLevelItem 在「更多」菜单 m 里加「安全级别」一行：盾牌图标，右边写当前级别，展开是各级别的单选子菜单。
func (s *Shell) safeLevelItem(m *ui.Menu, p tokens, w *Workspace) {
	cur := w.cfg.EffectiveSafeMode()
	menuItem2(m, locales.SafeTitle(), safeModeIcon(cur), func(sm *ui.Menu) {
		for _, mode := range model.SafeModes {
			if safeModeGroup(mode) != "" { // 提醒类、安全类之间画条分隔线
				sm.Separator()
			}
			it := menuItem(sm, safeModeLabel(mode), safeModeIcon(mode)).IconColor(safeModeColor(p, mode)).
				Radio(mode == cur).Subtitle(safeModeDesc(mode)).Tooltip(safeModeTip(mode))
			if it.Chosen() {
				defer s.setSafeMode(w, mode)
			}
		}
	}).Trailing(safeModeLabel(cur)).IconColor(safeModeColor(p, cur))
}
