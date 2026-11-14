package ui

import (
	"os"
	"testing"
)

// TestShotsTabJoin 渲染选中标签与内容区衔接处，检查标签下面有没有多余的线。
func TestShotsTabJoin(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	for _, dark := range []bool{false, true} {
		suffix := map[bool]string{false: "light", true: "dark"}[dark]
		e := newEnv(t)
		e.tt.SetDark(dark)
		w := openWorkspace(e)
		w.openTable("orders")
		w.openTable("users")
		e.tt.Frame()
		tab := w.tabs[w.active].(*tableTab)
		tab.mode = 2
		tab.modeChanged()
		e.shot("j1-ddl-" + suffix)
		tab.mode = 0
		tab.modeChanged()
		e.shot("j2-data-" + suffix)
	}
}
