package ui

// @author ixxxxoooo
//
// 连接状态相关：工作区是否在忙、重新连接、主题切换。原来在 toolbar.go 里，标题栏重做后这些不再属于标题栏。

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// busyFor 报告 w 里有没有查询或表格翻页已经加载了超过 d：再短的加载不值得在标题栏上显示。
func (w *Workspace) busyFor(c *ui.Context, d time.Duration) bool {
	for _, tb := range w.tabs {
		switch t := tb.(type) {
		case *queryTab:
			if t.running && delayed(c, t.started, d) {
				return true
			}
		case *tableTab:
			if t.loading && delayed(c, t.loadStart, d) {
				return true
			}
		}
	}
	return false
}

// pillDetails is the tooltip of the status pill: what the connection is.
func (s *Shell) pillDetails(w *Workspace) string {
	lines := []string{w.cfg.Name, connSubtitle(w.cfg)}
	if w.version != "" {
		lines = append(lines, w.version)
	}
	lines = append(lines, locales.SafeTitle()+": "+safeModeLabel(w.cfg.EffectiveSafeMode()))
	if s.lost[w.cfg.ID] {
		lines = append(lines, locales.ConnLost(w.cfg.Name))
	}
	lines = append(lines, locales.WelcomeTitle()+s.keyHint("open-conn"))
	return strings.Join(lines, "\n")
}

// reconnect 断开再重连 w 的连接。过程中胶囊里的按钮转圈，结束后弹提示告诉用户成功还是失败：
// 之前它静悄悄地跑完，用户点了看不到任何变化，就像没点中一样。
func (s *Shell) reconnect(w *Workspace) {
	id := w.cfg.ID
	if s.reconnecting[id] {
		return
	}
	s.reconnects++
	s.reconnecting[id] = true
	var connErr error
	s.up.bg(func() {
		start := time.Now()
		_ = s.app.Connections.Disconnect(id)
		connErr = s.app.Connections.Connect(id)
		// 本地库一瞬间就连好了，转圈至少停留一会儿，否则看起来仍然没反应
		if d := 700*time.Millisecond - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}, func() {
		delete(s.reconnecting, id)
		if connErr != nil {
			s.lost[id] = true
			s.toastErr(errMessage(connErr))
			return
		}
		delete(s.lost, id)
		s.toastOK(locales.ConnRestored(w.cfg.Name))
		w.loadDatabases()
	})
}

func (s *Shell) toggleTheme(dark bool) {
	name := "dark"
	if dark {
		name = "light"
	}
	applyTheme(name)
	s.prefs.Theme = name
	s.savePrefs()
}
