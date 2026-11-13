package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 工作区的主题色要贯穿整个界面：窗口主题的强调色、Shell.tokens 与 baseTokens 取到的都是它。
func TestWorkspaceColorDrivesAccent(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	w := e.sh.spaces[0]
	w.cfg.Color = "#ff2d55"
	var gotTokens, gotBase, gotTheme ui.Color
	e.tt.Frame()
	want := ui.Hex("#ff2d55")
	e.sh.probe = func(c *ui.Context) {
		gotTokens = e.sh.tokens(c).accent
		gotBase = baseTokens(c).accent
		gotTheme = c.Theme().Accent
	}
	e.tt.Frame()
	e.tt.Frame()
	for name, got := range map[string]ui.Color{"tokens": gotTokens, "baseTokens": gotBase, "theme": gotTheme} {
		if got != want {
			t.Errorf("%s 的强调色 = %v，应为工作区颜色 %v", name, got, want)
		}
	}
}
