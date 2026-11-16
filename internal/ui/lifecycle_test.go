package ui

import (
	"testing"

	"github.com/egoist/mygo"
)

// 点关闭按钮只能藏起窗口，不能让它真的关掉（关掉最后一个窗口应用就退出了）。
func TestWindowCloseIsKeptOpen(t *testing.T) {
	e := newEnv(t)
	ev := &mygo.CloseEvent{}
	e.sh.onWindowClose(ev)
	if !ev.DefaultPrevented() {
		t.Fatal("closing the window must be prevented: the app keeps running, only Cmd+Q quits")
	}
}

// 退出过程中关窗不能再拦，否则 Cmd+Q 会被自己取消。
func TestWindowCloseDuringQuitGoesThrough(t *testing.T) {
	e := newEnv(t)
	e.sh.exiting = true
	ev := &mygo.CloseEvent{}
	e.sh.onWindowClose(ev)
	if ev.DefaultPrevented() {
		t.Fatal("a window closing as part of a quit must not be prevented")
	}
}
