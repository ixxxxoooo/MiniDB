package ui

import "testing"

// @author ixxxxoooo

// TestOpeningAConnectionAgainOpensANewWorkspace 同一个连接再次打开，是新开一个工作区，不是切回老的；
// 新工作区是全新的（不带老工作区的标签），复用已经连着的连接；只关掉其中一个时连接不断开。
func TestOpeningAConnectionAgainOpensANewWorkspace(t *testing.T) {
	e := newEnv(t)
	first := openWorkspace(e)
	first.openTable("users")
	e.tt.Frame()
	if len(e.sh.spaces) != 1 || len(first.tabs) != 1 {
		t.Fatalf("准备: spaces=%d tabs=%d", len(e.sh.spaces), len(first.tabs))
	}

	e.sh.openConnection(e.sh.conns[0])
	e.tt.Frame()
	if len(e.sh.spaces) != 2 || e.sh.active != 1 {
		t.Fatalf("再次打开同一个连接应新开一个工作区并切过去: spaces=%d active=%d", len(e.sh.spaces), e.sh.active)
	}
	second := e.sh.spaces[1]
	if second == first || second.cfg.ID != first.cfg.ID {
		t.Fatal("新工作区应是另一个对象、同一个连接")
	}
	if len(second.tabs) != 0 {
		t.Fatalf("新工作区是全新的，不应带上老工作区的标签: %d", len(second.tabs))
	}
	if len(first.tabs) != 1 {
		t.Fatalf("老工作区不受影响: tabs=%d", len(first.tabs))
	}
	if first.id() == second.id() {
		t.Fatalf("同一个连接、同一个库的两个工作区要有不同的界面键: %q", first.id())
	}
	if first.dbKey() != second.dbKey() {
		t.Fatal("存档键按连接和库，两个工作区共用")
	}
	if first.ordinal() != 1 || second.ordinal() != 2 {
		t.Fatalf("序号: %d, %d", first.ordinal(), second.ordinal())
	}
	e.shot("100-two-workspaces")

	// 再点第三次也是新开
	e.sh.openConnection(e.sh.conns[0])
	if len(e.sh.spaces) != 3 {
		t.Fatalf("第三次: spaces=%d", len(e.sh.spaces))
	}

	// 关掉后面两个，连接还连着；关掉最后一个才断开
	e.sh.closeSpace(2)
	e.sh.closeSpace(1)
	if _, err := e.sh.app.Manager.GetDB("c1"); err != nil {
		t.Fatalf("还剩一个工作区，连接不该断: %v", err)
	}
	e.sh.closeSpace(0)
	if _, err := e.sh.app.Manager.GetDB("c1"); err == nil {
		t.Fatal("最后一个工作区关掉后连接应断开")
	}
}

// TestSwitchingDatabaseStaysInTheWorkspace 切换数据库是在原来的工作区里就地切，不新开工作区。
func TestSwitchingDatabaseStaysInTheWorkspace(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.db = "main"
	before := w.id()
	e.sh.switchInPlace(w, "other")
	if len(e.sh.spaces) != 1 || e.sh.spaces[0] != w || w.db != "other" {
		t.Fatalf("切库应就地: spaces=%d db=%q", len(e.sh.spaces), w.db)
	}
	if w.id() == before {
		t.Fatal("切库后工作区的界面键应随库变（异步回调靠它判断已经切走）")
	}
}

// 从连接列表里点一个已经打开的连接，同样是新开一个工作区并关掉列表。
func TestConnectionListClickOnOpenConnectionOpensAnother(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.sh.newConnectionDialog()
	e.tt.Frame()
	e.click("local-sqlite")
	if len(e.sh.spaces) != 2 || e.sh.dlg.open {
		t.Fatalf("点已打开的连接应新开工作区: spaces=%d dialogOpen=%v", len(e.sh.spaces), e.sh.dlg.open)
	}
}
