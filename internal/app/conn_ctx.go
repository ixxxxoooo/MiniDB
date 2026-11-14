package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"minidb/internal/database"
)

// 元数据类查询（库表列表、列、DDL、统计…）的默认超时。卡住的查询到点会被取消，而不是一直占着界面。
const (
	metaTimeout    = 30 * time.Second // 元数据读取
	adminTimeout   = 15 * time.Second // 用户、进程等管理类读取与终止
	longMetaWindow = 2 * time.Minute  // 整库级别的读取（搜索、批量构建索引）
)

// connCtx 是一次对连接的访问所需的全部上下文：连接池、配置与方言。
// 取连接的样板（取池 + 取配置 + 错误包装）只在 resolveConn 里写一遍。
type connCtx struct {
	DB      *sql.DB
	Cfg     *database.ConnectionConfig
	Dialect database.Dialect
}

// resolveConn 取出 connID 在 dbName 库上使用的连接池与配置。
// dbName 为空时取主连接；PostgreSQL 访问其他库时会按需建独立连接池（见 Manager.GetDBForDatabase）。
func resolveConn(m *database.Manager, connID, dbName string) (connCtx, error) {
	db, err := m.GetDBForDatabase(connID, dbName)
	if err != nil {
		return connCtx{}, fmt.Errorf("获取数据库连接失败: %w", err)
	}
	cfg, ok := m.GetConfig(connID)
	if !ok {
		return connCtx{}, fmt.Errorf("连接配置不存在: %s", connID)
	}
	d, err := database.DialectOf(cfg.Type)
	if err != nil {
		return connCtx{}, err
	}
	return connCtx{DB: db, Cfg: cfg, Dialect: d}, nil
}

// resolveConfig 只取配置（不需要连接池的纯 SQL 生成场景）。
func resolveConfig(m *database.Manager, connID string) (*database.ConnectionConfig, error) {
	cfg, ok := m.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	return cfg, nil
}

// withTimeout 返回带默认超时的 ctx，调用方 defer cancel()。
func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// with 取 connID 在 dbName 库上的连接上下文。
func (s *DatabaseService) with(connID, dbName string) (connCtx, error) {
	return resolveConn(s.manager, connID, dbName)
}
