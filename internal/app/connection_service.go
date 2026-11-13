package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"
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
	env     *Env
	store   *storage.Store
	manager *database.Manager
	schema  *schemaindex.Manager

	keepalive *keepalive
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager, schema *schemaindex.Manager) *ConnectionService {
	return &ConnectionService{store: store, manager: manager, schema: schema}
}

// SetWailsApplication 设置 Wails 应用实例（在 startup 时调用）
func (s *ConnectionService) SetEnv(env *Env) {
	s.env = env
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
	if cfg.SSH.Enabled {
		if cfg.Type == "sqlite" {
			return fmt.Errorf("SQLite 不支持 SSH 隧道")
		}
		if strings.TrimSpace(cfg.SSH.Host) == "" || strings.TrimSpace(cfg.SSH.User) == "" {
			return fmt.Errorf("SSH 主机和用户名不能为空")
		}
		if cfg.SSH.Port < 1 || cfg.SSH.Port > 65535 {
			return fmt.Errorf("SSH 端口号不合法: %d（范围 1-65535）", cfg.SSH.Port)
		}
		if cfg.SSH.AuthType == "key" && strings.TrimSpace(cfg.SSH.KeyPath) == "" {
			return fmt.Errorf("请选择 SSH 私钥文件")
		}
	}
	return nil
}

// storedPassword 读取已保存的（解密后的）密码；没有保存过返回空串。
func (s *ConnectionService) storedPassword(id string) string {
	if id == "" {
		return ""
	}
	var stored database.ConnectionConfig
	if err := s.store.Get("connections", id, &stored); err != nil {
		return ""
	}
	password, err := storage.DecryptString(stored.Password)
	if err != nil {
		return ""
	}
	return password
}

// resolvePassword 解析本次操作实际使用的密码：
// 前端拿不到明文密码，所以"密码留空"表示沿用已保存的密码，ClearPassword 表示显式清除。
func (s *ConnectionService) resolvePassword(cfg database.ConnectionConfig) string {
	if cfg.ClearPassword {
		return ""
	}
	if cfg.Password != "" {
		return cfg.Password
	}
	return s.storedPassword(cfg.ID)
}

// storedSSHSecret 读取已保存的（解密后的）SSH 密码/私钥口令。
func (s *ConnectionService) storedSSHSecret(id string) string {
	if id == "" {
		return ""
	}
	var stored database.ConnectionConfig
	if err := s.store.Get("connections", id, &stored); err != nil {
		return ""
	}
	secret, err := storage.DecryptString(stored.SSH.Secret)
	if err != nil {
		return ""
	}
	return secret
}

// resolveSSHSecret 与 resolvePassword 同理：Secret 留空沿用已保存的，ClearSecret 显式清除。
func (s *ConnectionService) resolveSSHSecret(cfg database.ConnectionConfig) string {
	if cfg.SSH.ClearSecret {
		return ""
	}
	if cfg.SSH.Secret != "" {
		return cfg.SSH.Secret
	}
	return s.storedSSHSecret(cfg.ID)
}

// SaveConnection 保存连接配置到本地存储
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	if cfg.ID == "" { // a new connection: the store keys it by a fresh random ID
		cfg.ID = newConnectionID()
	}
	if err := validateConnectionConfig(cfg); err != nil {
		return fmt.Errorf("连接配置校验失败: %w", err)
	}
	logger.Info("[ConnectionService] 保存连接配置: name=%s type=%s", cfg.Name, cfg.Type)
	cfg.Password = s.resolvePassword(cfg)
	cfg.HasPassword, cfg.ClearPassword = false, false
	cfg.SSH.Secret = s.resolveSSHSecret(cfg)
	cfg.SSH.HasSecret, cfg.SSH.ClearSecret = false, false
	encryptedCfg, err := encryptConnectionConfig(cfg)
	if err != nil {
		return fmt.Errorf("加密连接配置失败: %w", err)
	}
	if err := s.store.Put("connections", cfg.ID, encryptedCfg); err != nil {
		return fmt.Errorf("保存连接配置失败: %w", err)
	}
	return nil
}

// loadConnectionsWithSecrets 读取所有连接（含解密后的密码）。仅供后端内部使用（如备份导出），
// 不能直接暴露给前端。
func (s *ConnectionService) loadConnectionsWithSecrets() ([]database.ConnectionConfig, error) {
	items, err := s.store.List("connections", func() interface{} {
		return &database.ConnectionConfig{}
	})
	if err != nil {
		return nil, fmt.Errorf("获取连接列表失败: %w", err)
	}
	conns := make([]database.ConnectionConfig, 0, len(items))
	for _, item := range items {
		cfg := *item.(*database.ConnectionConfig)
		decrypted, err := decryptConnectionConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("解密连接配置失败(id=%s): %w", cfg.ID, err)
		}
		conns = append(conns, decrypted)
	}
	return conns, nil
}

// GetConnections 获取所有连接配置。出于安全考虑不返回明文密码，
// 前端通过 HasPassword 判断是否已保存密码；连接时由后端按 ID 读取密码。
func (s *ConnectionService) GetConnections() ([]database.ConnectionConfig, error) {
	// Listing needs no secrets, so nothing is decrypted: a lost or wrong key
	// cannot blank the list, and a connection fails alone when it is opened.
	items, err := s.store.List("connections", func() interface{} { return &database.ConnectionConfig{} })
	if err != nil {
		logger.Error("[ConnectionService] 获取连接列表失败: %v", err)
		return nil, fmt.Errorf("获取连接列表失败: %w", err)
	}
	conns := make([]database.ConnectionConfig, 0, len(items))
	for _, item := range items {
		cfg := *item.(*database.ConnectionConfig)
		cfg.HasPassword = cfg.Password != ""
		cfg.Password = ""
		cfg.SSH.HasSecret = cfg.SSH.Secret != ""
		cfg.SSH.Secret = ""
		conns = append(conns, cfg)
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
	cfg.Password = s.resolvePassword(cfg)
	cfg.SSH.Secret = s.resolveSSHSecret(cfg)
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

// ExportConnections 导出所有连接配置为备份文件。
// passphrase 非空时整个文件用口令加密（AES-256-GCM + PBKDF2）；为空时密码以明文写入，仅建议临时使用。
func (s *ConnectionService) ExportConnections(passphrase string) (string, error) {
	conns, err := s.loadConnectionsWithSecrets()
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
	if passphrase != "" {
		if data, err = encryptBackup(data, passphrase); err != nil {
			return "", fmt.Errorf("加密备份失败: %w", err)
		}
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}
	logger.Info("[ConnectionService] 导出连接配置成功: path=%s count=%d", filePath, len(conns))
	return filePath, nil
}

// ImportConnections 从备份文件导入连接配置（同 ID 已存在则跳过）。
// 备份已加密时需要提供 passphrase。
func (s *ConnectionService) ImportConnections(passphrase string) (ImportConnectionsResult, error) {
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

	if isEncryptedBackup(data) {
		if data, err = decryptBackup(data, passphrase); err != nil {
			return result, err
		}
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
	if s.env != nil && s.env.Host != nil {
		return s.env.Host.SaveFile(SaveOptions{
			Title:    "导出连接配置",
			Filename: fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405")),
		})
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop",
		fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405"))), nil
}

// PickSSHKeyFile 弹出文件选择框选择 SSH 私钥，默认打开 ~/.ssh；用户取消返回空串。
func (s *ConnectionService) PickSSHKeyFile() (string, error) {
	if s.env == nil || s.env.Host == nil {
		return "", fmt.Errorf("应用未初始化，无法打开文件选择对话框")
	}
	dir := ""
	if home, err := os.UserHomeDir(); err == nil {
		dir = filepath.Join(home, ".ssh")
	}
	return s.env.Host.OpenFile(OpenOptions{
		Title:      "选择 SSH 私钥文件",
		ShowHidden: true,
		Directory:  dir,
	})
}

func (s *ConnectionService) getConnectionsImportPath() (string, error) {
	if s.env != nil && s.env.Host != nil {
		return s.env.Host.OpenFile(OpenOptions{
			Title:   "导入连接配置",
			Filters: []FileFilter{{Name: "JSON 文件", Exts: []string{"json"}}},
		})
	}
	return "", fmt.Errorf("应用未初始化，无法打开文件选择对话框")
}

func encryptConnectionConfig(cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	encryptedPassword, err := storage.EncryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = encryptedPassword
	encryptedSecret, err := storage.EncryptString(cfg.SSH.Secret)
	if err != nil {
		return cfg, err
	}
	cfg.SSH.Secret = encryptedSecret
	return cfg, nil
}

func decryptConnectionConfig(cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	password, err := storage.DecryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = password
	secret, err := storage.DecryptString(cfg.SSH.Secret)
	if err != nil {
		return cfg, err
	}
	cfg.SSH.Secret = secret
	return cfg, nil
}

// migrateConnectionEncryptionIfNeeded 在连接成功使用前，把仍是明文的密码加密落库。
// 仅在确实需要迁移时触发，写库失败会被记录而不会被静默吞掉。
func (s *ConnectionService) migrateConnectionEncryptionIfNeeded(cfg database.ConnectionConfig) {
	if (cfg.Password == "" || storage.IsEncryptedString(cfg.Password)) && (cfg.SSH.Secret == "" || storage.IsEncryptedString(cfg.SSH.Secret)) {
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

// newConnectionID returns a random 128-bit hex ID.
func newConnectionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("c%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
