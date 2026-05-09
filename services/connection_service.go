package services

import (
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"
)

// ConnectionService 连接管理服务，负责连接的增删改查和连接/断开操作
type ConnectionService struct {
	store   *storage.Store
	manager *database.Manager
	schema  *schemaindex.Manager
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager, schema *schemaindex.Manager) *ConnectionService {
	return &ConnectionService{store: store, manager: manager, schema: schema}
}

// SaveConnection 保存连接配置到本地存储
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	logger.Info("[ConnectionService] 保存连接配置: name=%s type=%s", cfg.Name, cfg.Type)
	encryptedCfg, err := encryptConnectionConfig(cfg)
	if err != nil {
		return err
	}
	return s.store.Put("connections", cfg.ID, encryptedCfg)
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
		cfg := *item.(*database.ConnectionConfig)
		decrypted, err := decryptConnectionConfig(cfg)
		if err != nil {
			return nil, err
		}
		if cfg.Password != "" && !storage.IsEncryptedString(cfg.Password) {
			if encryptedCfg, err := encryptConnectionConfig(decrypted); err == nil {
				_ = s.store.Put("connections", encryptedCfg.ID, encryptedCfg)
			}
		}
		conns = append(conns, decrypted)
	}
	logger.Info("[ConnectionService] 获取到 %d 个连接配置", len(conns))
	return conns, nil
}

// DeleteConnection 删除连接配置，同时断开对应连接
func (s *ConnectionService) DeleteConnection(id string) error {
	logger.Info("[ConnectionService] 删除连接: %s", id)
	if s.schema != nil {
		s.schema.ForgetConnection(id)
	}
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
	cfg, err := decryptConnectionConfig(cfg)
	if err != nil {
		logger.Error("[ConnectionService] 解密连接配置失败: id=%s err=%v", id, err)
		return false, err.Error()
	}
	if cfg.Password != "" && !storage.IsEncryptedString(cfg.Password) {
		if encryptedCfg, err := encryptConnectionConfig(cfg); err == nil {
			_ = s.store.Put("connections", encryptedCfg.ID, encryptedCfg)
		}
	}

	if err := s.manager.Connect(&cfg); err != nil {
		logger.Error("[ConnectionService] 连接失败: id=%s err=%v", id, err)
		return false, err.Error()
	}
	if s.schema != nil && cfg.Database != "" {
		s.schema.WarmAsync(id, cfg.Database)
	}
	logger.Info("[ConnectionService] 连接成功: id=%s name=%s", id, cfg.Name)
	return true, ""
}

// Disconnect 断开指定连接
func (s *ConnectionService) Disconnect(id string) error {
	logger.Info("[ConnectionService] 断开连接: id=%s", id)
	if s.schema != nil {
		s.schema.ForgetConnection(id)
	}
	return s.manager.Disconnect(id)
}

func encryptConnectionConfig(cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	encryptedPassword, err := storage.EncryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = encryptedPassword
	return cfg, nil
}

func decryptConnectionConfig(cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	password, err := storage.DecryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = password
	return cfg, nil
}
