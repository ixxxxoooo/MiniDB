package ui

import "minidb/internal/app"

// rowEditable 报告这个连接的数据网格能不能直接改、增、删行。
// 与连接的「只读」开关不同：这是引擎本身的限制（如 ClickHouse 的 UPDATE/DELETE 是异步 mutation），
// 不能靠关掉只读开关解决；SQL 编辑器不受影响。
func (w *Workspace) rowEditable() bool { return app.CapsOf(w.cfg.Type).RowEdit }

// engineLabel 是连接的数据库类型的显示名（提示语里用）。
func (w *Workspace) engineLabel() string { return app.DialectLabel(w.cfg.Type) }
