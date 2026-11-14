package database

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"minidb/internal/logger"
	"minidb/internal/model"
	"minidb/internal/sshtunnel"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
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
	// 开启了"手动提交"的会话，key = connID + "\x00" + dbName。独立的锁：
	// 界面线程要每帧读它的状态，不能被 mu（重连时会长时间持有）挡住。
	txMu   sync.Mutex
	manual map[string]*ManualTx
	txHook TxStmtHook // 新建的手动提交会话用它上报 BEGIN/COMMIT/ROLLBACK，受 txMu 保护
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

// installLocked 用新建好的连接池与隧道替换连接 ID 对应的旧资源（旧池、派生池、旧隧道一并关闭）。
// 调用方须持有写锁，且新连接已验证可用，这样一次失败的重连不会打断现有连接。
func (m *Manager) installLocked(cfg *ConnectionConfig, db *sql.DB, tun *sshtunnel.Tunnel) {
	m.discardManualTx(cfg.ID)
	if old, ok := m.conns[cfg.ID]; ok {
		old.Close()
	}
	m.closePoolsLocked(cfg.ID)
	m.setTunnelLocked(cfg.ID, tun)
	m.conns[cfg.ID] = db
	m.cfgs[cfg.ID] = cfg
}

// Connect 建立数据库连接
func (m *Manager) Connect(cfg *ConnectionConfig) error {
	logger.Info("正在连接数据库: name=%s type=%s host=%s:%d db=%s",
		cfg.Name, cfg.Type, cfg.Host, cfg.Port, cfg.Database)

	db, tun, err := dial(cfg, connectFailed)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conns[cfg.ID]; ok {
		logger.Info("检测到已有连接 %s，替换旧连接", cfg.ID)
	}
	m.installLocked(cfg, db, tun)
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
	db, tun, err := dial(cfg, connectFailed)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conns[cfg.ID]; !ok {
		db.Close()
		if tun != nil {
			tun.Close()
		}
		return ErrDisconnected
	}
	m.installLocked(cfg, db, tun)
	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.discardManualTx(connID)
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
	db, tun, err := dial(cfg, func(e error) error {
		return model.Wrapf(model.CodeConnTestFailed, "连接测试失败: %w", e)
	})
	if err != nil {
		return err
	}
	db.Close()
	if tun != nil {
		tun.Close()
	}
	logger.Info("测试连接成功")
	return nil
}

// CloseAll 关闭所有连接
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.conns {
		m.discardManualTx(id)
	}
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
