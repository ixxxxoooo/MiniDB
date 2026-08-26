package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const connectionsBackupVersion = 1

// ConnectionsBackup 连接配置备份文件结构
type ConnectionsBackup struct {
	Version     int                         `json:"version"`
	ExportedAt  string                      `json:"exportedAt"`
	Connections []database.ConnectionConfig `json:"connections"`
}

// ImportConnectionsResult 导入连接配置结果
type ImportConnectionsResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ConnectionService 连接管理服务，负责连接的增删改查和连接/断开操作
type ConnectionService struct {
	app     *application.App
	store   *storage.Store
	manager *database.Manager
	schema  *schemaindex.Manager
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager, schema *schemaindex.Manager) *ConnectionService {
	return &ConnectionService{store: store, manager: manager, schema: schema}
}

// SetWailsApplication 设置 Wails 应用实例（在 startup 时调用）
//
//wails:ignore
func (s *ConnectionService) SetWailsApplication(app *application.App) {
	s.app = app
}

// validateConnectionConfig 校验连接配置必填字段与合法性
func validateConnectionConfig(cfg database.ConnectionConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("连接配置 ID 不能为空")
	}
	validTypes := map[string]bool{"mysql": true, "postgres": true, "sqlite": true, "tidb": true, "starrocks": true}
	if !validTypes[cfg.Type] {
		return fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
	if cfg.Type != "sqlite" {
		if cfg.Host == "" {
			return fmt.Errorf("主机地址不能为空")
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			return fmt.Errorf("端口号不合法: %d（范围 1-65535）", cfg.Port)
		}
	}
	return nil
}

// SaveConnection 保存连接配置到本地存储
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	if err := validateConnectionConfig(cfg); err != nil {
		return fmt.Errorf("连接配置校验失败: %w", err)
	}
	logger.Info("[ConnectionService] 保存连接配置: name=%s type=%s", cfg.Name, cfg.Type)
	encryptedCfg, err := encryptConnectionConfig(cfg)
	if err != nil {
		return fmt.Errorf("加密连接配置失败: %w", err)
	}
	if err := s.store.Put("connections", cfg.ID, encryptedCfg); err != nil {
		return fmt.Errorf("保存连接配置失败: %w", err)
	}
	return nil
}

// GetConnections 获取所有连接配置
func (s *ConnectionService) GetConnections() ([]database.ConnectionConfig, error) {
	items, err := s.store.List("connections", func() interface{} {
		return &database.ConnectionConfig{}
	})
	if err != nil {
		logger.Error("[ConnectionService] 获取连接列表失败: %v", err)
		return nil, fmt.Errorf("获取连接列表失败: %w", err)
	}

	var conns []database.ConnectionConfig
	for _, item := range items {
		cfg := *item.(*database.ConnectionConfig)
		decrypted, err := decryptConnectionConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("解密连接配置失败(id=%s): %w", cfg.ID, err)
		}
		// 仅解密返回；加密迁移不在读路径触发，避免读操作引发意外的写库。
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
	s.migrateConnectionEncryptionIfNeeded(cfg)

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

// ExportConnections 导出所有连接配置为 JSON 备份文件（密码为明文，便于跨设备恢复）
func (s *ConnectionService) ExportConnections() (string, error) {
	conns, err := s.GetConnections()
	if err != nil {
		return "", err
	}
	filePath, err := s.getConnectionsExportPath()
	if err != nil {
		return "", err
	}
	if filePath == "" {
		logger.Info("[ConnectionService] 用户取消了导出路径选择")
		return "", nil
	}

	backup := ConnectionsBackup{
		Version:     connectionsBackupVersion,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		Connections: conns,
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化连接配置失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}
	logger.Info("[ConnectionService] 导出连接配置成功: path=%s count=%d", filePath, len(conns))
	return filePath, nil
}

// ImportConnections 从 JSON 备份文件导入连接配置（同 ID 已存在则跳过）
func (s *ConnectionService) ImportConnections() (ImportConnectionsResult, error) {
	result := ImportConnectionsResult{}
	filePath, err := s.getConnectionsImportPath()
	if err != nil {
		return result, err
	}
	if filePath == "" {
		logger.Info("[ConnectionService] 用户取消了导入文件选择")
		return result, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return result, fmt.Errorf("读取备份文件失败: %w", err)
	}

	var backup ConnectionsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return result, fmt.Errorf("解析备份文件失败: %w", err)
	}
	if len(backup.Connections) == 0 {
		return result, fmt.Errorf("备份文件中没有连接配置")
	}

	for _, cfg := range backup.Connections {
		if cfg.ID == "" {
			result.Skipped++
			continue
		}
		if s.connectionExists(cfg.ID) {
			result.Skipped++
			continue
		}
		if err := s.SaveConnection(cfg); err != nil {
			return result, fmt.Errorf("导入连接 %s 失败: %w", cfg.Name, err)
		}
		result.Imported++
	}

	logger.Info("[ConnectionService] 导入连接配置完成: imported=%d skipped=%d path=%s", result.Imported, result.Skipped, filePath)
	return result, nil
}

func (s *ConnectionService) connectionExists(id string) bool {
	var cfg database.ConnectionConfig
	return s.store.Get("connections", id, &cfg) == nil
}

func (s *ConnectionService) getConnectionsExportPath() (string, error) {
	if s.app != nil {
		return s.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
			Title:    "导出连接配置",
			Filename: fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405")),
		}).PromptForSingleSelection()
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop",
		fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405"))), nil
}

func (s *ConnectionService) getConnectionsImportPath() (string, error) {
	if s.app != nil {
		return s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
			Title:            "导入连接配置",
			CanChooseFiles:   true,
			CanChooseDirectories: false,
			Filters: []application.FileFilter{
				{DisplayName: "JSON 文件", Pattern: "*.json"},
			},
		}).PromptForSingleSelection()
	}
	return "", fmt.Errorf("应用未初始化，无法打开文件选择对话框")
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

// migrateConnectionEncryptionIfNeeded 在连接成功使用前，把仍是明文的密码加密落库。
// 仅在确实需要迁移时触发，写库失败会被记录而不会被静默吞掉。
func (s *ConnectionService) migrateConnectionEncryptionIfNeeded(cfg database.ConnectionConfig) {
	if cfg.Password == "" || storage.IsEncryptedString(cfg.Password) {
		return
	}
	encryptedCfg, err := encryptConnectionConfig(cfg)
	if err != nil {
		logger.Warn("[ConnectionService] 加密迁移失败: id=%s err=%v", cfg.ID, err)
		return
	}
	if err := s.store.Put("connections", encryptedCfg.ID, encryptedCfg); err != nil {
		logger.Warn("[ConnectionService] 加密迁移落库失败: id=%s err=%v", cfg.ID, err)
	}
}
