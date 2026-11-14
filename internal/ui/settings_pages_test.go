package ui

import (
	"testing"
	"time"

	"minidb/internal/model"
)

func TestSettingsPagesRender(t *testing.T) {
	e := newEnv(t)
	want := map[int][]string{
		setGeneral:    {"恢复默认设置"},
		setData:       {"斑马纹", "显示行号"},
		setEditor:     {"运行"},
		setCSV:        {"分隔符"},
		setConn:       {"连接保活", "查询超时"},
		setAppearance: {"编辑器字号", "浅色"},
		setSecurity:   {"新连接的安全模式"},
	}
	for tab, texts := range want {
		e.sh.openSettings(tab)
		e.tt.Frame()
		for _, s := range texts {
			e.has(s)
		}
	}
}

func TestSettingsPrefsTakeEffect(t *testing.T) {
	e := newEnv(t)
	pf := &e.sh.prefs
	pf.NoZebra, pf.QueryTimeout, pf.KeepaliveSec = true, 60, 300
	e.sh.applyRuntimePrefs()
	if showZebra {
		t.Fatal("斑马纹应关闭")
	}
	if got := e.sh.app.Connections.KeepaliveInterval(); got != 5*time.Minute {
		t.Fatalf("保活间隔 %v", got)
	}
	pf.KeepaliveSec = -1
	e.sh.applyRuntimePrefs()
	if e.sh.app.Connections.KeepaliveInterval() != 0 {
		t.Fatal("保活应关闭")
	}

	// 新建连接预选设置里的安全模式
	pf.DefaultSafe = string(model.SafeModeAlert2)
	e.sh.newConnection()
	if got := safeModeFromLabel(e.sh.dlg.safeSel); got != model.SafeModeAlert2 || !e.sh.dlg.safeSet {
		t.Fatalf("新连接应预选提醒 2，实际 %v", got)
	}

	// 恢复默认
	e.sh.set.theme = 2
	e.sh.resetPrefs()
	if e.sh.prefs.QueryTimeout != 0 || !showZebra || e.sh.set.theme != 0 || e.sh.app.Connections.KeepaliveInterval() != 30*time.Second {
		t.Fatalf("恢复默认没有生效: %+v", e.sh.prefs)
	}
}

func TestDurationLabel(t *testing.T) {
	cases := []struct {
		sec  int
		zero string
		want string
	}{{-1, "z", "关闭"}, {0, "不限", "不限"}, {30, "", "30 秒"}, {300, "", "5 分钟"}, {90, "", "90 秒"}}
	for _, c := range cases {
		if got := durationLabel(c.sec, c.zero); got != c.want {
			t.Errorf("durationLabel(%d)=%q want %q", c.sec, got, c.want)
		}
	}
}
