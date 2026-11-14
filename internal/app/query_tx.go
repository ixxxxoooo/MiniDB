package app

import (
	"context"

	"minidb/internal/logger"
	"minidb/internal/model"
)

// 手动提交（事务模式）的服务方法；会话本身在 database/manual_tx*.go。

// SetManualTransaction 开启或关闭 connID/dbName 上的手动提交模式。
// 关闭时若还有未提交的更改会返回错误，调用方应先提交或回滚。
func (s *QueryService) SetManualTransaction(connID, dbName string, on bool) error {
	defer logger.Perf("app.Queries.SetManualTransaction", "conn", connID, "db", dbName)()
	if on {
		return s.manager.EnableManualTx(connID, dbName)
	}
	return s.manager.DisableManualTx(connID, dbName)
}

// TransactionStatus 返回手动提交状态快照（纯内存读取，可在界面线程调用）。
func (s *QueryService) TransactionStatus(connID, dbName string) model.TxStatus {
	return s.manager.ManualTxStatus(connID, dbName)
}

// CommitTransaction 提交手动提交模式下的当前事务。
func (s *QueryService) CommitTransaction(connID, dbName string) error {
	defer logger.Perf("app.Queries.CommitTransaction", "conn", connID, "db", dbName)()
	mt := s.manager.ManualTx(connID, dbName)
	if mt == nil {
		return nil
	}
	if err := mt.Commit(context.Background()); err != nil {
		return err
	}
	s.refreshAfterTx(connID, dbName)
	return nil
}

// RollbackTransaction 回滚手动提交模式下的当前事务。
func (s *QueryService) RollbackTransaction(connID, dbName string) error {
	defer logger.Perf("app.Queries.RollbackTransaction", "conn", connID, "db", dbName)()
	mt := s.manager.ManualTx(connID, dbName)
	if mt == nil {
		return nil
	}
	if err := mt.Rollback(context.Background()); err != nil {
		return err
	}
	s.refreshAfterTx(connID, dbName)
	return nil
}

// refreshAfterTx 事务结束后让结构缓存重新对齐（回滚可能撤销了 DDL）。
func (s *QueryService) refreshAfterTx(connID, dbName string) {
	if s.schema != nil {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
}
