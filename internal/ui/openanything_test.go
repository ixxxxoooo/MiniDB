package ui

import (
	"encoding/json"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/database"
)

func TestPaletteMatchAnchors(t *testing.T) {
	cases := []struct {
		label, q string
		want     bool
	}{
		{"user_orders", "order", true},
		{"user_orders", "^order", false},
		{"user_orders", "^user", true},
		{"user_orders", "orders$", true},
		{"user_orders", "user$", false},
		{"orders", "^orders$", true},
		{"user_orders", "^orders$", false},
		{"Orders", "^ORD", true},
		{"anything", "", true},
	}
	for _, c := range cases {
		if got := paletteMatch(c.label, c.q); got != c.want {
			t.Errorf("paletteMatch(%q, %q) = %v, want %v", c.label, c.q, got, c.want)
		}
	}
}

// Tab 把输入补全成选中项；⌘↵ 在新标签里打开，而不是复用已有标签。
func TestPaletteTabCompletesAndCmdEnterOpensNewTab(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.sh.openPalette()
	e.tt.Frame()
	e.tt.Type("^use")
	e.tt.Key(0, mui.KeyTab)
	e.tt.Frame()
	if e.sh.pal.query != "users" {
		t.Fatalf("Tab 应补全为 users，得到 %q", e.sh.pal.query)
	}
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if e.sh.pal.open {
		t.Fatal("⌘↵ 应关闭面板")
	}
	if len(w.tabs) != 2 || w.tabs[1].title() != "users" {
		t.Fatalf("⌘↵ 应另开一个 users 标签: %v", tableTitles(w))
	}
}

// 面板里能找到应用功能：安全模式、控制台等。
func TestPaletteListsAppFeatures(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openPalette()
	e.sh.pal.query = "安全模式"
	items := e.sh.paletteItems()
	if len(items) != len(database.SafeModes) {
		t.Fatalf("应列出 5 个安全模式: %d", len(items))
	}
	for _, it := range items {
		if it.label == "安全模式: 提醒 1" {
			it.run()
		}
	}
	if w.cfg.EffectiveSafeMode() != database.SafeModeAlert1 {
		t.Fatalf("应切到提醒 1: %s", w.cfg.EffectiveSafeMode())
	}
}

// 右侧栏默认自动美化 JSON；关掉后显示原文。
func TestRowPaneAutoPrettyJSON(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	tt := w.tabs[0].(*tableTab)
	raw, _ := json.Marshal(map[string]any{"a": 1, "b": []int{1, 2}})
	tt.g.rows[0]["bio"] = string(raw)
	tt.g.chosen.Clear()
	tt.g.chosen.Add(0)
	tt.showPreview = true
	e.tt.Frame()
	e.tt.Frame()
	if got := tt.pv.buf["bio"]; got == string(raw) || len(got) < len(raw) {
		t.Fatalf("应显示美化后的 JSON: %q", got)
	}
	if tt.pending() != 0 {
		t.Fatal("只是显示，不应产生修改")
	}

	e.sh.prefs.RawJSON = true
	tt.pv.sig = map[string]string{}
	e.tt.Frame()
	if got := tt.pv.buf["bio"]; got != string(raw) {
		t.Fatalf("关闭后应显示原文: %q", got)
	}
}

// 状态胶囊显示环境标签。
func TestStatusPillShowsTag(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.cfg.Tag = "production"
	e.tt.Frame()
	e.has("生产")
}
