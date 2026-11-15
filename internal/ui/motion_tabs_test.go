package ui

import (
	"testing"
	"time"
)

// @author ixxxxoooo
//
// 第五轮补的动效：标签新增 / 关闭、活动栏选中、筛选芯片增删。
// 动效只表达状态：新开 / 关掉标签的过渡要有帧，过渡走完后一帧不多（不能空转烧 CPU）。

// 新开一个标签有入场过渡，走完之后不再出帧。
func TestTabOpenAnimatesThenSettles(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	e.tt.Advance(time.Second)
	e.tt.Frame()
	w.openTable("orders")
	e.tt.Frame()
	if !e.tt.FramePending() {
		t.Fatal("新开的标签应有入场过渡（还有帧要出）")
	}
	e.tt.Advance(time.Second)
	e.tt.Frame()
	e.tt.Advance(time.Second)
	if e.tt.FramePending() {
		t.Fatal("过渡走完后不该再出帧")
	}
}

// 关掉前面的标签，后面的标签是同一个对象，不会被当成新标签重新入场（key 按标签对象，而不是下标）。
func TestClosingATabDoesNotReplayOthers(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	w.openTable("users")
	e.tt.Frame()
	e.tt.Advance(time.Second)
	e.tt.Frame()
	e.tt.Advance(time.Second)
	if e.tt.FramePending() {
		t.Fatal("前置：标签都稳定了不该再出帧")
	}
	n := len(w.tabs)
	w.closeTab(0)
	e.tt.Frame()
	if len(w.tabs) != n-1 {
		t.Fatalf("closeTab 应关掉一个标签: %d → %d", n, len(w.tabs))
	}
	e.tt.Advance(time.Second)
	e.tt.Frame()
	e.tt.Advance(time.Second)
	if e.tt.FramePending() {
		t.Fatal("关掉标签的过渡走完后不该再出帧")
	}
	e.has(w.tabs[len(w.tabs)-1].title()) // 剩下的标签还在
}
