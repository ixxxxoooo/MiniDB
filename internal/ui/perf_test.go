package ui

import "testing"

type perfProbe struct{}

func (p *perfProbe) here() string  { return perfSite(1) }
func (p *perfProbe) outer() string { return p.wrapped() }

// wrapped 模拟 loader 这样的封装：它替调用者取名，所以要往上数两层。
func (p *perfProbe) wrapped() string { return perfSite(2) }

type perfGeneric[T any] struct{}

func (g *perfGeneric[T]) load() string { return perfSite(2) }

func perfInitiator() string { return (&perfGeneric[int]{}).load() }

// 打点的名字要指向发起者：日志里 bg.<函数> 看到的是谁发起的，而不是中间的封装。
func TestPerfSiteNamesTheInitiator(t *testing.T) {
	if got := (&perfProbe{}).here(); got != "perfProbe.here" {
		t.Errorf("直接调用: %q", got)
	}
	if got := (&perfProbe{}).outer(); got != "perfProbe.outer" {
		t.Errorf("封装里往上数两层应是发起者 outer，实际 %q", got)
	}
	if got := perfInitiator(); got != "perfInitiator" {
		t.Errorf("泛型封装里应取到真正的发起者，不是 loader[...]：实际 %q", got)
	}
}

func TestTabPerfNameCoversEveryTabKind(t *testing.T) {
	cases := map[string]any{
		"ui.build.tab.table":      &tableTab{},
		"ui.build.tab.query":      &queryTab{},
		"ui.build.tab.overview":   &overviewTab{},
		"ui.build.tab.definition": &defTab{},
		"ui.build.tab.er":         &erTab{},
		"ui.build.tab.other":      struct{}{},
	}
	for want, tab := range cases {
		if got := tabPerfName(tab); got != want {
			t.Errorf("%T → %q，期望 %q", tab, got, want)
		}
	}
}
