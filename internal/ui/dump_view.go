package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 向导和对话框共用的小部件（设计稿）：标题前的图标方块 dlgTile、表单行 dlgRow（右对齐的标签 + 控件）、提示条 dlgNotice。
// 备份、还原向导在 dump_wizard.go，状态与动作（startBackup / startRestore 等）在 dump.go。

// dlgTile 返回对话框标题前的图标方块（强调色淡底），给 dlgOpts.lead 用。
func dlgTile(c *ui.Context, p tokens, icon *ui.SVG) func() {
	return func() {
		ui.Row(c).Size(sizeDlgTile, sizeDlgTile).Shrink(0).Center().Radius(radiusPanel).Background(p.accentSoft).Children(func() {
			ui.Icon(c, icon).FontSize(btnActIcon).TextColor(p.accent)
		})
	}
}

// dlgRow 画对话框表单的一行：左边是定宽、右对齐的标签，右边是控件 fn。
func dlgRow(c *ui.Context, p tokens, label string, fn func()) {
	ui.Row(c).Gap(spaceMD).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(fontCaption).TextColor(p.fgMuted).Width(sizeDlgLabel).TextAlign(ui.End).Shrink(0)
		ui.Column(c).Gap(spaceXS).Grow(1).MinWidth(0).Children(fn)
	})
}

// dlgNotice 是对话框里的提示条：warn 为真用琥珀色，否则用中性色。
func dlgNotice(c *ui.Context, p tokens, text string, warn bool) {
	bg, fg, icon := p.surfaceSecondary, p.fgSecondary, icInfo
	if warn {
		bg, fg, icon = p.warningBg, p.warning, icShieldAlert
	}
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceSM+2, spaceMD).FillWidth().Radius(radiusPanel).Background(bg).Children(func() {
		ui.Icon(c, icon).FontSize(btnActIcon).TextColor(fg).Shrink(0)
		ui.Text(c, text).FontSize(fontDense).TextColor(p.fg).Grow(1).MinWidth(0)
	})
}
