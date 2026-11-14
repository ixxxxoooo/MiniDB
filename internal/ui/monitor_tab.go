package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 监控页（设计稿 A「监控」）：监控不再是弹窗，而是工作区里的一个标签。
// 顶部条左边是「概览 / 进程 / 慢查询」下划线标签，右边是采样间隔分段和「实时」标记（进程、慢查询页是刷新按钮）；
// 下面是当前页签的内容，进程和慢查询页底部有一条操作栏。
// 采样与进程列表的状态仍在 s.proc 里，标签关掉（或换了连接）后由 monitorSweep 停掉采样。

// monitorTab 是监控标签；状态都在 Shell 的 s.proc 里，标签本身只记属于哪个工作区。
type monitorTab struct{ w *Workspace }

func (m *monitorTab) title() string       { return locales.MonitorTitle() }
func (m *monitorTab) icon() *ui.SVG       { return icActivity }
func (m *monitorTab) objKind() ObjectKind { return KindTable }
func (m *monitorTab) pending() int        { return 0 }
func (m *monitorTab) view(c *ui.Context, p tokens) {
	m.w.sh.monitorPage(c, p)
}

// showMonitorTab 把监控标签切到前台；还没有就新开一个。
func (w *Workspace) showMonitorTab() {
	for i, t := range w.tabs {
		if _, ok := t.(*monitorTab); ok {
			w.active = i
			return
		}
	}
	w.tabs = append(w.tabs, &monitorTab{w: w})
	w.active = len(w.tabs) - 1
}

// hasMonitorTab 报告工作区里有没有监控标签。
func (w *Workspace) hasMonitorTab() bool {
	for _, t := range w.tabs {
		if _, ok := t.(*monitorTab); ok {
			return true
		}
	}
	return false
}

// monitorSweep 每帧调用一次：监控标签关掉了，就把采样也停掉。
func (s *Shell) monitorSweep(c *ui.Context) {
	d := &s.proc
	if d.open && (d.w == nil || !d.w.hasMonitorTab()) {
		d.open = false
	}
	if !d.open {
		s.stopMonitor()
	}
}

// monitorPage 画监控页：顶部条、内容、（进程 / 慢查询的）底部操作栏。
func (s *Shell) monitorPage(c *ui.Context, p tokens) {
	d := &s.proc
	if !d.open || d.w == nil {
		return
	}
	m := &d.mon
	tabs := monitorTabs(d.w)
	ui.Toolbar(c, func() {
		ui.Row(c.Key("mon-bar")).Gap(spaceSM).Padding(0, spaceSM+2, 0, spaceSM).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Background(p.surface).Children(func() {
			labels, idx := make([]string, len(tabs)), 0
			for i, k := range tabs {
				labels[i] = monitorTabLabel(k)
				if k == m.kind {
					idx = i
				}
			}
			if i := ulStrip(c, p, "mon-tabs", labels, nil, idx, sizeBar, false); i >= 0 && i != idx {
				s.setMonitorTab(tabs[i])
			}
			ui.Spacer(c)
			if d.loading || m.slow.loading {
				ui.Spinner(c)
			}
			s.monitorBarRight(c, p)
		})
	})
	hairline(c, p.border)
	body := ui.Column(c.Key("mon-body")).Grow(1).FillWidth().MinHeight(0).Gap(spaceSM)
	if m.kind != monOverview {
		body.Padding(spaceMD, spaceLG)
	} else {
		body.Padding(spaceLG)
	}
	body.Children(func() {
		switch m.kind {
		case monOverview:
			s.monitorOverview(c, p)
		case monSlow:
			s.slowBody(c, p)
		default:
			s.processBody(c, p)
		}
	})
	if foot := s.monitorFoot(c); foot != nil {
		hairline(c, p.border)
		ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Justify(ui.End).Padding(spaceSM+2, spaceMD+2).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(foot)
	}
}

// monitorBarRight 是顶部条右边：概览页是「采样」间隔分段和「实时」标记，其它页是刷新按钮。
func (s *Shell) monitorBarRight(c *ui.Context, p tokens) {
	m := &s.proc.mon
	if m.kind == monOverview && m.sess != nil {
		labels := make([]string, len(app.MonitorIntervals))
		for i, iv := range app.MonitorIntervals {
			labels[i] = strings.TrimSpace(locales.MonitorEveryShort(int(iv.Seconds())))
		}
		ui.Text(c, locales.MonitorSample()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		sel := m.interval
		if segmented(c.Key("mon-iv"), &sel, labels...) {
			m.interval = sel
			m.sess.SetInterval(app.MonitorIntervals[sel])
		}
		chip(c, chipOK, locales.MonitorLive(), true)
		return
	}
	if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
		s.refreshMonitorTab()
	}
}
