package ui

import (
	"reflect"
	"testing"
	"unsafe"
)

// @author ixxxxoooo
//
// 标题栏空白处必须是「纯拖动手柄」：mygo 只有在按下的位置上最近的可交互元素只带 DragWindow 一个标记时才拖动窗口，
// 手柄上多读一个 Clicked / DoubleClicked 就会变成可点击元素、拖不动。无窗口测试看不到真正的拖动，
// 但同一个条件决定双击会不会交给窗口（最大化），Tester 里记着这个次数，这里用反射读出来。

// windowDoubleClicks 是窗口收到的标题栏双击次数（Tester 内部的 titleDoubles）。
func windowDoubleClicks(e *env) int {
	h := reflect.ValueOf(e.tt).Elem().FieldByName("h")
	f := h.Elem().FieldByName("titleDoubles")
	return int(*(*int)(unsafe.Pointer(f.UnsafeAddr())))
}

func TestTitleBarBlankMovesWindow(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tabs := len(w.tabs)
	for _, x := range []float32{520, 600, 700} { // 标签之后、右侧按钮之前的空白
		before := windowDoubleClicks(e)
		e.tt.ClickAt(x, 18)
		e.tt.Frame()
		e.tt.ClickAt(x, 18)
		e.tt.Frame()
		if windowDoubleClicks(e) != before+1 {
			t.Errorf("x=%v：标题栏空白处的双击没有交给窗口，说明它不是纯拖动手柄，也就拖不动窗口", x)
		}
	}
	if len(w.tabs) != tabs {
		t.Errorf("双击标题栏空白处不应新建标签：%d → %d", tabs, len(w.tabs))
	}
}

func TestWelcomeTitleBarBlankMovesWindow(t *testing.T) {
	e := newEnv(t)
	e.tt.Frame()
	before := windowDoubleClicks(e)
	e.tt.ClickAt(300, 18)
	e.tt.Frame()
	e.tt.ClickAt(300, 18)
	e.tt.Frame()
	if windowDoubleClicks(e) != before+1 {
		t.Error("欢迎页标题栏空白处不是纯拖动手柄")
	}
}
