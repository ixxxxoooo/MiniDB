package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 基础控件对照设计稿「组件规范」：按钮三档高度、芯片高度、分段开关与下划线标签的点击，
// 以及一张把所有控件摆在一起的画廊截图（MINIDB_SHOTS=<目录> 时输出，浅色、深色各一张）。

// ctlGallery 是一个只摆控件的窗口，用来量尺寸、点点看、截图。
type ctlGallery struct {
	e                               *env
	sel, tab                        int
	changed                         bool
	clicked                         int
	hAct, hField, hFoot, hChip, hSq float32
	hSoft, hGhost, hDang            float32
}

// view 画画廊：和设计稿里「按钮 · 分段开关 · 下划线标签 · 芯片」几块对应。
func (g *ctlGallery) view(c *mui.Context) {
	p := g.e.sh.applyLook(c)
	mui.Column(c).Fill().Background(p.surface).Padding(spaceXL).Gap(spaceLG).Children(func() {
		mui.Row(c.Key("btns")).Gap(spaceSM).AlignItems(mui.Center).Children(func() {
			g.hAct = actPrimary(c.Key("a"), icPlay, "执行").Bounds().H
			g.hField = fieldBtn(c.Key("f"), icRefresh, "取消").Bounds().H
			g.hFoot = footPrimary(c.Key("o"), "确认").Bounds().H
			g.hSoft = softBtn(c.Key("s"), icSparkle, "AI 优化").Bounds().H
			g.hGhost = ghostBtn(c.Key("g"), nil, "幽灵").Bounds().H
			g.hDang = dangerSoftBtn(c.Key("d"), icTrash, "删除").Bounds().H
			dangerBtn(c.Key("dd"), nil, "确认删除")
		})
		mui.Row(c.Key("seg")).Gap(spaceMD).AlignItems(mui.Center).Children(func() {
			if segmented(c, &g.sel, "数据", "结构", "DDL", "文档") {
				g.changed = true
			}
			mui.Column(c.Key("ulc")).Width(260).Children(func() {
				if i := ulTabs(c, p, "ul", []string{"索引", "外键", "触发器"}, []int{2, 1, 0}, g.tab); i >= 0 {
					g.tab, g.clicked = i, i
				}
			})
		})
		mui.Row(c.Key("chips")).Gap(6).AlignItems(mui.Center).Children(func() {
			g.hChip = chip(c.Key("c1"), chipOK, "已连接", true).Bounds().H
			chip(c.Key("c2"), chipWarn, "待提交 3", false)
			chip(c.Key("c3"), chipErr, "生产环境", false)
			chip(c.Key("c4"), chipInfo, "只读", false)
			chip(c.Key("c5"), chipAccent, "已固定", false)
			chip(c.Key("c6"), chipNeutral, "InnoDB", false)
			g.hSq = sqChip(c.Key("c7"), chipWarn, "PROD").Bounds().H
			sqChip(c.Key("c8"), chipOK, "DEV")
			kbd(c.Key("k1"), "⌘")
			kbd(c.Key("k2"), "⏎")
		})
	})
}

func newGallery(t *testing.T, dark bool) (*ctlGallery, *mui.Tester) {
	g := &ctlGallery{e: newEnv(t), clicked: -1}
	tt := mui.NewTester(g.view, 720, 220)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return g, tt
}

// 三档按钮的高度是 22 / 26 / 30，芯片 20，方角芯片 18：改了令牌，这里先红。
func TestControlHeights(t *testing.T) {
	g, _ := newGallery(t, false)
	for name, c := range map[string]struct{ got, want float32 }{
		"actBtn":        {g.hAct, sizeBtnSm},
		"fieldBtn":      {g.hField, sizeBtn},
		"footBtn":       {g.hFoot, sizeBtnLg},
		"softBtn":       {g.hSoft, sizeBtnSm},
		"ghostBtn":      {g.hGhost, sizeBtnSm},
		"dangerSoftBtn": {g.hDang, sizeBtnSm},
		"chip":          {g.hChip, sizeChip},
		"sqChip":        {g.hSq, sizeChipSq},
	} {
		if c.got != c.want {
			t.Errorf("%s 高度 = %v，设计稿要求 %v", name, c.got, c.want)
		}
	}
}

// 点另一段会改写选中项并报告变化；点已选中的段不算变化。
func TestSegmentedChangesSelection(t *testing.T) {
	g, tt := newGallery(t, false)
	if err := tt.Click("结构"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.sel != 1 || !g.changed {
		t.Fatalf("点「结构」后 sel=%d changed=%v，期望 1 / true", g.sel, g.changed)
	}
	g.changed = false
	if err := tt.Click("结构"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.changed {
		t.Fatal("点已选中的段不应报告变化")
	}
}

// 下划线标签返回被点中的下标，计数跟在标签后面。
func TestUnderlineTabsClick(t *testing.T) {
	g, tt := newGallery(t, false)
	if !tt.HasText("2") || !tt.HasText("1") {
		t.Fatalf("标签后应显示计数，屏幕上有 %q", tt.Texts())
	}
	if err := tt.Click("外键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.clicked != 1 || g.tab != 1 {
		t.Fatalf("点「外键」后 clicked=%d tab=%d，期望 1", g.clicked, g.tab)
	}
}

// 画廊截图：浅色、深色各一张，看三套控件的实际观感。
func TestShotsControls(t *testing.T) {
	for _, dark := range []bool{false, true} {
		g, tt := newGallery(t, dark)
		name := "ctl-light"
		if dark {
			name = "ctl-dark"
		}
		g.e.tt = tt
		g.e.shot(name)
	}
}
