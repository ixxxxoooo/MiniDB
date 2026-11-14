package ui

import "testing"

// 更新进度按下载字节算百分比；总大小未知时是 0，不会除零。
func TestUpdatePercent(t *testing.T) {
	var u updateState
	if u.percent() != 0 {
		t.Fatal("总大小未知时应为 0")
	}
	u.total.Store(200)
	u.done.Store(50)
	if u.percent() != 25 {
		t.Fatalf("percent = %d", u.percent())
	}
	u.done.Store(999)
	if u.percent() != 100 {
		t.Fatal("不应超过 100")
	}
}

// 更新提示只在需要用户注意时出现：下载中显示进度，下载好了「重启更新」，没有更新什么也不画。
func TestUpdateChipFollowsPhase(t *testing.T) {
	e := newEnv(t)
	e.sh.upd.version = "9.9.9"
	for phase, want := range map[updatePhase]string{
		upDownloading: "正在下载更新", upReady: "新版本 9.9.9 · 重启更新", upAvailable: "新版本 9.9.9 · 点击下载",
	} {
		e.sh.upd.phase = phase
		e.tt.Frame()
		e.has(want)
	}
	e.sh.upd.phase = upUpToDate
	e.tt.Frame()
	e.hasNot("重启更新")
	// 开发版（没有更新源）不会自己检查，也不会改状态
	e.sh.upd.phase = upIdle
	e.sh.checkUpdates(true)
	if e.sh.upd.phase != upIdle {
		t.Fatal("没有启用更新的构建不应进入检查状态")
	}
}
