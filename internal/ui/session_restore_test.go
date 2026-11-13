package ui

import "testing"

// 关闭前打开的连接，下次启动要按原来的顺序连回来，并回到当时在前台的那个。
func TestRestoreLastSessionOnLaunch(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newQuery("SELECT 1")
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name = "c2", "second"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.openConnection(cfg2)
	e.sh.active = 0 // 关闭前停在第一个
	e.sh.SaveState()

	// 同一份数据再起一个壳，相当于重新启动应用
	sh := NewShell(e.sh.app, fakeHost{}, true)
	sh.loadConnections()
	if len(sh.spaces) != 2 || sh.spaces[0].cfg.ID != "c1" || sh.spaces[1].cfg.ID != "c2" {
		t.Fatalf("要按顺序恢复两个连接: n=%d", len(sh.spaces))
	}
	if sh.active != 0 {
		t.Fatalf("要回到关闭前在前台的连接: active=%d", sh.active)
	}
	if len(sh.spaces[0].tabs) == 0 {
		t.Fatal("连接里的标签页也要一并恢复")
	}
}

// 在设置里关掉"恢复上次的连接"后，启动时不再自动连接，也不会丢掉已有的存档。
func TestRestoreSessionCanBeTurnedOff(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.sh.SaveState()

	sh := NewShell(e.sh.app, fakeHost{}, true)
	sh.prefs.NoRestore = true
	sh.loadConnections()
	if len(sh.spaces) != 0 {
		t.Fatalf("关掉恢复后不该自动连接: n=%d", len(sh.spaces))
	}
}
