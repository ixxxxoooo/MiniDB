package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// 监控窗口：概览（实时指标与折线）、进程、慢查询三个页签。
// 窗口是否打开由 s.proc.open 决定（进程列表的状态也在 s.proc 里）；
// 概览的后台采样只在窗口打开期间运行，关窗即停。

const (
	monProcess  = iota // 零值：没有概览的连接（TiDB、StarRocks、SQLite）只有这一页
	monOverview        // 指标与折线
	monSlow            // 慢查询
)

// monitorPanel 是监控窗口自己的状态。
type monitorPanel struct {
	kind     int
	interval int // MonitorIntervals 的下标
	sess     *app.MonitorSession
	slow     slowState

	chartAt time.Time // 缓存的折线数据对应的最后一个点的时间，有新点才重算
	charts  map[string]chartData
}

// monitorTabs 列出连接有哪些页签。
func monitorTabs(w *Workspace) []int {
	var tabs []int
	monitor := app.SupportsMonitor(w.cfg.Type)
	if monitor {
		tabs = append(tabs, monOverview)
	}
	if app.SupportsProcessList(w.cfg.Type) {
		tabs = append(tabs, monProcess)
	}
	if monitor {
		tabs = append(tabs, monSlow)
	}
	return tabs
}

func monitorTabLabel(kind int) string {
	switch kind {
	case monOverview:
		return locales.MonitorTabOverview()
	case monSlow:
		return locales.MonitorTabSlow()
	}
	return locales.MonitorTabProcess()
}

// openMonitor 打开监控窗口并停在 kind 页签（连接没有这一页时停在第一页）。
func (s *Shell) openMonitor(w *Workspace, kind int) {
	s.stopMonitor()
	s.proc = processDialog{open: true, w: w, sel: map[string]bool{}}
	s.proc.mon = monitorPanel{interval: 1}
	tabs := monitorTabs(w)
	s.proc.mon.kind = kind
	if len(tabs) > 0 && !containsInt(tabs, kind) {
		s.proc.mon.kind = tabs[0]
	}
	switch {
	case app.SupportsProcessList(w.cfg.Type):
		s.reloadProcesses()
	case len(tabs) == 0:
		s.proc.err = locales.ProcessUnsupported()
	}
	if app.SupportsMonitor(w.cfg.Type) {
		s.startMonitorSampling()
	}
	if s.proc.mon.kind == monSlow {
		s.loadSlow()
	}
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// startMonitorSampling 开始后台采样；有新点时只通知重绘，不碰任何界面对象。
func (s *Shell) startMonitorSampling() {
	w := s.proc.w
	sess, err := s.app.Monitor.Start(w.cfg.ID, app.MonitorIntervals[s.proc.mon.interval], s.redraw)
	if err != nil {
		s.toastErr(errMessage(err))
		return
	}
	s.proc.mon.sess = sess
}

// stopMonitor 停掉采样。窗口关闭、换连接或重新打开时调用，可以重复调用。
func (s *Shell) stopMonitor() {
	if s.proc.mon.sess != nil {
		s.proc.mon.sess.Stop()
		s.proc.mon.sess = nil
	}
}

// setMonitorTab 切换页签，并按需加载数据。
func (s *Shell) setMonitorTab(kind int) {
	if s.proc.mon.kind == kind {
		return
	}
	s.proc.mon.kind = kind
	switch kind {
	case monProcess:
		s.reloadProcesses()
	case monSlow:
		if !s.proc.mon.slow.loaded {
			s.loadSlow()
		}
	}
}

// refreshMonitorTab 是标题栏刷新按钮：进程和慢查询重新读取，概览本来就在实时刷新。
func (s *Shell) refreshMonitorTab() {
	switch s.proc.mon.kind {
	case monProcess:
		s.reloadProcesses()
	case monSlow:
		s.loadSlow()
	}
}

func (s *Shell) cmdMonitor() {
	if w := s.activeWorkspace(); w != nil {
		s.openMonitor(w, monOverview)
		s.redraw()
	}
}

func (s *Shell) monitorView(c *ui.Context) {
	d := &s.proc
	if !d.open {
		s.stopMonitor() // 窗口被 Esc、点击空白或关闭按钮关掉后，在这里收尾
		return
	}
	p := s.tokens(c)
	m := &s.proc.mon
	tabs := monitorTabs(d.w)
	s.dialog(c, &d.open, dlgOpts{title: locales.MonitorTitle(), sub: d.w.cfg.Name, width: 980, height: 600, extra: func() {
		if d.loading || m.slow.loading {
			ui.Spinner(c)
		}
		if m.kind == monOverview && m.sess != nil {
			labels := make([]string, len(app.MonitorIntervals))
			for i, iv := range app.MonitorIntervals {
				labels[i] = locales.MonitorEvery(int(iv.Seconds()))
			}
			cur := labels[m.interval]
			if ui.Select(c.Key("mon-iv"), &cur, labels).MinWidth(0).Width(120).Height(sizeBtnSm).Changed() {
				for i, l := range labels {
					if l == cur {
						m.interval = i
						m.sess.SetInterval(app.MonitorIntervals[i])
					}
				}
			}
		} else if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			s.refreshMonitorTab()
		}
	}}, func() {
		if len(tabs) > 1 {
			labels := make([]string, len(tabs))
			idx := 0
			for i, k := range tabs {
				labels[i] = monitorTabLabel(k)
				if k == m.kind {
					idx = i
				}
			}
			ui.Row(c).FillWidth().Shrink(0).Children(func() {
				if ui.Segmented(c.Key("mon-tabs"), &idx, labels...).Changed() {
					s.setMonitorTab(tabs[idx])
				}
			})
		}
		switch m.kind {
		case monOverview:
			s.monitorOverview(c, p)
		case monSlow:
			s.slowBody(c, p)
		default:
			s.processBody(c, p)
		}
	}, s.monitorFoot(c))
}

// monitorFoot 返回当前页签的底部按钮；概览没有底栏。
func (s *Shell) monitorFoot(c *ui.Context) func() {
	switch s.proc.mon.kind {
	case monProcess:
		return func() { s.processFoot(c) }
	case monSlow:
		return func() { s.slowFoot(c) }
	}
	return nil
}
