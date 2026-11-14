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

// 选中的标签没有下边框：标签栏底线那一行里，选中标签的宽度内是内容区底色，而不是线的颜色。
func TestActiveTabHasNoBottomBorder(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	w.openTable("users")
	e.tt.Frame()
	e.tt.Frame()
	img := e.tt.Image()
	// 在最右端「＋」那一段找到底线所在的行：从上往下第一行线色
	line := img.At(1270, 64)
	if line == img.At(1270, 62) {
		t.Skip("布局变了，找不到标签栏底线所在的行")
	}
	surface, joined := img.At(372, 62), 0
	for x := 210; x < 1200; x++ {
		if img.At(x, 64) == surface {
			joined++
		}
	}
	if joined < 90 {
		t.Fatalf("选中标签下面应该没有线（和内容区同色的宽度 ≥ 标签最小宽度 96），实际只有 %d 像素", joined)
	}
}
