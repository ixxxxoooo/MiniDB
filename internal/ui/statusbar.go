package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 底部状态栏：26px，在内容区的最下面。左边是连接状态（状态点 + 文字，掉线时带重连按钮）和事务条，
// 右边是索引芯片、后台任务、更新提示和待提交条。原来这些都挤在标题栏里。
// 每个标签自己的信息（行数、分页）以后也放在这一条上。

// statusBar 画状态栏。
func (w *Workspace) statusBar(c *ui.Context, p tokens) {
	s := w.sh
	hairline(c, p.border)
	ui.Row(c.Key("statusbar")).FillWidth().Height(sizeStatus-1).Shrink(0).AlignItems(ui.Center).Gap(spaceSM+2).Padding(0, spaceSM+2).Background(p.toolbar).Children(func() {
		s.connStatus(c, p, w)
		s.txGroup(c, p, w)
		ui.Spacer(c)
		s.indexChip(c, p, w)
		s.taskBar(c)
		s.updateChip(c, p)
		s.pendingGroup(c, p)
	})
}

// connStatus 是状态栏最左边的连接状态：正常是绿点「已连接」，连接中是橙点，掉线是红点并给一个重连按钮。
// 查询或翻页超过 0.4 秒才转圈：一次排序、翻页只要几毫秒，状态点一闪变成转圈再变回来，状态栏就像在闪。
func (s *Shell) connStatus(c *ui.Context, p tokens, w *Workspace) {
	lost := s.lost[w.cfg.ID]
	dot, text := p.success, locales.StatusConnected()
	switch {
	case lost:
		dot, text = p.danger, locales.StatusLost()
	case len(s.connecting) > 0:
		dot, text = p.warning, locales.StatusConnecting()
	}
	ui.Row(c.Key("conn-state")).Gap(spaceSM - 2).AlignItems(ui.Center).Shrink(0).Children(func() {
		if w.busyFor(c, 400*time.Millisecond) {
			ui.Spinner(c).Size(btnActIcon-3, btnActIcon-3)
		} else {
			ui.Box(c).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(dot)
		}
		ui.Text(c, text).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine()
	})
	if !lost {
		return
	}
	rb := tipOn(c, ui.Button(c.Key("reconnect"), "").Label(locales.ToolbarReconnect()), locales.ToolbarReconnect()).
		Size(sizeBtnSm, sizeBtnSm).Padding(0).Radius(radiusSm).BorderWidth(0).Background(ui.Color{}).Shrink(0)
	if rb.Hovered() {
		rb.Background(p.hover)
	}
	if rb.Children(func() {
		if s.reconnecting[w.cfg.ID] {
			ui.Spinner(c).Size(btnActIcon-3, btnActIcon-3)
		} else {
			ui.Icon(c, icRefresh).FontSize(btnActIcon).TextColor(p.danger)
		}
	}).Clicked() {
		s.reconnect(w)
	}
}
