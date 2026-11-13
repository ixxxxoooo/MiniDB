package ui

import "testing"

// 标签栏与排序标记的外观快照（设 MINIDB_SHOTS 目录才会落盘）。
func TestTabStyleShot(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	for range 3 {
		e.tt.Frame()
	}
	_ = e.tt.Click("orders")
	for range 3 {
		e.tt.Frame()
	}
	e.sh.spaces[0].active = 0
	for range 8 {
		sleepMs(20)
		e.tt.Frame()
	}
	_ = e.tt.Click("name")
	for range 10 {
		sleepMs(20)
		e.tt.Frame()
	}
	e.shot("tabs-style")
}
