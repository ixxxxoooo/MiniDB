package ui

import (
	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// setVersion 记下服务器版本；顺便看一下是不是 MariaDB 被当成了 MySQL。
func (w *Workspace) setVersion(ver string) {
	w.version = ver
	w.hintMariaDB()
}

// hintMariaDB 在连上的服务器其实是 MariaDB、而连接类型还是 MySQL 时提示一次：可以一键改成 MariaDB 类型，也可以不管。
// 每个连接每次运行只提示一次，不会替用户改类型。
func (w *Workspace) hintMariaDB() {
	target, ok := app.RefineType(w.cfg.Type, w.version)
	if !ok {
		return
	}
	s := w.sh
	if s.mariaHinted == nil {
		s.mariaHinted = map[string]bool{}
	}
	if s.mariaHinted[w.cfg.ID] {
		return
	}
	s.mariaHinted[w.cfg.ID] = true
	cfg := w.cfg
	s.toastAction(locales.MariadbHint(), locales.MariadbSwitch(), func() { s.switchConnectionType(cfg, target) })
}

// switchConnectionType 把连接的类型改成 typ 存下来，再用新类型重新打开。
func (s *Shell) switchConnectionType(cfg model.ConnectionConfig, typ model.DBType) {
	cfg.Type = typ
	var err error
	s.up.bg(func() { err = s.app.Connections.SaveConnection(cfg) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.closeConnection(cfg.ID)
		s.loadConnections()
		s.openConnection(cfg)
	})
}
