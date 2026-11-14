package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 强调色固定：不论连接是什么环境，窗口主题、Shell.tokens 与 baseTokens 取到的强调色都是配色方案自己的那一个。
// 连接颜色只用在连接头像和生产环境的顶部色带上（见 docs/UI_TOKENS.md「使用规则」）。
func TestAccentIgnoresConnectionColor(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	w := e.sh.spaces[0]
	w.cfg.Tag = "production"
	var gotTokens, gotBase, gotTheme ui.Color
	e.tt.Frame()
	e.sh.probe = func(c *ui.Context) {
		gotTokens = e.sh.tokens(c).accent
		gotBase = baseTokens(c).accent
		gotTheme = c.Theme().Accent
	}
	e.tt.Frame()
	e.tt.Frame()
	want := tokensFor(activeScheme, false).accent
	if dark := tokensFor(activeScheme, true).accent; gotTokens != want && gotTokens != dark {
		t.Fatalf("Shell.tokens 的强调色 %v 既不是浅色 %v 也不是深色 %v 的方案强调色", gotTokens, want, dark)
	}
	for name, got := range map[string]ui.Color{"baseTokens": gotBase, "theme": gotTheme} {
		if got != gotTokens {
			t.Errorf("%s 的强调色 = %v，应与 Shell.tokens 一致 %v", name, got, gotTokens)
		}
	}
	if prod := ui.Hex(tagColors["production"]); gotTokens == prod {
		t.Errorf("强调色不应随生产环境变成 %v", prod)
	}
}
