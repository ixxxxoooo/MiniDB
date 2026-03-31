package services

import (
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
)

// ConnectionService 连接管理服务，负责连接的增删改查和连接/断开操作
type ConnectionService struct {
	store   *storage.Store
	manager *database.Manager
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager) *ConnectionService {
	return &ConnectionService{store: store, manager: manager}
}

// SaveConnection 保存连接配置到本地存储
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	logger.Info("[ConnectionService] 保存连接配置: name=%s type=%s", cfg.Name, cfg.Type)
	return s.store.Put("connections", cfg.ID, cfg)
}

// GetConnections 获取所有连接配置
func (s *ConnectionService) GetConnections() ([]database.ConnectionConfig, error) {
	items, err := s.store.List("connections", func() interface{} {
		return &database.ConnectionConfig{}
	})
	if err != nil {
		logger.Error("[ConnectionService] 获取连接列表失败: %v", err)
		return nil, err
	}

	var conns []database.ConnectionConfig
	for _, item := range items {
		conns = append(conns, *item.(*database.ConnectionConfig))
	}
	logger.Info("[ConnectionService] 获取到 %d 个连接配置", len(conns))
	return conns, nil
}

// DeleteConnection 删除连接配置，同时断开对应连接
func (s *ConnectionService) DeleteConnection(id string) error {
	logger.Info("[ConnectionService] 删除连接: %s", id)
	s.manager.Disconnect(id)
	return s.store.Delete("connections", id)
}

// TestConnection 测试连接是否可达
func (s *ConnectionService) TestConnection(cfg database.ConnectionConfig) (bool, string) {
	logger.Info("[ConnectionService] 测试连接: name=%s", cfg.Name)
	err := s.manager.TestConnection(&cfg)
	if err != nil {
		logger.Warn("[ConnectionService] 测试连接失败: %v", err)
		return false, err.Error()
	}
	logger.Info("[ConnectionService] 测试连接成功: name=%s", cfg.Name)
	return true, ""
}

// Connect 根据 ID 从存储加载配置并建立连接
func (s *ConnectionService) Connect(id string) (bool, string) {
	logger.Info("[ConnectionService] 正在连接: id=%s", id)
	var cfg database.ConnectionConfig
	if err := s.store.Get("connections", id, &cfg); err != nil {
		logger.Error("[ConnectionService] 加载连接配置失败: id=%s err=%v", id, err)
		return false, err.Error()
	}

	if err := s.manager.Connect(&cfg); err != nil {
		logger.Error("[ConnectionService] 连接失败: id=%s err=%v", id, err)
		return false, err.Error()
	}
	logger.Info("[ConnectionService] 连接成功: id=%s name=%s", id, cfg.Name)
	return true, ""
}

// Disconnect 断开指定连接
func (s *ConnectionService) Disconnect(id string) error {
	logger.Info("[ConnectionService] 断开连接: id=%s", id)
	return s.manager.Disconnect(id)
}
