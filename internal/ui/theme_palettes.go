package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 三套配色（石墨 / 纸墨 / 极光）× 明暗的色值，逐项对应设计稿的 CSS 变量。
// 数值的文字版在 docs/UI_TOKENS.md，两边由 TestTokensMatchDocs 对账；改这里要同步改文档。

// surfaceSet 是随配色变化的那部分令牌：表面、文字、线、强调色。
type surfaceSet struct {
	bg, bg2, bg3, elev, segOn ui.Color
	fg, fg2, fg3              ui.Color
	line, line2, stripe       ui.Color
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
				h("#efeae2"), h("#b9b0a3"), h("#948b7e"), h("#36322b"), h("#2a2722"), h("#201e1a"),
				h("#cf5a30"), h("#f28f68"), h("#ffffff"), a(242, 143, 104, .10), a(242, 143, 104, .18)}
		}
		return surfaceSet{h("#fdfcfa"), h("#f5f2ed"), h("#ebe6de"), h("#fffefc"), h("#fffefc"),
			h("#26221d"), h("#5a5247"), h("#74695c"), h("#e6e0d6"), h("#f0ebe3"), h("#faf8f4"),
			h("#c1481a"), h("#b24218"), h("#ffffff"), a(193, 72, 26, .08), a(193, 72, 26, .14)}
	case schemeAurora:
		if dark {
			return surfaceSet{h("#0f141b"), h("#0b1016"), h("#18202b"), h("#161d27"), h("#243040"),
				h("#e4eaf2"), h("#9ba8b9"), h("#7b899c"), h("#222b38"), h("#19212b"), h("#121820"),
				h("#1fbfab"), h("#35d9c4"), h("#04201c"), a(53, 217, 196, .09), a(53, 217, 196, .16)}
		}
		return surfaceSet{h("#ffffff"), h("#f2f6f8"), h("#e4ebf0"), h("#ffffff"), h("#ffffff"),
			h("#0f1720"), h("#455564"), h("#62727f"), h("#dce4ea"), h("#ebf0f4"), h("#f9fbfc"),
			h("#0b8c80"), h("#0a7f74"), h("#ffffff"), a(11, 140, 128, .08), a(11, 140, 128, .14)}
	}
	if dark { // 石墨
		return surfaceSet{h("#15171c"), h("#1b1e24"), h("#262a33"), h("#232730"), h("#373d49"),
			h("#e8eaef"), h("#aab0bd"), h("#868d9b"), h("#2c3039"), h("#22262d"), h("#191c21"),
			h("#686af4"), h("#a4a6ff"), h("#ffffff"), a(130, 132, 255, .11), a(130, 132, 255, .19)}
	}
	return surfaceSet{h("#ffffff"), h("#f6f7f9"), h("#eceef2"), h("#ffffff"), h("#ffffff"),
		h("#16181d"), h("#4b5260"), h("#6b7383"), h("#e1e4ea"), h("#eef0f4"), h("#fafbfc"),
		h("#5b5df0"), h("#4d4fd0"), h("#ffffff"), a(91, 93, 240, .08), a(91, 93, 240, .14)}
}

// sharedTokens 是与配色无关的令牌：状态色、行状态、图标与数值语义色、语法色、图表色、阴影与遮罩。
func sharedTokens(dark bool) tokens {
	h, a := ui.Hex, ui.RGBA
	if dark {
		return tokens{
			success: h("#4cc38a"), successBg: a(76, 195, 138, .14),
			warning: h("#eab04a"), warningBg: a(234, 176, 74, .14),
			danger: h("#f27575"), dangerBg: a(242, 117, 117, .14),
			info: h("#74aef7"), infoBg: a(116, 174, 247, .14),
			rowNew: h("#1c3326"), rowDelete: h("#3b2227"), cellEdit: h("#3b3520"),
			iconTable: h("#74a6f7"), iconView: h("#b896f4"), iconQuery: h("#4cc38a"), iconRoutine: h("#eab04a"),
			valNum: h("#83abff"), valTime: h("#4fd1c5"),
			sxKeyword: h("#b59cff"), sxFunc: h("#7fb5ff"), sxString: h("#7fd8a3"),
			sxNumber: h("#f2a76d"), sxComment: h("#6e7688"), sxOp: h("#a9afbc"),
			chart:  [5]ui.Color{h("#9a9cff"), h("#4fd1c5"), h("#f2a76d"), h("#f27aa8"), h("#9aa7b8")},
			shadow: a(0, 0, 0, .5), scrim: a(0, 0, 0, .5),
		}
	}
	return tokens{
		success: h("#16794c"), successBg: a(22, 121, 76, .10),
		warning: h("#a45f00"), warningBg: a(217, 130, 0, .13),
		danger: h("#c9302c"), dangerBg: a(201, 48, 44, .09),
		info: h("#1f64b8"), infoBg: a(31, 100, 184, .09),
		rowNew: h("#e7f4e8"), rowDelete: h("#fce8e8"), cellEdit: h("#fff3c4"),
		iconTable: h("#3d78d4"), iconView: h("#8656cc"), iconQuery: h("#1d9363"), iconRoutine: h("#c26f10"),
		valNum: h("#2b5cc4"), valTime: h("#0c7f80"),
		sxKeyword: h("#6a43d6"), sxFunc: h("#1f66c2"), sxString: h("#16794c"),
		sxNumber: h("#c0560a"), sxComment: h("#7c8494"), sxOp: h("#5a6270"),
		chart:  [5]ui.Color{h("#5b5df0"), h("#1b9a89"), h("#e08a2e"), h("#d4538a"), h("#7b8aa3")},
		shadow: a(16, 24, 40, .16), scrim: a(16, 20, 30, .34),
	}
}
