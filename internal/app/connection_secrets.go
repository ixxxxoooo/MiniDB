package app

import (
	"crypto/subtle"

	"minidb/internal/database"
	"minidb/internal/logger"

	"minidb/internal/storage"
)

// storedPassword 读取已保存的（解密后的）密码；没有保存过返回空串。
func (s *ConnectionService) storedPassword(id string) string {
	if id == "" {
		return ""
	}
	var stored database.ConnectionConfig
	if err := s.store.Get("connections", id, &stored); err != nil {
		return ""
	}
	password, err := s.store.DecryptString(stored.Password)
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
	secret, err := s.store.DecryptString(stored.SSH.Secret)
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

func encryptConnectionConfig(store *storage.Store, cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	encryptedPassword, err := store.EncryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = encryptedPassword
	encryptedSecret, err := store.EncryptString(cfg.SSH.Secret)
	if err != nil {
		return cfg, err
	}
	cfg.SSH.Secret = encryptedSecret
	return cfg, nil
}

func decryptConnectionConfig(store *storage.Store, cfg database.ConnectionConfig) (database.ConnectionConfig, error) {
	password, err := store.DecryptString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = password
	secret, err := store.DecryptString(cfg.SSH.Secret)
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
	encryptedCfg, err := encryptConnectionConfig(s.store, cfg)
	if err != nil {
		logger.Warn("[ConnectionService] 加密迁移失败: id=%s err=%v", cfg.ID, err)
		return
	}
	if err := s.store.Put("connections", encryptedCfg.ID, encryptedCfg); err != nil {
		logger.Warn("[ConnectionService] 加密迁移落库失败: id=%s err=%v", cfg.ID, err)
	}
}

// VerifyPassword 校验用户输入的密码是否与该连接保存的数据库密码一致（供 Safe Mode 使用）。
// 明文密码不离开应用层。没有保存过密码时返回 false。
func (s *ConnectionService) VerifyPassword(id, password string) bool {
	defer logger.Perf("app.Connections.VerifyPassword")()
	stored := s.storedPassword(id)
	if stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
}

// NeedsPassword 判断连接时是否要先向用户询问密码。
func NeedsPassword(cfg database.ConnectionConfig) bool {
	return cfg.PasswordMode == PasswordAsk && !database.CapsOf(cfg.Type).LocalFile()
}
