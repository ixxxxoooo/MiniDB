package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 底部状态栏：24px，在内容区的最下面。左边是当前标签的信息（行范围、耗时、翻页），
// 右边依次是当前标签的操作（查询页：查找、固定、导出结果）、后台任务、事务、连接状态（环境名；掉线时带重连按钮）、表数量、更新提示。

// statusBar 画状态栏。
func (w *Workspace) statusBar(c *ui.Context, p tokens) {
	s := w.sh
	hairline(c, p.border)
	ui.Row(c.Key("statusbar")).FillWidth().Height(sizeStatus-1).Shrink(0).AlignItems(ui.Center).Gap(spaceMD).Padding(0, spaceSM+2).Background(p.toolbar).Children(func() {
		w.tabStatus(c, p) // 设计稿 A：左边是当前标签的行范围与翻页，右边依次是事务、连接状态、表数量
		ui.Spacer(c)
		w.tabStatusRight(c, p) // 查询页：在当前页中查找、固定、导出结果
		s.taskBar(c)
		s.txGroup(c, p, w)
		s.connStatus(c, p, w)
		s.indexChip(c, p, w)
		s.updateChip(c, p)
	})
}

// connStatus 是状态栏最左边的连接状态：正常是绿点「已连接」，连接中是橙点，掉线是红点并给一个重连按钮。
// 查询或翻页超过 0.4 秒才转圈：一次排序、翻页只要几毫秒，状态点一闪变成转圈再变回来，状态栏就像在闪。
func (s *Shell) connStatus(c *ui.Context, p tokens, w *Workspace) {
	lost := s.lost[w.cfg.ID]
	dot, text := p.success, envLabel(w.cfg.Tag)
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
		ui.Text(c, text).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	})
}

// envLabel 是状态栏连接状态旁的环境名（DEV、PROD…），没有环境标记的连接显示「已连接」。
func envLabel(tag string) string {
	if tag == "" {
		return locales.StatusConnected()
	}
	return tagShort(tag)
}
