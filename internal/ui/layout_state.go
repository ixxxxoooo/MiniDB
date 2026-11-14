package ui

// @author ixxxxoooo

// 关闭前是什么状态，重新打开就是什么状态：窗口大小靠窗口的 StateKey 记，工作区的标签、侧栏分段和折叠记在
// 各自的会话里（session.go），这里记的是和连接无关的整体布局——侧栏和 AI 面板拖出来的宽度、
// 侧栏是否收起、日志面板是否打开及高度、行预览栏的宽度。

// layoutKey 是布局在 settings 桶里的键。
const layoutKey = "ui_layout"

// layoutState 是全局布局。零值的尺寸表示「没记过」，用默认值。
type layoutState struct {
	SidebarW      float32 `json:"sidebarW,omitempty"`
	SidebarHidden bool    `json:"sidebarHidden,omitempty"`
	PanelW        float32 `json:"panelW,omitempty"` // AI 面板
	PanelPct      float32 `json:"panelPct,omitempty"`
	PreviewW      float32 `json:"previewW,omitempty"` // 行预览栏
	ConsoleOpen   bool    `json:"consoleOpen,omitempty"`
	ConsolePct    float32 `json:"consolePct,omitempty"`
}

// clampWidth 把存档里的宽度限制在拖动允许的范围内，坏数据或换了小屏幕时不至于把界面撑坏。
func clampWidth(v, lo, hi, def float32) float32 {
	if v <= 0 {
		return def
	}
	return min(max(v, lo), hi)
}

// applyTo 把记住的布局用到一个工作区上（新开或恢复的工作区）。
func (l layoutState) applyTo(w *Workspace) {
	w.sidebarW = clampWidth(l.SidebarW, 160, 520, 260)
	w.panelW = clampWidth(l.PanelW, 280, 720, 340)
	if l.PanelPct > 0 {
		w.panelPct = l.PanelPct
	}
	w.sidebarHidden = l.SidebarHidden
}

// loadLayout 读出上次的布局，并用到全局的部分（日志面板）。
func (s *Shell) loadLayout() {
	var l layoutState
	_ = s.app.Store.Get("settings", layoutKey, &l)
	s.layout = l
	s.consoleOpen = l.ConsoleOpen
	if l.ConsolePct > 0 {
		s.consolePct = min(max(l.ConsolePct, 20), 90)
	}
}

// saveLayout 记下当前的布局：以前台工作区的侧栏、AI 面板为准（没有工作区时保留原来的）。
func (s *Shell) saveLayout() {
	l := s.layout
	if w := s.activeWorkspace(); w != nil {
		l.SidebarW, l.SidebarHidden, l.PanelW, l.PanelPct = w.sidebarW, w.sidebarHidden, w.panelW, w.panelPct
		for _, tb := range w.tabs {
			if t, ok := tb.(*tableTab); ok && t.previewW > 0 {
				l.PreviewW = t.previewW
			}
		}
	}
	l.ConsoleOpen, l.ConsolePct = s.consoleOpen, s.consolePct
	s.layout = l
	_ = s.app.Store.Put("settings", layoutKey, l)
}
