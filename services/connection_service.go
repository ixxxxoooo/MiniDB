package services

import (
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/storage"
)

// ConnectionService 连接管理服务
type ConnectionService struct {
	store   *storage.Store
	manager *database.Manager
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager) *ConnectionService {
	return &ConnectionService{store: store, manager: manager}
}

// SaveConnection 保存连接配置
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	return s.store.Put("connections", cfg.ID, cfg)
}

// GetConnections 获取所有连接配置
func (s *ConnectionService) GetConnections() ([]database.ConnectionConfig, error) {
	items, err := s.store.List("connections", func() interface{} {
		return &database.ConnectionConfig{}
	})
	if err != nil {
		return nil, err
	}

	var conns []database.ConnectionConfig
	for _, item := range items {
		conns = append(conns, *item.(*database.ConnectionConfig))
	}
	return conns, nil
}

// DeleteConnection 删除连接配置
func (s *ConnectionService) DeleteConnection(id string) error {
	s.manager.Disconnect(id)
	return s.store.Delete("connections", id)
}

// TestConnection 测试连接
func (s *ConnectionService) TestConnection(cfg database.ConnectionConfig) (bool, string) {
	err := s.manager.TestConnection(&cfg)
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}

// Connect 建立连接
func (s *ConnectionService) Connect(id string) (bool, string) {
	var cfg database.ConnectionConfig
	if err := s.store.Get("connections", id, &cfg); err != nil {
		return false, err.Error()
	}

	if err := s.manager.Connect(&cfg); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// Disconnect 断开连接
func (s *ConnectionService) Disconnect(id string) error {
	return s.manager.Disconnect(id)
}
