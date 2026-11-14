package app

import (
	"crypto/rand"

	"encoding/hex"

	"fmt"
	"minidb/internal/model"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"
)

// ConnectionService 连接管理服务，负责连接的增删改查和连接/断开操作
type ConnectionService struct {
	env     envRef
	store   *storage.Store
	manager *database.Manager
	schema  *schemaindex.Manager

	keepalive      *keepalive
	keepaliveEvery atomic.Int64 // 保活探测间隔（纳秒），0 关闭
}

// NewConnectionService 创建连接服务
func NewConnectionService(store *storage.Store, manager *database.Manager, schema *schemaindex.Manager) *ConnectionService {
	s := &ConnectionService{store: store, manager: manager, schema: schema}
	s.keepaliveEvery.Store(int64(keepaliveInterval))
	return s
}

// SetEnv 设置应用运行环境（在 startup 时调用）
func (s *ConnectionService) SetEnv(env *Env) {
	s.env.set(env)
}

// validateConnectionConfig 校验连接配置必填字段与合法性
func validateConnectionConfig(cfg database.ConnectionConfig) error {
	if cfg.ID == "" {
		return model.Errf(model.CodeConnIDRequired, "连接配置 ID 不能为空")
	}
	if !cfg.Type.Valid() {
		return model.Errf(model.CodeUnsupportedType, "不支持的数据库类型: %s", cfg.Type)
	}
	if database.CapsOf(cfg.Type).Network {
		if cfg.Host == "" {
			return model.Errf(model.CodeHostRequired, "主机地址不能为空")
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			return model.Errf(model.CodePortInvalid, "端口号不合法: %d（范围 1-65535）", cfg.Port)
		}
	}
	if cfg.SSH.Enabled {
		if database.CapsOf(cfg.Type).LocalFile() {
			return model.Errf(model.CodeSQLiteNoSSH, "SQLite 不支持 SSH 隧道")
		}
		if strings.TrimSpace(cfg.SSH.Host) == "" || strings.TrimSpace(cfg.SSH.User) == "" {
			return model.Errf(model.CodeSSHHostUser, "SSH 主机和用户名不能为空")
		}
		if cfg.SSH.Port < 1 || cfg.SSH.Port > 65535 {
			return model.Errf(model.CodeSSHPortInvalid, "SSH 端口号不合法: %d（范围 1-65535）", cfg.SSH.Port)
		}
		if cfg.SSH.AuthType == "key" && strings.TrimSpace(cfg.SSH.KeyPath) == "" {
			return model.Errf(model.CodeSSHKeyRequired, "请选择 SSH 私钥文件")
		}
	}
	return nil
}

// SaveConnection 保存连接配置到本地存储
func (s *ConnectionService) SaveConnection(cfg database.ConnectionConfig) error {
	defer logger.Perf("app.Connections.SaveConnection")()
	if cfg.ID == "" { // a new connection: the store keys it by a fresh random ID
		cfg.ID = newConnectionID()
	}
	if err := validateConnectionConfig(cfg); err != nil {
		return fmt.Errorf("连接配置校验失败: %w", err)
	}
	logger.Info("[ConnectionService] 保存连接配置: name=%s type=%s", cfg.Name, cfg.Type)
	cfg.Password = s.resolvePassword(cfg)
	if cfg.PasswordMode == PasswordAsk || cfg.PasswordMode == PasswordNone {
		cfg.Password = "" // 每次询问或不用密码：不落盘
	}
	cfg.HasPassword, cfg.ClearPassword = false, false
	if !s.connectionExists(cfg.ID) && cfg.Order == 0 {
		cfg.Order = s.nextConnectionOrder(cfg.FolderID)
	}
	cfg.SSH.Secret = s.resolveSSHSecret(cfg)
	cfg.SSH.HasSecret, cfg.SSH.ClearSecret = false, false
	encryptedCfg, err := encryptConnectionConfig(s.store, cfg)
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
		decrypted, err := decryptConnectionConfig(s.store, cfg)
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
	defer logger.Perf("app.Connections.GetConnections")()
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
	SortConnections(conns)
	logger.Info("[ConnectionService] 获取到 %d 个连接配置", len(conns))
	return conns, nil
}

// DeleteConnection 删除连接配置，同时断开对应连接
func (s *ConnectionService) DeleteConnection(id string) error {
	defer logger.Perf("app.Connections.DeleteConnection")()
	logger.Info("[ConnectionService] 删除连接: %s", id)
	if s.schema != nil {
		s.schema.ForgetConnection(id)
	}
	s.manager.Disconnect(id)
	return s.store.Delete("connections", id)
}

// TestConnection 测试连接是否可达；不通时返回驱动给出的错误。
func (s *ConnectionService) TestConnection(cfg database.ConnectionConfig) error {
	defer logger.Perf("app.Connections.TestConnection")()
	logger.Info("[ConnectionService] 测试连接: name=%s", cfg.Name)
	cfg.Password = s.resolvePassword(cfg)
	cfg.SSH.Secret = s.resolveSSHSecret(cfg)
	err := s.manager.TestConnection(&cfg)
	if err != nil {
		logger.Warn("[ConnectionService] 测试连接失败: %v", err)
		return err
	}
	logger.Info("[ConnectionService] 测试连接成功: name=%s", cfg.Name)
	return nil
}

// 密码来源（ConnectionConfig.PasswordMode）。
const (
	PasswordKeychain = "keychain" // 已保存到系统钥匙串（空值同此）
	PasswordAsk      = "ask"      // 每次连接时询问
	PasswordNone     = "none"     // 不使用密码
)

// Connect 根据 ID 从存储加载配置并建立连接
func (s *ConnectionService) Connect(id string) error {
	defer logger.Perf("app.Connections.Connect")()
	return s.ConnectWithPassword(id, "")
}

// ConnectWithPassword 与 Connect 相同；密码来源为"每次询问"时用传入的密码连接。
func (s *ConnectionService) ConnectWithPassword(id, password string) error {
	defer logger.Perf("app.Connections.ConnectWithPassword")()
	logger.Info("[ConnectionService] 正在连接: id=%s", id)
	var cfg database.ConnectionConfig
	if err := s.store.Get("connections", id, &cfg); err != nil {
		logger.Error("[ConnectionService] 加载连接配置失败: id=%s err=%v", id, err)
		return err
	}
	cfg, err := decryptConnectionConfig(s.store, cfg)
	if err != nil {
		logger.Error("[ConnectionService] 解密连接配置失败: id=%s err=%v", id, err)
		return err
	}
	s.migrateConnectionEncryptionIfNeeded(cfg)
	if cfg.PasswordMode == PasswordAsk {
		cfg.Password = password
	}

	if err := s.manager.Connect(&cfg); err != nil {
		logger.Error("[ConnectionService] 连接失败: id=%s err=%v", id, err)
		return err
	}
	if s.schema != nil && cfg.Database != "" {
		s.schema.WarmAsync(id, cfg.Database)
	}
	logger.Info("[ConnectionService] 连接成功: id=%s name=%s", id, cfg.Name)
	return nil
}

// IsConnected 判断连接当前是否有可用的连接池。
func (s *ConnectionService) IsConnected(id string) bool {
	_, err := s.manager.GetDB(id)
	return err == nil
}

// Disconnect 断开指定连接
func (s *ConnectionService) Disconnect(id string) error {
	defer logger.Perf("app.Connections.Disconnect")()
	logger.Info("[ConnectionService] 断开连接: id=%s", id)
	if s.schema != nil {
		s.schema.ForgetConnection(id)
	}
	return s.manager.Disconnect(id)
}

// PickSSHKeyFile 弹出文件选择框选择 SSH 私钥，默认打开 ~/.ssh；用户取消返回空串。
func (s *ConnectionService) PickSSHKeyFile() (string, error) {
	defer logger.Perf("app.Connections.PickSSHKeyFile")()
	if s.env.get() == nil || s.env.get().Host == nil {
		return "", model.Errf(model.CodeAppNotReady, "应用未初始化，无法打开文件选择对话框")
	}
	dir := ""
	if home, err := os.UserHomeDir(); err == nil {
		dir = filepath.Join(home, ".ssh")
	}
	return s.env.get().Host.OpenFile(OpenOptions{
		Title:      "选择 SSH 私钥文件",
		ShowHidden: true,
		Directory:  dir,
	})
}

// newConnectionID returns a random 128-bit hex ID.
func newConnectionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("c%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// DuplicateConnection 复制一个连接（含已保存的密码），新连接用传入的名字；返回新连接 ID。
func (s *ConnectionService) DuplicateConnection(id, name string) (string, error) {
	defer logger.Perf("app.Connections.DuplicateConnection")()
	all, err := s.loadConnectionsWithSecrets()
	if err != nil {
		return "", err
	}
	for _, cfg := range all {
		if cfg.ID != id {
			continue
		}
		cfg.ID, cfg.Name = newConnectionID(), name
		cfg.Order = 0 // 副本排到所在文件夹的末尾
		cfg.HasPassword, cfg.ClearPassword = false, false
		cfg.SSH.HasSecret, cfg.SSH.ClearSecret = false, false
		if err := s.SaveConnection(cfg); err != nil {
			return "", err
		}
		return cfg.ID, nil
	}
	return "", model.Errf(model.CodeConnNotFound, "连接不存在: %s", id)
}
