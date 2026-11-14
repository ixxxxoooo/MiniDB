package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 芯片（状态标签）与按键帽。设计稿 A：芯片高 18、圆角满、字号 10.5、字重 600；
// 方角芯片（环境标记 PROD / DEV）高 16、圆角 4、字号 9.5、字重 700、字距 .04em。颜色只表达状态：
// 中性 / 成功 / 警告 / 失败 / 提示 / 强调，各自的淡底和文字色取自令牌。

// chipKind 是芯片的语义。
type chipKind int

const (
	chipNeutral chipKind = iota // 中性：灰底（引擎、类型）
	chipOK                      // 成功：已连接
	chipWarn                    // 警告：待提交
	chipErr                     // 失败 / 危险：生产环境
	chipInfo                    // 提示：只读
	chipAccent                  // 强调：已固定
)

// chipColors 返回芯片的底色和文字色。
func chipColors(p tokens, k chipKind) (bg, fg ui.Color) {
	switch k {
	case chipOK:
		return p.successBg, p.success
	case chipWarn:
		return p.warningBg, p.warning
	case chipErr:
		return p.dangerBg, p.danger
	case chipInfo:
		return p.infoBg, p.info
	case chipAccent:
		return p.accentSoft, p.accentText
	}
	return p.surfaceTertiary, p.fgSecondary
}

// chip 画一枚圆角芯片：k 为语义，text 为文字；dot 为真时文字前带一个实心小圆点（用于「已连接」这类状态）。
// 返回元素，调用方可以继续 .Margin() 等。
func chip(c *ui.Context, k chipKind, text string, dot bool) ui.Element {
	p := baseTokens(c)
	bg, fg := chipColors(p, k)
	return ui.Row(c).Height(sizeChip).Padding(0, 7).Gap(spaceXS).AlignItems(ui.Center).Shrink(0).Radius(sizeChip / 2).Background(bg).Children(func() {
		if dot {
			ui.Box(c).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(fg).Shrink(0)
		}
		ui.Text(c, text).FontSize(fontCaption - 0.5).FontWeight(600).TextColor(fg).SingleLine()
	})
}

// sqChip 画一枚方角小芯片，用于环境标记（PROD / DEV）。k、text 同 chip。
func sqChip(c *ui.Context, k chipKind, text string) ui.Element {
	p := baseTokens(c)
	bg, fg := chipColors(p, k)
	return ui.Row(c).Height(sizeChipSq).Padding(0, 5).AlignItems(ui.Center).Shrink(0).Radius(radiusXs).Background(bg).Children(func() {
		ui.Text(c, text).FontSize(fontMicro - 0.5).FontWeight(700).LetterSpacing(0.4).TextColor(fg).SingleLine()
	})
}

// kbd 画一枚按键帽（⌘ ⏎ 这类快捷键提示）：1px 描边，底边加粗到 2px。
func kbd(c *ui.Context, text string) ui.Element {
	p := baseTokens(c)
	return ui.Row(c).Height(sizeKbd).Padding(0, 5).Radius(4).Background(p.surfaceSecondary).Border(1, p.border).Shrink(0).Center().Children(func() {
		ui.Text(c, text).FontSize(fontMicro).FontWeight(600).TextColor(p.fgMuted).SingleLine()
	})
}
