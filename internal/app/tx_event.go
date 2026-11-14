package app

import (
	"time"

	"minidb/internal/database"
)

// TxStmtEvent 是手动提交会话自己发出的一条事务控制语句（BEGIN、COMMIT、ROLLBACK），
// 界面把它写进控制台日志，这样日志里能看到完整的事务边界。
//
// @author ixxxxoooo
type TxStmtEvent struct {
	ConnID   string
	DB       string
	SQL      string
	Duration time.Duration
	Error    string // 空表示成功
}

// hookTxStatements 把数据库层的事务控制语句转成总线事件。
//
// 参数：manager 数据库管理器；bus 事件总线（nil 时什么也不做）。
func hookTxStatements(manager *database.Manager, bus *Bus) {
	if bus == nil {
		return
	}
	manager.SetTxStmtHook(func(connID, dbName, stmt string, dur time.Duration, err error) {
		ev := TxStmtEvent{ConnID: connID, DB: dbName, SQL: stmt, Duration: dur}
		if err != nil {
			ev.Error = err.Error()
		}
		bus.Emit(TopicTxStmt, ev)
	})
}
