package ui

import (
	"runtime"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func TestChordRoundTrip(t *testing.T) {
	for _, ch := range []chord{
		{mui.Cmd, mui.KeyN}, {mui.Cmd | mui.Shift, mui.KeyBracketLeft}, {mui.Cmd | mui.Ctrl, mui.KeyLeft},
		{mui.Cmd | mui.Alt, mui.KeyPeriod}, {0, mui.KeyF5}, {mui.Cmd, mui.KeyF12}, {mui.Cmd | mui.Shift, mui.KeyBackspace},
	} {
		got, ok := parseChord(ch.String())
		if !ok || got != ch {
			t.Errorf("%q 读回来是 %+v（ok=%v），应为 %+v", ch.String(), got, ok, ch)
		}
	}
	if _, ok := parseChord("cmd+nope"); ok {
		t.Error("不认识的键不应解析成功")
	}
	if _, ok := parseChord("hyper+a"); ok {
		t.Error("不认识的修饰键不应解析成功")
	}
	if got := (chord{mui.Cmd | mui.Shift, mui.KeyF}).accelerator(); got != map[bool]string{true: "Super+Shift+f", false: "Ctrl+Shift+f"}[runtime.GOOS == "darwin"] {
		t.Errorf("accelerator=%q", got)
	}
	if got := (chord{mui.Cmd, mui.KeyBracketLeft}).accelerator(); got[len(got)-1] != '[' {
		t.Errorf("accelerator=%q", got)
	}
	if runtime.GOOS == "darwin" {
		if got := (chord{mui.Cmd | mui.Shift, mui.KeyE}).Label(); got != "⇧⌘E" {
			t.Errorf("Label=%q", got)
		}
		if got := (chord{mui.Cmd | mui.Ctrl, mui.KeyLeft}).Label(); got != "⌃⌘←" {
			t.Errorf("Label=%q", got)
		}
	}
}

func TestCommandDefaultsAreBindable(t *testing.T) {
	seen := map[string]bool{}
	for _, cm := range commands {
		if seen[cm.id] {
			t.Errorf("命令 %s 重复", cm.id)
		}
		seen[cm.id] = true
		if !cm.def.bindable() {
			t.Errorf("命令 %s 的默认键 %s 不合规", cm.id, cm.def.Label())
		}
		if cm.name() == "" {
			t.Errorf("命令 %s 没有名称", cm.id)
		}
		if got, ok := parseChord(cm.def.String()); !ok || got != cm.def {
			t.Errorf("命令 %s 的默认键存不回来", cm.id)
		}
	}
}

func TestBindKeyRules(t *testing.T) {
	e := newEnv(t)
	s := e.sh
	if got := s.chordOf("new-query"); got != (chord{mui.Cmd, mui.KeyT}) {
		t.Fatalf("默认键 %+v", got)
	}
	// 不带 ⌘/Ctrl 的普通键、Esc 不能绑
	if s.bindKey("new-query", chord{0, mui.KeyA}) == "" || s.bindKey("new-query", chord{mui.Cmd, mui.KeyEscape}) == "" {
		t.Fatal("普通键和 Esc 应被拒绝")
	}
	// 与别的命令冲突
	if msg := s.bindKey("new-query", chord{mui.Cmd, mui.KeyN}); msg == "" {
		t.Fatal("⌘N 已被新建连接占用")
	}
	// 与固定键冲突
	if s.bindKey("new-query", chord{mui.Cmd, mui.KeyC}) == "" || s.bindKey("new-query", chord{mui.Cmd, mui.Key3}) == "" {
		t.Fatal("⌘C、⌘3 是固定键")
	}
	if len(s.prefs.Keymap) != 0 {
		t.Fatalf("被拒绝时不应改动: %v", s.prefs.Keymap)
	}
	// 正常改键
	if msg := s.bindKey("new-query", chord{mui.Cmd | mui.Shift, mui.KeyT}); msg != "" {
		t.Fatal(msg)
	}
	if s.chordOf("new-query") != (chord{mui.Cmd | mui.Shift, mui.KeyT}) {
		t.Fatal("改键没有生效")
	}
	// 腾出来的 ⌘T 现在可以给别的命令
	if msg := s.bindKey("new-sql", chord{mui.Cmd, mui.KeyT}); msg != "" {
		t.Fatal(msg)
	}
	// 清空、恢复
	s.clearKey("new-sql")
	if s.chordOf("new-sql").valid() || s.cmdKeys("new-sql") != "" || s.keyHint("new-sql") != "" {
		t.Fatal("清空后不应再有快捷键")
	}
	s.resetKey("new-sql")
	if s.chordOf("new-sql") != (chord{mui.Cmd, mui.KeyE}) {
		t.Fatal("恢复默认没有生效")
	}
	// 改回默认值时不留痕迹
	if msg := s.bindKey("new-query", chord{mui.Cmd, mui.KeyT}); msg != "" {
		t.Fatal(msg)
	}
	if len(s.prefs.Keymap) != 0 {
		t.Fatalf("和默认一样就不该存: %v", s.prefs.Keymap)
	}
	// 全部恢复
	s.bindKey("export", chord{mui.Cmd | mui.Alt, mui.KeyX})
	s.bindKey("processes", chord{mui.Cmd | mui.Alt, mui.KeyY})
	s.resetKeys()
	if len(s.prefs.Keymap) != 0 || s.chordOf("export") != (chord{mui.Cmd | mui.Shift, mui.KeyE}) {
		t.Fatal("全部恢复没有生效")
	}
}

func TestRebindChangesShortcut(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if msg := e.sh.bindKey("new-query", chord{mui.Cmd | mui.Shift, mui.KeyT}); msg != "" {
		t.Fatal(msg)
	}
	e.tt.Key(mui.Cmd, mui.KeyT) // 旧键不再起作用
	e.tt.Frame()
	if len(w.tabs) != 0 {
		t.Fatalf("⌘T 不应再新建查询，tabs=%d", len(w.tabs))
	}
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyT)
	e.tt.Frame()
	if len(w.tabs) != 1 {
		t.Fatalf("⇧⌘T 应新建查询，tabs=%d", len(w.tabs))
	}
	// 编辑器让出新键，而不是自己吃掉
	q := w.tabs[w.active].(*queryTab)
	e.sh.bindKey("run", chord{mui.Cmd | mui.Alt, mui.KeyG})
	found := false
	for _, b := range q.ed.Bindings {
		if b.Mods == mui.Cmd|mui.Alt && b.Key == mui.KeyG {
			found = true
		}
	}
	if !found {
		t.Fatal("已打开的查询标签没有更新编辑器的按键让位")
	}
	if e.sh.keyHint("run") == "" {
		t.Fatal("运行按钮的提示应跟着改")
	}
}

func TestShortcutSettingsPage(t *testing.T) {
	e := newEnv(t)
	e.sh.openSettings(setShortcuts)
	e.tt.Frame()
	e.has("全部恢复默认")
	e.has("新建查询")
	e.has("固定快捷键")
	e.shot("55-keymap")
	e.sh.startRecording("new-query")
	e.sh.rec.msg = "x"
	e.tt.Frame()
	e.has("请按下新的快捷键…")
	e.shot("56-keymap-recording")
	e.sh.set.open = false
	e.tt.Frame()
	if e.sh.rec.active() {
		t.Fatal("设置关闭后应结束录制")
	}
}
