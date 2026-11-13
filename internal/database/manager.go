package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"minidb/internal/appdata"
	"minidb/internal/logger"
	"minidb/internal/sshtunnel"

	mysql "github.com/go-sql-driver/mysql"
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

// SSHConfig SSH 隧道配置：启用后通过跳板机转发到 Host:Port，数据库驱动连接本机转发端口。
type SSHConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	AuthType string `json:"authType"` // "password" | "key"
	KeyPath  string `json:"keyPath"`
	// Secret 为 SSH 密码或私钥口令（取决于 AuthType），落库时加密。
	Secret string `json:"secret"`
	// HasSecret 仅用于前端展示：列表接口不回传 Secret。不会被持久化。
	HasSecret bool `json:"hasSecret,omitempty"`
	// ClearSecret 仅在保存时有效：显式清除已保存的 Secret（Secret 为空默认表示"保持不变"）。不会被持久化。
	ClearSecret bool `json:"clearSecret,omitempty"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	// HasPassword 仅用于前端展示：列表接口不回传明文密码，用它表示"已保存过密码"。不会被持久化。
	HasPassword bool `json:"hasPassword,omitempty"`
	// ClearPassword 仅在保存时有效：显式清除已保存的密码（Password 为空默认表示"保持不变"）。不会被持久化。
	ClearPassword bool   `json:"clearPassword,omitempty"`
	Database      string `json:"database"`
	SSLMode       string `json:"sslMode"`
	Color         string `json:"color"`
	Group         string `json:"group"`
	Tag           string `json:"tag"` // 环境标签：local/test/production
	// ReadOnly 只读模式：后端拒绝一切写操作（SQL 编辑器、表数据编辑、DDL、清空/删表）。
	ReadOnly bool `json:"readOnly"`
	// SafeMode 安全级别（silent/alert1/alert2/safe1/safe2）；空表示未设置，见 EffectiveSafeMode。
	SafeMode string `json:"safeMode,omitempty"`
	// PasswordMode 密码来源：空或 keychain 为已保存（系统钥匙串）；ask 每次连接时询问；none 不使用密码。
	PasswordMode string `json:"passwordMode,omitempty"`
	// SSH SSH 隧道（仅网络型数据库）。
	SSH SSHConfig `json:"ssh"`
}

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
	if !cfg.SSH.Enabled || cfg.Type == "sqlite" {
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
		return fmt.Errorf("打开数据库失败: %w", err)
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
		return fmt.Errorf("连接数据库失败: %w", err)
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
var ErrDisconnected = fmt.Errorf("连接已被断开")

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
		return fmt.Errorf("打开数据库失败: %w", err)
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
		return fmt.Errorf("连接数据库失败: %w", err)
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
		return nil, fmt.Errorf("连接不存在: %s", connID)
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
		return nil, fmt.Errorf("连接不存在: %s", connID)
	}
	if cfg == nil || cfg.Type != "postgres" || dbName == "" || dbName == cfg.Database {
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
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(2)
	pool.SetConnMaxLifetime(defaultConnMaxLifetime)
	pool.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接数据库 %s 失败: %w", dbName, err)
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
		return nil, fmt.Errorf("连接不存在: %s", connID)
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
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("测试连接 Ping 失败: %v", err)
		return fmt.Errorf("连接测试失败: %w", err)
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
func IsMySQLCompatible(dbType string) bool {
	return dbType == "mysql" || dbType == "tidb" || dbType == "starrocks"
}

// UseDatabase switches the current schema for MySQL-compatible connections.
func UseDatabase(db Executor, dbType, dbName string) error {
	if !IsMySQLCompatible(dbType) || dbName == "" {
		return nil
	}
	if _, err := db.Exec("USE " + QuoteIdent(dbType, dbName)); err != nil {
		return fmt.Errorf("切换数据库失败: %w", err)
	}
	return nil
}

// buildDSN 根据配置构建 DSN
func buildDSN(cfg *ConnectionConfig) (string, string, error) {
	switch cfg.Type {
	case "mysql", "tidb":
		return mysqlDSN(cfg, false), "mysql", nil
	case "starrocks":
		// StarRocks 不支持 COM_STMT_PREPARE，需要 interpolateParams=true 让驱动在客户端插值参数
		return mysqlDSN(cfg, true), "mysql", nil
	case "postgres":
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(cfg.User, cfg.Password),
			Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Path:   cfg.Database,
		}
		q := u.Query()
		q.Set("sslmode", sslMode)
		q.Set("connect_timeout", fmt.Sprintf("%d", int(defaultConnectTimeout/time.Second)))
		u.RawQuery = q.Encode()
		dsn := u.String()
		return dsn, "postgres", nil
	case "sqlite":
		return sqliteDSN(cfg.Database), "sqlite", nil
	default:
		return "", "", fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
}

// sqliteDSN opens path with a 5 s busy timeout. The previous cgo driver had
// that by default; modernc.org/sqlite waits not at all, so concurrent pooled
// connections (a DDL while a read finishes) fail with SQLITE_BUSY.
func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") || path == ":memory:" {
		return path
	}
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?_pragma=busy_timeout(5000)"
}

func mysqlDSN(cfg *ConnectionConfig, interpolateParams bool) string {
	c := mysql.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName:               cfg.Database,
		AllowNativePasswords: true,
		ParseTime:            true,
		Loc:                  time.Local,
		InterpolateParams:    interpolateParams,
		// 只限制建连超时。不能设置 ReadTimeout/WriteTimeout：它们作用于每一次网络读写，
		// 会让任何执行时间超过该值的查询、DDL、导出批次以 i/o timeout 失败。
		// 语句级超时与取消统一由调用方的 context 控制。
		Timeout: defaultConnectTimeout,
		Params: map[string]string{
			"charset": "utf8mb4",
		},
	}
	return c.FormatDSN()
}
