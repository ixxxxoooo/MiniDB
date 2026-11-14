package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

func hairline(c *ui.Context, col ui.Color) ui.Element {
	return ui.Box(c).Height(1).Background(col).FillWidth()
}
func vline(c *ui.Context, col ui.Color, h float32) ui.Element {
	return ui.Box(c).Width(1).Height(h).Background(col).Margin(0, 2)
}

// tip returns a flat icon button of the toolbar (no face until hovered).
// Its icon is the secondary grey, darker under the pointer, as TablePlus's are.
func tipBtn(c *ui.Context, ic *ui.SVG, tip string) ui.Element {
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtn, sizeBtn).Padding(0).BorderWidth(0).Background(ui.Color{})
	p := baseTokens(c)
	col := p.fgSecondary
	if b.Hovered() {
		col = p.fg
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(15).TextColor(col) })
}

// toggleBtn is tipBtn for a switch: its icon takes the accent while it is on.
func toggleBtn(c *ui.Context, ic *ui.SVG, tip string, on bool) ui.Element {
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtnSm, sizeBtnSm).Padding(0).BorderWidth(0).Background(ui.Color{})
	p := baseTokens(c)
	col := p.fgSecondary
	switch {
	case on:
		col = p.accent
		b.Background(p.accent.Alpha(0.10))
	case b.Hovered():
		col = p.fg
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(14).TextColor(col) })
}

func (s *Shell) view(c *ui.Context) {
	s.up.drain() // 同步模式下，先执行后台协程排队的 UI 任务
	p := s.applyLook(c)
	if s.probe != nil {
		s.probe(c)
	}
	s.flushToasts(c)
	if s.clip != "" {
		c.Services().WriteClipboard(s.clip)
		s.clip = ""
	}

	ui.Column(c).Fill().Background(p.surface).Children(func() {
		s.envBand(c, p)
		perfBuild("ui.build.titlebar", func() { s.titleBar(c, p) })
		body := func() {
			ui.Row(c).Fill().MinWidth(0).Children(func() {
				perfBuild("ui.build.rail", func() { s.activityRail(c, p) })
				if w := s.activeWorkspace(); w != nil {
					perfBuild("ui.build.workspace", func() { w.view(c, p) })
				} else {
					s.emptyWorkspace(c, p)
				}
			})
		}
		if !s.consoleOpen {
			ui.Column(c.Key("body")).Grow(1).FillWidth().MinWidth(0).Children(body)
			return
		}
		if s.consolePct <= 0 {
			s.consolePct = 74
		}
		// 分隔条可以拖：日志的高度随窗口按比例伸缩
		ui.SplitVerticalPercent(c, &s.consolePct, body, func() { perfBuild("ui.build.console", func() { s.consolePanel(c, p) }) }).Grow(1).FillWidth()
	})
	s.shortcuts(c)
	perfBuild("ui.build.modals", func() { s.drawModals(c) })
	s.toastViewport(c, p) // 最后建，盖在对话框之上
}

// emptyWorkspace is what shows without a connection.
func (s *Shell) emptyWorkspace(c *ui.Context, p tokens) {
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(260).FillHeight().Background(p.toolbar).Center().Gap(6).Padding(16).Children(func() {
			logo(c, 56)
			ui.Text(c, locales.SidebarNotConnected()).FontSize(font2XS + 1).FontWeight(500).TextColor(p.fgSecondary)
			ui.Text(c, locales.SidebarConnectHint()).FontSize(font2XS).TextColor(p.fgMuted).TextAlign(ui.Center)
		})
		ui.Box(c).Width(1).FillHeight().Background(p.border)
		ui.Column(c).Grow(1).FillHeight()
	})
}

// ---- workspace ----

// main 是内容区：上面是当前标签，最下面是 26px 的状态栏。
func (w *Workspace) main(c *ui.Context, p tokens) {
	ui.Column(c).Fill().MinWidth(0).Background(p.surface).Children(func() {
		ui.Column(c.Key("tab-area")).Grow(1).FillWidth().MinWidth(0).Children(func() { w.tabArea(c, p) })
		perfBuild("ui.build.status", func() { w.statusBar(c, p) })
	})
}

// tabArea 是当前标签的内容；一个标签都没有时是「新建查询」的空态。
func (w *Workspace) tabArea(c *ui.Context, p tokens) {
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		if len(w.tabs) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				ui.Icon(c, icTable).FontSize(30).TextColor(p.fgMuted)
				ui.Text(c, locales.TabNone()).FontSize(font2XS + 1).TextColor(p.fgMuted)
				if actPrimary(c.Key("empty-new-query"), icPlus, locales.MenuNewQuery()+w.sh.keyHint("new-query")).Clicked() {
					w.newQuery("")
				}
			})
			return
		}
		if w.active >= len(w.tabs) {
			w.active = len(w.tabs) - 1
		}
		ui.Column(c.Key(fmt.Sprintf("tab%d", w.active))).Grow(1).FillWidth().MinWidth(0).Children(func() { perfBuild(tabPerfName(w.tabs[w.active]), func() { w.tabs[w.active].view(c, p) }) })
	})
}
