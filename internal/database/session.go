package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	"minidb/internal/logger"
)

// Executor 抽象了 *sql.DB / *sql.Tx / *Session 共有的执行能力，
// 让查询与元数据函数既能跑在连接池上，也能跑在一条固定的会话连接上。
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Session 是从连接池中独占取出的一条连接。
// `USE db`、`SET @var`、事务、临时表等会话级状态只在同一条连接上有效，
// 因此凡是依赖"当前库"或跨语句状态的操作都必须经由 Session 执行，
// 不能直接在 *sql.DB 上执行（连续两次调用可能落在池中的不同连接上）。
type Session struct {
	ctx  context.Context
	conn *sql.Conn
}

func (s *Session) Exec(query string, args ...any) (sql.Result, error) {
	defer logger.Perf("db.exec", "sql", logger.SQLText(query))()
	return s.conn.ExecContext(s.ctx, query, args...)
}

func (s *Session) Query(query string, args ...any) (*sql.Rows, error) {
	defer logger.Perf("db.query", "sql", logger.SQLText(query))() // 只量到拿到结果集，逐行读取的时间不在内
	return s.conn.QueryContext(s.ctx, query, args...)
}

func (s *Session) QueryRow(query string, args ...any) *sql.Row {
	return s.conn.QueryRowContext(s.ctx, query, args...)
}

func (s *Session) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	defer logger.Perf("db.exec", "sql", logger.SQLText(query))()
	return s.conn.ExecContext(ctx, query, args...)
}

func (s *Session) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	defer logger.Perf("db.query", "sql", logger.SQLText(query))()
	return s.conn.QueryContext(ctx, query, args...)
}

func (s *Session) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.conn.QueryRowContext(ctx, query, args...)
}

// Context 返回会话创建时传入的 ctx，会话上的元数据读取用它保持与调用方一致的取消/超时。
func (s *Session) Context() context.Context { return s.ctx }

// Begin 在该会话连接上开启事务。
func (s *Session) Begin() (*sql.Tx, error) {
	return s.conn.BeginTx(s.ctx, nil)
}

// BeginReadOnly 在该会话连接上开启只读事务（由驱动/服务端强制拒绝写操作）。
func (s *Session) BeginReadOnly() (*sql.Tx, error) {
	return s.conn.BeginTx(s.ctx, &sql.TxOptions{ReadOnly: true})
}

// OpenSession 取一条独占连接并切换到 dbName（仅 MySQL 协议族需要）。
// 调用方必须 Close() 归还连接。适合后台长任务（如流式导出）在整个任务期间固定一条连接。
func OpenSession(ctx context.Context, db *sql.DB, dbType DBType, dbName string) (*Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取数据库会话失败: %w", err)
	}
	session := &Session{ctx: ctx, conn: conn}
	if err := UseDatabase(session, dbType, dbName); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return session, nil
}

// Close 把连接归还连接池。下一次使用者会重新 USE，因此残留的当前库不会造成串库。
func (s *Session) Close() error {
	return s.conn.Close()
}

// WithSession 取一条独占连接，在其上切换到 dbName，再执行 fn；fn 返回后连接归还连接池。
func WithSession(ctx context.Context, db *sql.DB, dbType DBType, dbName string, fn func(*Session) error) error {
	session, err := OpenSession(ctx, db, dbType, dbName)
	if err != nil {
		return err
	}
	defer session.Close()
	return fn(session)
}

// WithQueryOnly 在会话连接上打开 SQLite 的 PRAGMA query_only 再执行 fn：
// SQLite 驱动的只读事务选项不生效，query_only 才会让引擎本身拒绝一切写入（含 DDL）。
// fn 返回后一定关回去；关不掉就把这条连接标记为损坏丢弃，避免只读状态漏给下一个使用者。
func (s *Session) WithQueryOnly(fn func() error) error {
	if _, err := s.conn.ExecContext(s.ctx, "PRAGMA query_only = ON"); err != nil {
		return fmt.Errorf("开启 query_only 失败: %w", err)
	}
	defer func() {
		// 调用方的 ctx 可能已取消，复位要用独立的 ctx
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := s.conn.ExecContext(ctx, "PRAGMA query_only = OFF"); err != nil {
			_ = s.conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	return fn()
}

// SupportsReadOnlyTx 判断该方言的只读事务是否真的会拒绝写操作。
// StarRocks 不支持事务；SQLite 驱动的 ReadOnly 选项不生效（改用 Session.WithQueryOnly）。
func SupportsReadOnlyTx(dbType DBType) bool { return CapsOf(dbType).ReadOnlyTx }
