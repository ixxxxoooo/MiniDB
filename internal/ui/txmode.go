package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// 手动提交（事务模式）：开启后该库上的写操作都进同一个事务，直到点「提交」或「回滚」。
// 事务本身由 app/database 层管理，这里只负责开关、按钮、提示和离开前的确认。

var icCommit = svg(`<circle cx="12" cy="12" r="3"/><line x1="3" x2="9" y1="12" y2="12"/><line x1="15" x2="21" y1="12" y2="12"/>`)

// txState 是工作区对手动提交的界面侧记忆：只用来检测"状态被服务端悄悄改变"并提示用户。
type txState struct {
	busy      bool // 正在开关、提交或回滚
	wasManual bool // 上一帧是开着的
	lost      int  // 已提示过的"事务被中断"次数
}

// txStatus 取当前工作区所在库的手动提交状态（纯内存读取）。
func (w *Workspace) txStatus() model.TxStatus {
	return w.sh.app.Queries.TransactionStatus(w.cfg.ID, w.db)
}

// txSupported 报告该工作区能否使用手动提交。
func (w *Workspace) txSupported() bool {
	return app.CapsOf(w.cfg.Type).ManualTx && !w.cfg.ReadOnly
}

// txPendingTotal 是该工作区手动提交里还没提交的条数。
func (w *Workspace) txPendingTotal() int {
	if !w.txSupported() {
		return 0
	}
	st := w.txStatus()
	if !st.Active {
		return 0
	}
	return max(st.Pending, 1)
}

// watchTx 对比这一帧与上一帧的状态：事务被取消语句打断，或连接重建导致模式被关掉时，弹一条提示。
func (w *Workspace) watchTx(st model.TxStatus) {
	if st.Lost > w.tx.lost {
		w.tx.lost = st.Lost
		w.sh.toastWarn(locales.TxLost())
	}
	if w.tx.wasManual && !st.Manual && !w.tx.busy {
		w.sh.toastWarn(locales.TxEnded())
	}
	w.tx.wasManual = st.Manual
}

// txGroup 是顶栏上的手动提交控件：开关；开启后再多出「提交」「回滚」两个按钮和未提交条数。
func (s *Shell) txGroup(c *ui.Context, p tokens, w *Workspace) {
	if !w.txSupported() {
		return
	}
	st := w.txStatus()
	w.watchTx(st)
	tip := locales.TxManual() + s.keyHint("tx-toggle") + "\n" + locales.TxManualTip()
	if toggleBtn(c, icCommit, tip, st.Manual).Disabled(w.tx.busy).Clicked() {
		s.toggleManualTx(w)
	}
	if !st.Manual {
		return
	}
	on := st.Active && !w.tx.busy
	ui.Row(c).Gap(0).AlignItems(ui.Center).Height(sizeBtnSm+2).Radius(radiusBtn).Border(1, p.borderSubtle).Margin(0, 0, 0, 4).Children(func() {
		seg := func(key string, ic *ui.SVG, tip string, col ui.Color, label string) bool {
			b := tipOn(c, ui.Button(c.Key(key), "").Label(tip), tip).Height(sizeBtnSm).Padding(0, 7).BorderWidth(0).Background(ui.Color{}).Disabled(!on)
			if b.Hovered() && on {
				b.Background(p.hover)
			}
			b.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, ic).FontSize(14).TextColor(col)
					if label != "" {
						ui.Text(c, label).FontSize(font2XS).FontWeight(600).TextColor(col)
					}
				})
			})
			return b.Clicked()
		}
		label := ""
		if st.Active && st.Pending > 0 {
			label = fmt.Sprint(st.Pending)
		}
		commitTip := locales.TxCommit() + s.keyHint("tx-commit")
		if st.Active && st.Pending > 0 {
			commitTip += "\n" + locales.TxPending(st.Pending)
		}
		if seg("tx-commit", icCheck, commitTip, p.accent, label) {
			s.finishTx(w, true)
		}
		ui.Box(c).Width(1).Height(14).Background(p.borderSubtle)
		if seg("tx-rollback", icUndo, locales.TxRollback()+s.keyHint("tx-rollback"), p.fgSecondary, "") {
			s.finishTx(w, false)
		}
	})
}

// toggleManualTx 开启或关闭手动提交；关闭时若有未提交的更改，先确认并提交。
func (s *Shell) toggleManualTx(w *Workspace) {
	if w == nil || !w.txSupported() || w.tx.busy {
		return
	}
	st := w.txStatus()
	id, db := w.cfg.ID, w.db
	if !st.Manual {
		s.setManualTx(w, id, db, true, false)
		return
	}
	if st.Active {
		s.askWarn(locales.TxCloseTitle(), locales.TxCloseMsg(max(st.Pending, 1)), locales.TxCloseConfirm(), func() {
			s.setManualTx(w, id, db, false, true)
		})
		return
	}
	s.setManualTx(w, id, db, false, false)
}

// setManualTx 在后台开关手动提交；commitFirst 为 true 时先提交现有事务。
func (s *Shell) setManualTx(w *Workspace, id, db string, on, commitFirst bool) {
	w.tx.busy = true
	var err error
	s.up.bg(func() {
		if commitFirst {
			if err = s.app.Queries.CommitTransaction(id, db); err != nil {
				return
			}
		}
		err = s.app.Queries.SetManualTransaction(id, db, on)
	}, func() {
		w.tx.busy = false
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		w.tx.wasManual = on
		if on {
			s.toastOK(locales.TxOn())
		} else {
			s.toast(locales.TxOff())
		}
		s.redraw()
	})
}

// finishTx 提交或回滚当前事务，完成后重新读取已打开的数据表（回滚会让网格里的内容过期）。
func (s *Shell) finishTx(w *Workspace, commit bool) {
	if w == nil || w.tx.busy || !w.txStatus().Active {
		return
	}
	w.tx.busy = true
	id, db := w.cfg.ID, w.db
	var err error
	s.up.bg(func() {
		if commit {
			err = s.app.Queries.CommitTransaction(id, db)
		} else {
			err = s.app.Queries.RollbackTransaction(id, db)
		}
	}, func() {
		w.tx.busy = false
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		if commit {
			s.toastOK(locales.TxCommitted())
		} else {
			s.toast(locales.TxRolledBack())
			w.reloadCleanTables()
		}
		s.redraw()
	})
}

// reloadCleanTables 重新读取没有未保存修改的表格标签页。
func (w *Workspace) reloadCleanTables() {
	for _, t := range w.tabs {
		if tt, ok := t.(*tableTab); ok && !tt.isNew && tt.pending() == 0 {
			tt.load()
		}
	}
}

// txGuard 离开前的确认：手动提交里还有未提交的更改时，说明继续会把它们回滚。
func (s *Shell) txGuard(n int, fn func()) {
	if n == 0 {
		fn()
		return
	}
	s.askWarn(locales.TxLeaveTitle(), locales.TxLeaveMsg(n), locales.TxLeaveConfirm(), fn)
}

// cmdTxToggle、cmdTxCommit、cmdTxRollback 是命令表里三个事务命令的动作。
func (s *Shell) cmdTxToggle() { s.toggleManualTx(s.activeWorkspace()) }
func (s *Shell) cmdTxCommit() { s.finishTx(s.activeWorkspace(), true) }
func (s *Shell) cmdTxRollback() {
	s.finishTx(s.activeWorkspace(), false)
}

// commitTxIfActive 供 ⌘S 兜底：表格没有待提交的修改时，⌘S 提交手动事务。
func (s *Shell) commitTxIfActive() {
	if w := s.activeWorkspace(); w != nil && w.txSupported() {
		s.finishTx(w, true)
	}
}

// releaseTx 工作区离开 db（关闭或切库）时，若没有别的工作区还在用这个库的手动提交，就回滚并关掉它。
// 用户已在 txGuard 里确认过，这里不再询问。
func (s *Shell) releaseTx(w *Workspace, db string) {
	if !w.txSupported() {
		return
	}
	for _, o := range s.spaces {
		if o != w && o.cfg.ID == w.cfg.ID && o.db == db {
			return
		}
	}
	id := w.cfg.ID
	w.tx = txState{}
	s.up.bg(func() {
		_ = s.app.Queries.RollbackTransaction(id, db)
		_ = s.app.Queries.SetManualTransaction(id, db, false)
	}, func() {})
}
