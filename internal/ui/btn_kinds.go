package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 按钮的换皮变体：高度、字号、图标仍取 btn.go 的三档，只换底色、描边和文字色。
// 设计稿的按钮语义：默认（描边）/ 主要（强调色底）/ soft（强调淡底）/ ghost（无底）/
// 危险（红底）/ 危险淡底（红淡底）/ 成功（绿底）。主要和默认在 btn.go。

// btnKind 是按钮的语义变体。
type btnKind int

const (
	btnSoft       btnKind = iota // 强调淡底：AI 优化这类推荐动作
	btnGhost                     // 无底无边：弱操作
	btnDanger                    // 红底白字：二次确认里的最终删除
	btnDangerSoft                // 红淡底红字：删除入口
	btnOK                        // 绿底白字：成功后的确认
)

// skinBtn 给已建好的按钮 b 换上变体 k 的皮肤：悬停时底色变深一点。
// p 为当前令牌；返回 b 自己，可继续 .Clicked()、.Disabled()。
func skinBtn(b ui.Element, p tokens, k btnKind) ui.Element {
	var bg, hov, fg ui.Color
	border := ui.Transparent
	switch k {
	case btnSoft:
		bg, hov, fg = p.accentSoft2, p.accentSoft2.Mix(p.accent, 0.12), p.accentText
	case btnGhost:
		bg, hov, fg = ui.Transparent, p.hover, p.fgSecondary
	case btnDanger:
		bg, hov, fg, border = p.danger, shade(p.danger, -0.08), p.accentFg, p.danger
	case btnDangerSoft:
		bg, hov, fg = p.dangerBg, p.dangerBg.Mix(p.danger, 0.12), p.danger
	case btnOK:
		bg, hov, fg, border = p.success, shade(p.success, -0.08), p.accentFg, p.success
	}
	if b.Hovered() {
		bg = hov
	}
	return b.Background(bg).Border(1, border).TextColor(fg)
}

// softBtn 是强调淡底的操作按钮（22px，图标 + 文字）。
func softBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return skinBtn(actBtn(c, ic, label), baseTokens(c), btnSoft)
}

// ghostBtn 是无底无边的操作按钮（22px）。
func ghostBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return skinBtn(actBtn(c, ic, label), baseTokens(c), btnGhost)
}

// dangerBtn 是红底白字的按钮（26px，与输入框同排；对话框里请用 dangerFootBtn）。
func dangerBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return skinBtn(fieldBtn(c, ic, label), baseTokens(c), btnDanger)
}

// dangerSoftBtn 是红淡底红字的按钮（22px），用在「删除」这类入口上。
func dangerSoftBtn(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	return skinBtn(actBtn(c, ic, label), baseTokens(c), btnDangerSoft)
}

// dangerFootBtn 是对话框底部红底白字的确认键（30px，纯文字）。
func dangerFootBtn(c *ui.Context, label string) ui.Element {
	return skinBtn(footBtn(c, label), baseTokens(c), btnDanger).FontWeight(600)
}
