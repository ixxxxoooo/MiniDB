package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"minidb/internal/logger"
	"minidb/internal/model"
)

// TxStatus 是手动提交会话的状态快照。
type TxStatus = model.TxStatus

// TxStmtHook 接收手动提交会话自己发出的事务控制语句（BEGIN/COMMIT/ROLLBACK），
// 供界面写进控制台日志。用户在编辑器里手写的这类语句走普通执行路径，不经过它。
// 在执行语句的协程里同步调用，实现必须立即返回。
type TxStmtHook func(connID, dbName, stmt string, dur time.Duration, err error)

// SetTxStmtHook 设置事务控制语句的上报回调；只影响之后开启的手动提交会话。
func (m *Manager) SetTxStmtHook(h TxStmtHook) {
	m.txMu.Lock()
	m.txHook = h
	m.txMu.Unlock()
}

// ManualTx 是"手动提交"模式下某个连接/库独占的一条会话。
//
// 开启后，该库上所有操作（查询器、表格浏览、行编辑）都复用这一条连接：
// 第一条写语句之前自动开启事务，之后一直保持，直到用户点提交或回滚。
// 纯读语句不会开启事务，避免空占快照和元数据锁。
type ManualTx struct {
	mu     sync.Mutex // 串行使用这条会话；Use 期间持有
	connID string
	hook   TxStmtHook // 事务控制语句的上报；可为 nil
	db     *sql.DB
	dbType DBType
	dbName string
	sess   *Session

	active  atomic.Bool
	pending atomic.Int64
	since   atomic.Int64 // 事务开始时间（UnixNano）
	lost    atomic.Int64
}

func manualTxKey(connID, dbName string) string { return connID + "\x00" + dbName }

// EnableManualTx 为 connID/dbName 开启手动提交模式；重复开启是空操作。
func (m *Manager) EnableManualTx(connID, dbName string) error {
	cfg, ok := m.GetConfig(connID)
	if !ok {
		return model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if !CapsOf(cfg.Type).ManualTx {
		return fmt.Errorf("%s 不支持手动提交", cfg.Type)
	}
	if cfg.ReadOnly {
		return fmt.Errorf("只读连接不能开启手动提交")
	}
	key := manualTxKey(connID, dbName)
	if m.ManualTx(connID, dbName) != nil {
		return nil
	}
	// 取连接与建会话可能要走网络，不能持有 txMu（界面线程每帧要读它）。
	db, err := m.GetDBForDatabase(connID, dbName)
	if err != nil {
		return err
	}
	sess, err := OpenSession(context.Background(), db, cfg.Type, dbName)
	if err != nil {
		return err
	}
	m.txMu.Lock()
	defer m.txMu.Unlock()
	if _, ok := m.manual[key]; ok {
		_ = sess.Close() // 并发开启，让先到的那个生效
		return nil
	}
	if m.manual == nil {
		m.manual = make(map[string]*ManualTx)
	}
	m.manual[key] = &ManualTx{connID: connID, hook: m.txHook, db: db, dbType: cfg.Type, dbName: dbName, sess: sess}
	logger.Info("已开启手动提交: conn=%s db=%s", connID, dbName)
	return nil
}

// ManualTx 返回 connID/dbName 上的手动提交会话；未开启返回 nil。
func (m *Manager) ManualTx(connID, dbName string) *ManualTx {
	m.txMu.Lock()
	defer m.txMu.Unlock()
	return m.manual[manualTxKey(connID, dbName)]
}

// ManualTxStatus 返回状态快照；不涉及任何 I/O，可以在界面线程直接调用。
func (m *Manager) ManualTxStatus(connID, dbName string) TxStatus {
	if t := m.ManualTx(connID, dbName); t != nil {
		return t.Status()
	}
	return TxStatus{}
}

// DisableManualTx 关闭手动提交模式；事务仍未结束时拒绝，必须先提交或回滚。
func (m *Manager) DisableManualTx(connID, dbName string) error {
	key := manualTxKey(connID, dbName)
	m.txMu.Lock()
	t, ok := m.manual[key]
	if !ok {
		m.txMu.Unlock()
		return nil
	}
	if t.active.Load() {
		m.txMu.Unlock()
		return fmt.Errorf("还有 %d 条未提交的更改，请先提交或回滚", t.pending.Load())
	}
	delete(m.manual, key)
	m.txMu.Unlock()
	t.close(true)
	logger.Info("已关闭手动提交: conn=%s db=%s", connID, dbName)
	return nil
}

// discardManualTx 丢弃 connID 下所有手动提交会话（断开或重建连接池时调用）。
// 事务随连接一起被服务端回滚；用 TryLock 避免等待一条正在跑的长查询。
func (m *Manager) discardManualTx(connID string) {
	prefix := connID + "\x00"
	m.txMu.Lock()
	var gone []*ManualTx
	for key, t := range m.manual {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			gone = append(gone, t)
			delete(m.manual, key)
		}
	}
	m.txMu.Unlock()
	for _, t := range gone {
		if t.active.Load() {
			t.lost.Add(1)
			logger.Warn("连接关闭，丢弃未提交事务: conn=%s pending=%d", connID, t.pending.Load())
		}
		t.close(false)
	}
}
