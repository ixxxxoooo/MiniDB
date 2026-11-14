package ui

import (
	"testing"
)

// @author ixxxxoooo
//
// 命令面板的分类芯片、「>」命令模式、问 AI 行和命中加粗。

func TestPaletteCategoryChipsFilterByKind(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.sh.openPalette()
	e.sh.pal.query = "ord"
	e.has("全部")
	e.has("命令")
	all := e.sh.paletteItems()
	e.sh.pal.cat = 1 // 表
	tables := e.sh.paletteItems()
	if len(tables) == 0 || len(tables) > len(all) {
		t.Fatalf("表分类应只剩表: all=%d tables=%d", len(all), len(tables))
	}
	for _, it := range tables {
		if it.kind != "表" {
			t.Fatalf("表分类里混进了 %q", it.kind)
		}
	}
	e.click("命令") // 点芯片切换分类
	if e.sh.pal.cat != 2 {
		t.Fatalf("点「命令」芯片应切到命令分类: cat=%d", e.sh.pal.cat)
	}
}

func TestPaletteGreaterThanShowsOnlyCommands(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.sh.openPalette()
	e.sh.pal.query = ">"
	items := e.sh.paletteItems()
	if len(items) == 0 {
		t.Fatal("> 应列出命令")
	}
	for _, it := range items {
		if it.kind != "命令" {
			t.Fatalf("> 模式里出现了 %q: %q", it.kind, it.label)
		}
	}
	e.sh.pal.query = ">刷新"
	for _, it := range e.sh.paletteItems() {
		if it.kind != "命令" {
			t.Fatalf("> 加查询词仍只看命令: %q", it.kind)
		}
	}
}

func TestPaletteAskAIRowSendsTheQuery(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.openPalette()
	e.sh.pal.query = "zzz-no-such"
	items := e.sh.paletteAskAI(e.sh.paletteItems())
	if len(items) != 1 || items[0].kind != paletteAIKind {
		t.Fatalf("没有别的匹配时只剩问 AI 一行: %+v", items)
	}
	e.has("问 AI：")
	e.shot("p6-palette-ask-ai")
	e.sh.pal.query = ">run"
	for _, it := range e.sh.paletteAskAI(e.sh.paletteItems()) {
		if it.kind == paletteAIKind {
			t.Fatal("命令模式不该有问 AI 行")
		}
	}
	e.sh.pal.query = "zzz-no-such"
	items[0].run()
	if w.panel != 2 {
		t.Fatalf("问 AI 应打开 AI 面板: panel=%d", w.panel)
	}
}

func TestSplitMatch(t *testing.T) {
	cases := []struct{ label, q, pre, hit, post string }{
		{"orders", "ord", "", "ord", "ers"},
		{"v_order_summary", "ORD", "v_", "ord", "er_summary"},
		{"orders", "^ord", "", "ord", "ers"},
		{"orders", "zzz", "orders", "", ""},
		{"orders", "", "orders", "", ""},
	}
	for _, c := range cases {
		pre, hit, post := splitMatch(c.label, c.q)
		if pre != c.pre || hit != c.hit || post != c.post {
			t.Errorf("splitMatch(%q,%q) = %q|%q|%q", c.label, c.q, pre, hit, post)
		}
	}
}
