package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"minidb/internal/logger"
)

// 本文件是 ManualTx 会话本身的行为（开启事务、跟踪未提交条数、提交、回滚）；
// 按连接/库登记和管理会话的部分在 manual_tx.go。

// Status 返回当前状态快照。
func (t *ManualTx) Status() TxStatus {
	st := TxStatus{Manual: true, Active: t.active.Load(), Pending: int(t.pending.Load()), Lost: int(t.lost.Load())}
	if ns := t.since.Load(); ns > 0 && st.Active {
		st.Since = time.Unix(0, ns)
	}
	return st
}

// Use 独占这条会话执行 fn。会话在先前被取消打断而丢失时，会悄悄换一条新连接（此时事务已被服务端回滚）。
func (t *ManualTx) Use(ctx context.Context, fn func(*Session) error) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sess == nil {
		sess, err := OpenSession(context.Background(), t.db, t.dbType, t.dbName)
		if err != nil {
			return err
		}
		t.sess = sess
	}
	err := fn(t.sess)
	// 被取消的语句会让驱动直接断开连接（MySQL），连接状态不可信：丢掉它，事务随之回滚。
	if ctx != nil && ctx.Err() != nil {
		t.dropLocked()
	}
	return err
}

// dropLocked 丢弃当前连接；若事务未结束则记为"丢失"。调用方须持有 mu。
func (t *ManualTx) dropLocked() {
	if t.active.Load() {
		t.lost.Add(1)
		t.reset()
		logger.Warn("手动提交会话被中断，未提交的更改已回滚")
	}
	if t.sess != nil {
		_ = t.sess.Close()
		t.sess = nil
	}
}

func (t *ManualTx) reset() {
	t.active.Store(false)
	t.pending.Store(0)
	t.since.Store(0)
}

// close 释放连接。rollback 为 true 时先尽力回滚；拿不到锁（有查询在跑）就直接关，由服务端回滚。
func (t *ManualTx) close(rollback bool) {
	if !t.mu.TryLock() {
		return
	}
	defer t.mu.Unlock()
	if t.sess == nil {
		return
	}
	if rollback && t.active.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, _ = t.exec(ctx, t.sess, "ROLLBACK")
		cancel()
	}
	_ = t.sess.Close()
	t.sess = nil
	t.reset()
}

// exec 执行会话自己的事务控制语句，并把它上报给 hook（成功失败都报）。
func (t *ManualTx) exec(ctx context.Context, sess *Session, stmt string) (sql.Result, error) {
	start := time.Now()
	res, err := sess.ExecContext(ctx, stmt)
	if t.hook != nil {
		t.hook(t.connID, t.dbName, stmt, time.Since(start), err)
	}
	return res, err
}

// beginStmt 是该方言开启事务的语句。
func (t *ManualTx) beginStmt() string {
	if s := CapsOf(t.dbType).TxBegin; s != "" {
		return s
	}
	return "BEGIN"
}

// begin 在事务尚未开启时开启它。调用方须持有 mu。
func (t *ManualTx) begin(ctx context.Context, sess *Session) error {
	if t.active.Load() {
		return nil
	}
	if _, err := t.exec(ctx, sess, t.beginStmt()); err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	t.active.Store(true)
	t.pending.Store(0)
	t.since.Store(time.Now().UnixNano())
	return nil
}

// PrepareStatement 在执行 stmt 之前调用（须在 Use 的 fn 内）：写语句会先开启事务。
func (t *ManualTx) PrepareStatement(ctx context.Context, sess *Session, stmt string) error {
	switch SQLLeadingVerb(stmt) {
	case "begin", "start", "commit", "rollback", "end", "savepoint", "release":
		return nil // 用户自己控制事务，状态在 FinishStatement 里同步
	}
	if CheckReadOnlySQL(stmt).Allowed {
		return nil
	}
	return t.begin(ctx, sess)
}

// FinishStatement 在 stmt 执行后调用（须在 Use 的 fn 内），按结果同步事务状态和未提交条数。
func (t *ManualTx) FinishStatement(stmt string, execErr error) {
	if execErr != nil {
		return
	}
	verb := SQLLeadingVerb(stmt)
	switch verb {
	case "begin", "start":
		if !t.active.Load() {
			t.active.Store(true)
			t.pending.Store(0)
			t.since.Store(time.Now().UnixNano())
		}
		return
	case "commit", "rollback", "end":
		t.reset()
		return
	case "savepoint", "release":
		return
	}
	if CheckReadOnlySQL(stmt).Allowed {
		return
	}
	if CapsOf(t.dbType).ImplicitCommitDDL && isDDLVerb(verb) {
		t.reset() // MySQL 的 DDL 会隐式提交之前的所有更改
		return
	}
	t.pending.Add(1)
}

func isDDLVerb(verb string) bool {
	switch verb {
	case "create", "alter", "drop", "truncate", "rename":
		return true
	}
	return false
}

// RunWrite 在手动事务内执行一批行编辑（须在 Use 的 fn 内调用），成功后按 n 累加未提交条数。
func (t *ManualTx) RunWrite(ctx context.Context, sess *Session, n int, fn func(Executor) error) error {
	if err := t.begin(ctx, sess); err != nil {
		return err
	}
	if err := fn(sess); err != nil {
		return err
	}
	t.pending.Add(int64(n))
	return nil
}

// Commit 提交当前事务；没有事务时是空操作。
func (t *ManualTx) Commit(ctx context.Context) error { return t.finish(ctx, "COMMIT") }

// Rollback 回滚当前事务；没有事务时是空操作。
func (t *ManualTx) Rollback(ctx context.Context) error { return t.finish(ctx, "ROLLBACK") }

func (t *ManualTx) finish(ctx context.Context, verb string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.active.Load() {
		return nil
	}
	if t.sess == nil {
		t.reset()
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := t.exec(ctx, t.sess, verb); err != nil {
		// 连接坏了：事务已不可能再提交，丢弃连接，让界面看到"丢失"
		t.dropLocked()
		return fmt.Errorf("%s 失败: %w", verb, err)
	}
	logger.Info("手动事务 %s 完成: pending=%d", verb, t.pending.Load())
	t.reset()
	return nil
}
