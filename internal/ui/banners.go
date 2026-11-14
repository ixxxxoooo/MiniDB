package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 内容区顶部的两条提示：连接断开（红）和手动提交的事务进行中（橙）。设计稿「危险操作 · 安全模式 · 反馈」页。
// 它们原来是状态栏里的小图标，用户容易看漏；现在是整条提示，按钮就在条上。

// banners 画内容区顶部的提示条；没有要提示的就什么都不占。
func (w *Workspace) banners(c *ui.Context, p tokens) {
	if w.sh.lost[w.cfg.ID] {
		w.lostBar(c, p)
	}
	if st := w.txStatus(); w.txSupported() && st.Manual && st.Active {
		w.txBar(c, p, max(st.Pending, 1))
	}
}

// lostBar 是连接断开的红色整条提示：状态、重新连接按钮（重连中显示转圈）。
func (w *Workspace) lostBar(c *ui.Context, p tokens) {
	s := w.sh
	ui.Row(c.Key("lost-bar")).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(spaceSM).Padding(spaceSM, spaceMD).Background(p.dangerBg).Children(func() {
		ui.Icon(c, icUnplug).FontSize(btnActIcon).TextColor(p.danger)
		ui.Text(c, locales.StatusLost()).FontSize(fontCtl).FontWeight(600).TextColor(p.danger).SingleLine()
		ui.Text(c, locales.LostDetail(w.cfg.Name)).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine().Shrink(1)
		ui.Spacer(c)
		busy := s.reconnecting[w.cfg.ID]
		if fieldBtn(c.Key("reconnect"), icRefresh, locales.ToolbarReconnect()).Disabled(busy).Clicked() {
			s.reconnect(w)
		}
	})
	hairline(c, p.danger)
}

// txBar 是手动提交的橙色整条提示：「事务进行中 · N 条更改未提交」，右边是回滚和提交。
func (w *Workspace) txBar(c *ui.Context, p tokens, pending int) {
	s := w.sh
	ui.Row(c.Key("tx-bar")).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(spaceSM).Padding(spaceSM, spaceMD).Background(p.warningBg).Children(func() {
		ui.Box(c).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(p.warning).Shrink(0)
		ui.Text(c, locales.TxRunning()).FontSize(fontCtl).FontWeight(600).TextColor(p.warning).SingleLine()
		ui.Text(c, locales.TxPending(pending)).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine().Shrink(1)
		ui.Spacer(c)
		idle := !w.tx.busy
		if fieldBtn(c.Key("tx-rollback"), icUndo, locales.TxRollbackShort()+" "+s.cmdKeys("tx-rollback")).Disabled(!idle).Clicked() {
			s.finishTx(w, false)
		}
		ok := skinBtn(fieldBtn(c.Key("tx-commit"), icCheck, locales.TxCommitShort()+" "+s.cmdKeys("tx-commit")).Disabled(!idle), p, btnOK)
		if ok.Clicked() {
			s.finishTx(w, true)
		}
	})
	hairline(c, p.warning)
}
