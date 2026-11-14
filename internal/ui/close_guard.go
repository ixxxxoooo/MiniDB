package ui

// 关闭工作区、断开连接前的确认：先看表格里有没有没提交的修改，再看手动提交里有没有没提交的事务。

// closeSpaceGuarded asks first when the workspace has uncommitted changes.
func (s *Shell) closeSpaceGuarded(w *Workspace) {
	s.discardGuard(w.pendingTotal(), func() {
		s.txGuard(w.txPendingTotal(), func() {
			for i, o := range s.spaces {
				if o == w {
					s.closeSpace(i)
					return
				}
			}
		})
	})
}

// closeConnection closes every workspace of a connection (the toolbar's
// disconnect), after asking if any holds uncommitted changes.
func (s *Shell) closeConnection(id string) {
	n, tx := 0, 0
	seen := map[string]bool{}
	for _, w := range s.spaces {
		if w.cfg.ID == id {
			n += w.pendingTotal()
			if !seen[w.db] { // 同一个库的几个工作区共用一个事务，只数一次
				seen[w.db] = true
				tx += w.txPendingTotal()
			}
		}
	}
	s.discardGuard(n, func() {
		s.txGuard(tx, func() {
			for i := len(s.spaces) - 1; i >= 0; i-- {
				if s.spaces[i].cfg.ID == id {
					s.closeSpace(i)
				}
			}
		})
	})
}
