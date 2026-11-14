// @author ixxxxoooo

package database

import "minidb/internal/model"

// UseDatabase 在需要会话内 USE 切库的连接上切换当前库（Caps.SessionUseDB）；其他类型或 dbName 为空时什么也不做。
// 参数：db 必须是固定在单条连接上的执行器（Session / Tx）；dbType 连接类型；dbName 目标库名。
// 返回：切换失败时返回带错误码的错误。
func UseDatabase(db Executor, dbType DBType, dbName string) error {
	if !CapsOf(dbType).SessionUseDB || dbName == "" {
		return nil
	}
	if _, err := db.Exec("USE " + QuoteIdent(dbType, dbName)); err != nil {
		return model.Wrapf(model.CodeSwitchDBFailed, "切换数据库失败: %w", err)
	}
	return nil
}

// buildDSN 根据配置构建 DSN，返回 (连接串, 驱动名)。差异在各方言的 DSN 方法里。
func buildDSN(cfg *ConnectionConfig) (string, string, error) {
	d, err := DialectOf(cfg.Type)
	if err != nil {
		return "", "", err
	}
	return d.DSN(cfg), d.Driver(), nil
}
