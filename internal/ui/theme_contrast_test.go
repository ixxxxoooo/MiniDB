package ui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// relLum 是 WCAG 的相对亮度；半透明色先叠到 base 上再算。
func relLum(c, base ui.Color) float64 {
	a := float64(c.A) / 255
	ch := func(v, b uint8) float64 {
		x := (a*float64(v) + (1-a)*float64(b)) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R, base.R) + 0.7152*ch(c.G, base.G) + 0.0722*ch(c.B, base.B)
}

// contrast 是两个颜色的对比度（>= 1），b 是 a 所叠的底色。
func contrast(a, b ui.Color) float64 {
	la, lb := relLum(a, b), relLum(b, b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestSchemeContrast 约束三套配色的明暗两面：线条的轻重落在同一档，文字和强调色保证可读。
// 设计稿的线比旧版更淡（浅色 line 对白底约 1.27:1），区间按它推导；具体色值以 docs/UI_TOKENS.md 为准。
func TestSchemeContrast(t *testing.T) {
	for si, sn := range schemeNames {
		for _, dark := range []bool{false, true} {
			p := tokensFor(scheme(si), dark)
			name := sn + map[bool]string{false: "/浅色", true: "/深色"}[dark]
			check := func(what string, got, lo, hi float64) {
				t.Helper()
				if got < lo || got > hi {
					t.Errorf("%s %s 对比度 %.2f 不在 [%.2f, %.2f]", name, what, got, lo, hi)
				}
			}
			// 线条：border 是区域分隔与控件描边，borderSubtle 是卡片内部细线。
			check("border/surface", contrast(p.border, p.surface), 1.2, 1.45)
			check("border/surfaceSecondary", contrast(p.border, p.surfaceSecondary), 1.1, 1.4)
			check("borderSubtle/surface", contrast(p.borderSubtle, p.surface), 1.08, 1.25)
			check("hover/surface", contrast(p.hover, p.surface), 1.1, 1.3)
			// 文字：正文 AAA，次要 AA；弱化文字（设计稿 A 的 --fg-3，用于计数、辅助说明）不低于 2.9:1，比旧版淡。
			check("fg/surface", contrast(p.fg, p.surface), 7, 30)
			check("fgSecondary/surface", contrast(p.fgSecondary, p.surface), 4.5, 30)
			check("fgMuted/surface", contrast(p.fgMuted, p.surface), 3, 30)
			check("fgMuted/surfaceSecondary", contrast(p.fgMuted, p.surfaceSecondary), 2.9, 30)
			// 强调色：文字色在底上可读，实心按钮上的字也可读。
			check("accentText/surface", contrast(p.accentText, p.surface), 4.5, 30)
			check("accentFg/accent", contrast(p.accentFg, p.accent), 3.5, 30) // 设计稿深色靛蓝 #7476ff 上的白字约 3.6:1
			// 分段开关的滑块和轨道要分得开（设计稿深色里滑块是内容区底色，比轨道更暗）。
			if p.segThumb == p.segTrack {
				t.Errorf("%s 分段开关的滑块和轨道不能同色", name)
			}
			if contrast(p.border, p.surface) <= contrast(p.borderSubtle, p.surface) {
				t.Errorf("%s border 应比 borderSubtle 更明显", name)
			}
			// 强调色固定：和连接颜色无关，两套明暗下都不是零值。
			if p.accent == (ui.Color{}) || p.accentHover == p.accent {
				t.Errorf("%s 强调色或其悬停色缺失", name)
			}
		}
	}
}
