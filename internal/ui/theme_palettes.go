package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 三套配色（石墨 / 纸墨 / 极光）× 明暗的色值，逐项对应设计稿的 CSS 变量。
// 数值的文字版在 docs/UI_TOKENS.md，两边由 TestTokensMatchDocs 对账；改这里要同步改文档。

// surfaceSet 是随配色变化的那部分令牌：表面、文字、线、强调色。
// 层级和设计稿 A 一致：bg 内容区；bg2 侧栏、状态栏、表头、对话框底栏；bg3 标题栏、活动栏、分段轨道、芯片底。
type surfaceSet struct {
	bg, bg2, bg3, elev, segOn ui.Color
	fg, fg2, fg3, fg4         ui.Color
	line, line2, stripe, hov  ui.Color
	acc, accT, accFg          ui.Color
	accBg, accBg2             ui.Color
}

// schemeSurface 取某个配色在某个明暗下的表面色。
func schemeSurface(pal scheme, dark bool) surfaceSet {
	h, a := ui.Hex, ui.RGBA
	switch pal {
	case schemePaper:
		if dark {
			return surfaceSet{h("#1c1a17"), h("#23201c"), h("#2d2924"), h("#2a2722"), h("#3d3830"),
				h("#efeae2"), h("#b9b0a3"), h("#948b7e"), h("#5e564b"),
				h("#36322b"), h("#2a2722"), h("#201e1a"), a(255, 240, 220, .055),
				h("#cf5a30"), h("#f28f68"), h("#ffffff"), a(242, 143, 104, .12), a(242, 143, 104, .20)}
		}
		return surfaceSet{h("#fbf8f1"), h("#f5f0e4"), h("#ebe4d3"), h("#fbf8f1"), h("#fbf8f1"),
			h("#2b2620"), h("#5d5547"), h("#968c79"), h("#bdb39f"),
			h("#ded5c1"), h("#e8e1d0"), h("#f8f4ea"), a(60, 40, 10, .055),
			h("#c2552d"), h("#a74d2b"), h("#ffffff"), a(194, 85, 45, .12), a(194, 85, 45, .20)}
	case schemeAurora:
		if dark {
			return surfaceSet{h("#0e1226"), h("#131833"), h("#1a2042"), h("#0e1226"), h("#0e1226"),
				h("#e9ecff"), h("#a3a9d6"), h("#6a71a3"), h("#444b7a"),
				h("#232a52"), h("#1a2042"), h("#111630"), a(160, 170, 255, .08),
				h("#7476ff"), h("#898bff"), h("#ffffff"), a(116, 118, 255, .12), a(116, 118, 255, .20)}
		}
		return surfaceSet{h("#ffffff"), h("#f2f6f8"), h("#e4ebf0"), h("#ffffff"), h("#ffffff"),
			h("#0f1720"), h("#455564"), h("#62727f"), h("#a7b3bd"),
			h("#dce4ea"), h("#ebf0f4"), h("#f9fbfc"), a(15, 23, 32, .05),
			h("#0b8c80"), h("#0a7f74"), h("#ffffff"), a(11, 140, 128, .10), a(11, 140, 128, .18)}
	}
	if dark { // 石墨
		return surfaceSet{h("#13161c"), h("#181c24"), h("#1f242e"), h("#13161c"), h("#13161c"),
			h("#eceef4"), h("#a6acbb"), h("#6f7686"), h("#474d5b"),
			h("#262b36"), h("#1e232c"), h("#161a21"), a(255, 255, 255, .055),
			h("#7476ff"), h("#a4a6ff"), h("#ffffff"), a(116, 118, 255, .12), a(116, 118, 255, .20)}
	}
	return surfaceSet{h("#ffffff"), h("#f6f7fa"), h("#eceef3"), h("#ffffff"), h("#ffffff"),
		h("#14161c"), h("#4b5160"), h("#8a90a1"), h("#b3b8c5"),
		h("#e3e5ec"), h("#eceef3"), h("#f9fafc"), a(20, 22, 28, .05),
		h("#5b5df0"), h("#4e50ca"), h("#ffffff"), a(91, 93, 240, .12), a(91, 93, 240, .20)}
}

// sharedTokens 是与配色无关的令牌：状态色、行状态、图标与数值语义色、语法色、图表色、阴影与遮罩。
// 取值同设计稿 A 的 CSS 变量：状态色 --ok / --warn / --err / --info，对象图标 info / violet / warn，数值取 info，时间取 --code-str。
func sharedTokens(dark bool) tokens {
	h, a := ui.Hex, ui.RGBA
	if dark {
		return tokens{
			success: h("#17a56a"), successBg: a(23, 165, 106, .14),
			warning: h("#d98a0b"), warningBg: a(217, 138, 11, .16),
			danger: h("#e5484d"), dangerBg: a(229, 72, 77, .14),
			info: h("#3b82f6"), infoBg: a(59, 130, 246, .14),
			rowNew: h("#14281f"), rowDelete: h("#2e1c20"), cellEdit: h("#33290f"),
			iconTable: h("#3b82f6"), iconView: h("#8b5cf6"), iconQuery: h("#d98a0b"), iconRoutine: h("#d98a0b"),
			valNum: h("#3b82f6"), valTime: h("#5fd6a0"),
			sxKeyword: h("#b794f6"), sxFunc: h("#7db2ff"), sxString: h("#5fd6a0"),
			sxNumber: h("#f2a672"), sxComment: h("#5b6272"), sxOp: h("#a6acbb"),
			chart:  [5]ui.Color{h("#7476ff"), h("#14b8a6"), h("#f2a672"), h("#ec4899"), h("#9aa7b8")},
			shadow: a(0, 0, 0, .65), scrim: a(0, 0, 0, .55),
		}
	}
	return tokens{
		success: h("#17a56a"), successBg: a(23, 165, 106, .14),
		warning: h("#d98a0b"), warningBg: a(217, 138, 11, .16),
		danger: h("#e5484d"), dangerBg: a(229, 72, 77, .14),
		info: h("#3b82f6"), infoBg: a(59, 130, 246, .14),
		rowNew: h("#e7f6ee"), rowDelete: h("#fdeaea"), cellEdit: h("#fdf0d4"),
		iconTable: h("#3b82f6"), iconView: h("#8b5cf6"), iconQuery: h("#d98a0b"), iconRoutine: h("#d98a0b"),
		valNum: h("#3b82f6"), valTime: h("#0f8a5f"),
		sxKeyword: h("#7c3aed"), sxFunc: h("#2563eb"), sxString: h("#0f8a5f"),
		sxNumber: h("#c2410c"), sxComment: h("#9aa0af"), sxOp: h("#4b5160"),
		chart:  [5]ui.Color{h("#5b5df0"), h("#14b8a6"), h("#e08a2e"), h("#ec4899"), h("#7b8aa3")},
		shadow: a(16, 20, 40, .18), scrim: a(18, 20, 30, .32),
	}
}
