package ui

import "testing"

// 点标题栏胶囊里的重连按钮必须真的触发重连，且不能顺带把连接列表打开。
func TestPillReconnectClick(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	e.tt.Frame()
	before := e.sh.reconnects
	if err := e.tt.Click("重新连接"); err != nil {
		t.Fatalf("找不到重连按钮: %v", err)
	}
	e.tt.Frame()
	if e.sh.reconnects != before+1 {
		t.Fatalf("点重连按钮应触发一次重连，实际 %d -> %d", before, e.sh.reconnects)
	}
	if e.sh.dlg.open {
		t.Fatal("点重连按钮不应打开连接列表")
	}
}
