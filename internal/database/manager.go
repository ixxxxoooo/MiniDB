package database

import (
	"context"
	"database/sql"
	"minidb/internal/model"
	"strings"
	"sync"
	"time"

	"minidb/internal/appdata"
	"minidb/internal/logger"
	"minidb/internal/sshtunnel"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

const defaultConnectTimeout = 5 * time.Second

// 连接池生命周期：回收空闲/陈旧连接，避免被服务端或中间件（RDS/ProxySQL/TiDB LB）
// 单方面关闭后，下次复用时拿到死连接报 "bad connection"。
const (
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// Manager 数据库连接管理器
type Manager struct {
	mu    sync.RWMutex
	conns map[string]*sql.DB
	cfgs  map[string]*ConnectionConfig
	// PostgreSQL 的连接绑定在单个数据库上，无法像 MySQL 那样 USE 切库；
	// 访问其他库时按需为 (连接, 库名) 单独建池。key = connID + "\x00" + dbName
	dbPools map[string]*sql.DB
	// 每个启用 SSH 的连接对应一条隧道
	tunnels map[string]*sshtunnel.Tunnel
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		conns:   make(map[string]*sql.DB),
		cfgs:    make(map[string]*ConnectionConfig),
		dbPools: make(map[string]*sql.DB),
		tunnels: make(map[string]*sshtunnel.Tunnel),
	}
}

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

// Connect 建立数据库连接
func (m *Manager) Connect(cfg *ConnectionConfig) error {
	logger.Info("正在连接数据库: name=%s type=%s host=%s:%d db=%s",
		cfg.Name, cfg.Type, cfg.Host, cfg.Port, cfg.Database)

	eff, tun, err := prepareConfig(cfg)
	if err != nil {
		logger.Error("建立 SSH 隧道失败: %v", err)
		return err
	}
	closeTunnel := func() {
		if tun != nil {
			tun.Close()
		}
	}
	dsn, driverName, err := buildDSN(eff)
	if err != nil {
		closeTunnel()
		logger.Error("构建 DSN 失败: %v", err)
		return err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		closeTunnel()
		logger.Error("打开数据库失败: %v", err)
		return model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		closeTunnel()
		logger.Error("Ping 数据库失败: %v", err)
		return model.Wrapf(model.CodeDBConnectFailed, "连接数据库失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 新连接验证成功后再替换旧连接，避免一次失败重连打断现有可用连接。
	if oldDB, ok := m.conns[cfg.ID]; ok {
		logger.Info("检测到已有连接 %s，替换旧连接", cfg.ID)
		oldDB.Close()
	}

	m.closePoolsLocked(cfg.ID)
	m.setTunnelLocked(cfg.ID, tun)
	m.conns[cfg.ID] = db
	m.cfgs[cfg.ID] = cfg
	logger.Info("数据库连接成功: name=%s id=%s", cfg.Name, cfg.ID)
	return nil
}

// ConnectionIDs 返回当前已建立连接的 ID 列表。
func (m *Manager) ConnectionIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.conns))
	for id := range m.conns {
		ids = append(ids, id)
	}
	return ids
}

// Ping 检查连接是否仍然可用。
func (m *Manager) Ping(ctx context.Context, connID string) error {
	db, err := m.GetDB(connID)
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

// ErrDisconnected 表示重连期间连接已被用户断开，不应再被"复活"。
var ErrDisconnected = model.Errf(model.CodeDisconnected, "连接已被断开")

// Reconnect 用新配置替换已存在的连接池。与 Connect 的区别：只有当该连接此刻仍处于"已连接"状态
// 才会替换，避免后台自动重连把用户刚刚手动断开的连接重新连上。
func (m *Manager) Reconnect(cfg *ConnectionConfig) error {
	eff, tun, err := prepareConfig(cfg)
	if err != nil {
		return err
	}
	closeTunnel := func() {
		if tun != nil {
			tun.Close()
		}
	}
	dsn, driverName, err := buildDSN(eff)
	if err != nil {
		closeTunnel()
		return err
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		closeTunnel()
		return model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		closeTunnel()
		return model.Wrapf(model.CodeDBConnectFailed, "连接数据库失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.conns[cfg.ID]
	if !ok {
		db.Close()
		closeTunnel()
		return ErrDisconnected
	}
	old.Close()
	m.closePoolsLocked(cfg.ID)
	m.setTunnelLocked(cfg.ID, tun)
	m.conns[cfg.ID] = db
	m.cfgs[cfg.ID] = cfg
	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if db, ok := m.conns[connID]; ok {
		db.Close()
		delete(m.conns, connID)
		delete(m.cfgs, connID)
		m.closePoolsLocked(connID)
		m.setTunnelLocked(connID, nil)
		logger.Info("已断开数据库连接: %s", connID)
	}
	return nil
}

// GetDB 获取数据库连接
func (m *Manager) GetDB(connID string) (*sql.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db, ok := m.conns[connID]
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接不存在: %s", connID)
	}
	return db, nil
}

// setTunnelLocked 替换（或移除）连接的隧道，并关闭旧隧道。
func (m *Manager) setTunnelLocked(connID string, tun *sshtunnel.Tunnel) {
	if old, ok := m.tunnels[connID]; ok && old != tun {
		old.Close()
	}
	if tun == nil {
		delete(m.tunnels, connID)
		return
	}
	m.tunnels[connID] = tun
}

func (m *Manager) closePoolsLocked(connID string) {
	prefix := connID + "\x00"
	for key, pool := range m.dbPools {
		if strings.HasPrefix(key, prefix) {
			pool.Close()
			delete(m.dbPools, key)
		}
	}
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
	dsn, driverName, err := buildDSN(&derived)
	if err != nil {
		return nil, err
	}
	pool, err = sql.Open(driverName, dsn)
	if err != nil {
		return nil, model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(2)
	pool.SetConnMaxLifetime(defaultConnMaxLifetime)
	pool.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, model.Wrapf(model.CodeDBConnectNamed, "连接数据库 %s 失败: %w", dbName, err)
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

// GetConfig 获取连接配置
func (m *Manager) GetConfig(connID string) (*ConnectionConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.cfgs[connID]
	return cfg, ok
}

// TestConnection 测试连接，不保留连接句柄
func (m *Manager) TestConnection(cfg *ConnectionConfig) error {
	logger.Info("测试连接: type=%s host=%s:%d db=%s ssh=%v", cfg.Type, cfg.Host, cfg.Port, cfg.Database, cfg.SSH.Enabled)
	eff, tun, err := prepareConfig(cfg)
	if err != nil {
		logger.Error("测试连接建立 SSH 隧道失败: %v", err)
		return err
	}
	if tun != nil {
		defer tun.Close()
	}
	dsn, driverName, err := buildDSN(eff)
	if err != nil {
		logger.Error("测试连接构建 DSN 失败: %v", err)
		return err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		logger.Error("测试连接打开失败: %v", err)
		return model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("测试连接 Ping 失败: %v", err)
		return model.Wrapf(model.CodeConnTestFailed, "连接测试失败: %w", err)
	}
	logger.Info("测试连接成功")
	return nil
}

// CloseAll 关闭所有连接
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, db := range m.conns {
		db.Close()
		delete(m.conns, id)
	}
	for key, pool := range m.dbPools {
		pool.Close()
		delete(m.dbPools, key)
	}
	for id, tun := range m.tunnels {
		tun.Close()
		delete(m.tunnels, id)
	}
	// 同步清理配置，避免 GetConfig 返回已关闭连接的配置
	clear(m.cfgs)
}

// IsMySQLCompatible 判断数据库类型是否兼容 MySQL 协议（mysql、tidb、starrocks 均使用 MySQL 驱动）
func IsMySQLCompatible(dbType DBType) bool {
	return dbType == MySQL || dbType == TiDB || dbType == StarRocks
}

// UseDatabase switches the current schema for MySQL-compatible connections.
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

// Endpoint 返回客户端工具应当连的地址：启用 SSH 隧道时是本机转发端口，否则是配置的主机和端口。
func (m *Manager) Endpoint(connID string) (host string, port int, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, found := m.cfgs[connID]
	if !found {
		return "", 0, false
	}
	if tun, has := m.tunnels[connID]; has && tun != nil {
		return "127.0.0.1", tun.LocalPort(), true
	}
	return cfg.Host, cfg.Port, true
}
