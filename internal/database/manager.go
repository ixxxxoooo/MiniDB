package database

import (
	"database/sql"
	"fmt"
	"sync"
	"tableplus-ai/internal/logger"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"sslMode"`
	Color    string `json:"color"`
	Group    string `json:"group"`
}

// Manager 数据库连接管理器
type Manager struct {
	mu    sync.RWMutex
	conns map[string]*sql.DB
	cfgs  map[string]*ConnectionConfig
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		conns: make(map[string]*sql.DB),
		cfgs:  make(map[string]*ConnectionConfig),
	}
}

// Connect 建立数据库连接
func (m *Manager) Connect(cfg *ConnectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Info("正在连接数据库: name=%s type=%s host=%s:%d db=%s",
		cfg.Name, cfg.Type, cfg.Host, cfg.Port, cfg.Database)

	// 如果已有连接，先关闭
	if db, ok := m.conns[cfg.ID]; ok {
		logger.Info("检测到已有连接 %s，先关闭旧连接", cfg.ID)
		db.Close()
		delete(m.conns, cfg.ID)
	}

	dsn, driverName, err := buildDSN(cfg)
	if err != nil {
		logger.Error("构建 DSN 失败: %v", err)
		return err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		logger.Error("打开数据库失败: %v", err)
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		logger.Error("Ping 数据库失败: %v", err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	m.conns[cfg.ID] = db
	m.cfgs[cfg.ID] = cfg
	logger.Info("数据库连接成功: name=%s id=%s", cfg.Name, cfg.ID)
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

// GetConfig 获取连接配置
func (m *Manager) GetConfig(connID string) (*ConnectionConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.cfgs[connID]
	return cfg, ok
}

// TestConnection 测试连接，不保留连接句柄
func (m *Manager) TestConnection(cfg *ConnectionConfig) error {
	logger.Info("测试连接: type=%s host=%s:%d db=%s", cfg.Type, cfg.Host, cfg.Port, cfg.Database)
	dsn, driverName, err := buildDSN(cfg)
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

	if err := db.Ping(); err != nil {
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
}

// buildDSN 根据配置构建 DSN
func buildDSN(cfg *ConnectionConfig) (string, string, error) {
	switch cfg.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		return dsn, "mysql", nil
	case "postgres":
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode)
		return dsn, "postgres", nil
	case "sqlite":
		return cfg.Database, "sqlite3", nil
	default:
		return "", "", fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
}
