package ui

import (
	"testing"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 强调色选项：默认不动令牌，选了之后只换强调色那几项，深色下用单独调过的亮度，点色板写进偏好。

func TestAccentOverridesOnlyAccentTokens(t *testing.T) {
	defer func() { activeAccent = "" }()
	base := tokensFor(schemeGraphite, false)
	activeAccent = "teal"
	teal := tokensFor(schemeGraphite, false)
	if teal.accent != accentSwatches["teal"].light || teal.accent == base.accent {
		t.Fatalf("浅色下青色强调色不对: %v", teal.accent)
	}
	if teal.surface != base.surface || teal.fg != base.fg || teal.border != base.border {
		t.Fatal("换强调色不应动底色、文字色和线条")
	}
	if dark := tokensFor(schemeGraphite, true); dark.accent != accentSwatches["teal"].dark || dark.accentFg != accentSwatches["teal"].darkFg {
		t.Fatalf("深色下应取深色版本: %v", dark.accent)
	}
	activeAccent = "mono"
	if m := tokensFor(schemeGraphite, false); m.accent != base.fg {
		t.Fatal("单色强调色应跟随文字色")
	}
	activeAccent = "no-such"
	if tokensFor(schemeGraphite, false).accent != base.accent {
		t.Fatal("不认识的名字应退回默认强调色")
	}
}

func TestAccentSwatchesWriteBackToPrefs(t *testing.T) {
	e := newEnv(t)
	defer func() {
		e.sh.prefs = uiPrefs{}
		e.sh.applyRuntimePrefs()
	}()
	e.sh.openSettings(setAppearance)
	e.tt.Frame()
	if err := e.tt.Click(locales.AccentRose()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.sh.prefs.Accent != "rose" || activeAccent != "rose" {
		t.Fatalf("点玫红应写进偏好并生效: %q %q", e.sh.prefs.Accent, activeAccent)
	}
	if err := e.tt.Click(locales.AccentIndigo()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.sh.prefs.Accent != "" || activeAccent != "" {
		t.Fatal("点靛蓝应回到各配色的默认强调色")
	}
}
