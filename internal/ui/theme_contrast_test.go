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

// TestLineContrastParity 约束两套主题里线条与层级的对比度落在同一档：
// 以前深色的分隔线对底色是 2.2:1、斑马纹 1.26:1，比浅色（1.5:1、1.09:1）重得多，看上去"线条很粗"。
// 这里只守住区间，具体色值仍以 theme.go 为准。
func TestLineContrastParity(t *testing.T) {
	for name, p := range map[string]tokens{"light": lightTokens, "dark": darkTokens} {
		check := func(what string, got, lo, hi float64) {
			t.Helper()
			if got < lo || got > hi {
				t.Errorf("%s %s 对比度 %.2f 不在 [%.2f, %.2f]", name, what, got, lo, hi)
			}
		}
		check("border/surface", contrast(p.border, p.surface), 1.45, 1.85)
		check("border/toolbar", contrast(p.border, p.toolbar), 1.35, 1.8)
		check("borderSubtle/surface", contrast(p.borderSubtle, p.surface), 1.2, 1.45)
		check("borderSubtle/surfaceSecondary", contrast(p.borderSubtle, p.surfaceSecondary), 1.1, 1.3)
		check("stripe/surface", contrast(p.stripe, p.surface), 1.04, 1.12)
		check("hover/surface", contrast(p.hover, p.surface), 1.04, 1.4)
		// 分段开关的滑块要比轨道更"凸"：浅色更亮，深色也更亮
		if relLum(p.segThumb, p.surface) <= relLum(p.segTrack, p.surface) {
			t.Errorf("%s 分段开关的滑块应比轨道更亮", name)
		}
		if contrast(p.border, p.surface) <= contrast(p.borderSubtle, p.surface) {
			t.Errorf("%s border 应比 borderSubtle 更明显", name)
		}
	}
}
