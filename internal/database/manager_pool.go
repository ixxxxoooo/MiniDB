// @author ixxxxoooo

package database

import (
	"context"
	"database/sql"
	"time"

	"minidb/internal/appdata"
	"minidb/internal/logger"
	"minidb/internal/model"
	"minidb/internal/sshtunnel"
)

const defaultConnectTimeout = 5 * time.Second

// 连接池生命周期：回收空闲/陈旧连接，避免被服务端或中间件（RDS/ProxySQL/TiDB LB）
// 单方面关闭后，下次复用时拿到死连接报 "bad connection"。
const (
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
	mainPoolMaxIdle        = 4 // 主连接池的空闲连接数
	derivedPoolMaxIdle     = 2 // 按库派生的连接池（PostgreSQL 等）的空闲连接数
)

// prepareConfig 返回实际用于建连的配置：启用 SSH 时先建立隧道，并把 Host/Port 指向本机转发端口。
// 未启用 SSH 时原样返回，隧道为 nil。调用方负责在失败或不再使用时关闭隧道。
func prepareConfig(cfg *ConnectionConfig) (*ConnectionConfig, *sshtunnel.Tunnel, error) {
	if !cfg.SSH.Enabled || !CapsOf(cfg.Type).Network {
		return cfg, nil, nil
	}
	tun, err := sshtunnel.Open(sshtunnel.Config{
		Host: cfg.SSH.Host, Port: cfg.SSH.Port, User: cfg.SSH.User,
		AuthType: cfg.SSH.AuthType, KeyPath: cfg.SSH.KeyPath, Secret: cfg.SSH.Secret,
		KnownHostsPath: appdata.KnownHostsPath(),
	}, cfg.Host, cfg.Port)
	if err != nil {
		return nil, nil, err
	}
	eff := *cfg
	eff.Host, eff.Port = "127.0.0.1", tun.LocalPort()
	return &eff, tun, nil
}

// openPool 按配置生成 DSN、打开连接池并 Ping 一次；失败时连接池已关闭。
// pingErr 把 Ping 的错误包装成调用场景对应的错误码（连接、测试连接、连接某个库）。
func openPool(cfg *ConnectionConfig, maxIdle int, pingErr func(error) error) (*sql.DB, error) {
	dsn, driverName, err := buildDSN(cfg)
	if err != nil {
		logger.Error("构建 DSN 失败: %v", err)
		return nil, err
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		logger.Error("打开数据库失败: %v", err)
		return nil, model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		logger.Error("Ping 数据库失败: %v", err)
		return nil, pingErr(err)
	}
	return db, nil
}

// dial 建立（需要时）SSH 隧道并打开主连接池。成功时返回的隧道交给调用方管理（可能为 nil）；
// 失败时隧道已关闭。
func dial(cfg *ConnectionConfig, pingErr func(error) error) (*sql.DB, *sshtunnel.Tunnel, error) {
	eff, tun, err := prepareConfig(cfg)
	if err != nil {
		logger.Error("建立 SSH 隧道失败: %v", err)
		return nil, nil, err
	}
	db, err := openPool(eff, mainPoolMaxIdle, pingErr)
	if err != nil {
		if tun != nil {
			tun.Close()
		}
		return nil, nil, err
	}
	return db, tun, nil
}

// connectFailed 是建立/重连主连接时 Ping 失败的错误包装。
func connectFailed(err error) error {
	return model.Wrapf(model.CodeDBConnectFailed, "连接数据库失败: %w", err)
}

// GetDBForDatabase 返回访问指定数据库应使用的连接池。
// MySQL/TiDB/StarRocks 通过会话内 USE 切库，直接返回主连接池；
// PostgreSQL 与 SQLite 的连接绑定单个库，访问其他 PG 库时按需建立独立连接池并缓存。
func (m *Manager) GetDBForDatabase(connID, dbName string) (*sql.DB, error) {
	m.mu.RLock()
	main, ok := m.conns[connID]
	cfg := m.cfgs[connID]
	tun := m.tunnels[connID]
	m.mu.RUnlock()
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接不存在: %s", connID)
	}
	if cfg == nil || !CapsOf(cfg.Type).PerDatabasePool || dbName == "" || dbName == cfg.Database {
		return main, nil
	}

	key := connID + "\x00" + dbName
	m.mu.RLock()
	pool := m.dbPools[key]
	m.mu.RUnlock()
	if pool != nil {
		return pool, nil
	}

	derived := *cfg
	derived.Database = dbName
	if tun != nil { // 经同一条隧道访问
		derived.Host, derived.Port = "127.0.0.1", tun.LocalPort()
	}
	pool, err := openPool(&derived, derivedPoolMaxIdle, func(e error) error {
		return model.Wrapf(model.CodeDBConnectNamed, "连接数据库 %s 失败: %w", dbName, e)
	})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 并发创建时保留先到者
	if existing := m.dbPools[key]; existing != nil {
		pool.Close()
		return existing, nil
	}
	if _, still := m.conns[connID]; !still {
		pool.Close()
		return nil, model.Errf(model.CodeConnNotFound, "连接不存在: %s", connID)
	}
	m.dbPools[key] = pool
	return pool, nil
}
