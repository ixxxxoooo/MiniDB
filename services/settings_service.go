package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"minidb/internal/ai"
	"minidb/internal/logger"
	"minidb/internal/storage"
	"minidb/internal/updater"
	appversion "minidb/internal/version"
)

// AIConfig AI 配置
type AIConfig struct {
	BaseURL      string            `json:"baseURL"`
	APIKey       string            `json:"apiKey"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"systemPrompt"`
	MaxTokens    int               `json:"maxTokens"`
	Temperature  float64           `json:"temperature"`
	Headers      map[string]string `json:"headers,omitempty"`
}

type AppInfo struct {
	Name       string `json:"name"`
	Company    string `json:"company"`
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"buildDate"`
	Repository string `json:"repository"`
	ReleaseURL string `json:"releaseUrl"`
}

type AnalyticsConfig struct {
	Enabled        bool   `json:"enabled"`
	InstallationID string `json:"installationId"`
	AppVersion     string `json:"appVersion"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
}

// SettingsService 设置服务
type SettingsService struct {
	store   *storage.Store
	updater *updater.Manager
}

// NewSettingsService 创建设置服务
func NewSettingsService(store *storage.Store) *SettingsService {
	return &SettingsService{store: store}
}

//wails:ignore
func (s *SettingsService) SetUpdater(manager *updater.Manager) {
	s.updater = manager
	manager.SetSettingsProvider(s.loadUpdateSettings)
}

// GetAIConfig 获取 AI 配置
func (s *SettingsService) GetAIConfig() (*AIConfig, error) {
	var cfg AIConfig
	err := s.store.Get("settings", "ai_config", &cfg)
	if err != nil {
		return &AIConfig{
			BaseURL:      "https://api.openai.com/v1",
			Model:        "gpt-4o",
			SystemPrompt: "请使用简体中文回答。对于数据库问题优先给出可执行 SQL，并简要说明关键风险与注意事项。",
		}, nil
	}
	needsMigration := aiConfigNeedsEncryption(cfg.APIKey, cfg.Headers)
	if err := decryptAIConfig(&cfg); err != nil {
		return nil, err
	}
	if needsMigration {
		if encryptedCfg := cfg; encryptAIConfig(&encryptedCfg) == nil {
			_ = s.store.Put("settings", "ai_config", encryptedCfg)
		}
	}
	return &cfg, nil
}

// SaveAIConfig 保存 AI 配置到后端存储
func (s *SettingsService) SaveAIConfig(cfg AIConfig) error {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "请使用简体中文回答。对于数据库问题优先给出可执行 SQL，并简要说明关键风险与注意事项。"
	}
	logger.Info("[SettingsService] 保存 AI 配置: baseURL=%s model=%s systemPrompt_len=%d", cfg.BaseURL, cfg.Model, len(cfg.SystemPrompt))
	encryptedCfg := cfg
	if err := encryptAIConfig(&encryptedCfg); err != nil {
		return err
	}
	return s.store.Put("settings", "ai_config", encryptedCfg)
}

// TestAI 测试 AI 连接是否可用
func (s *SettingsService) TestAI(cfg AIConfig) (string, error) {
	logger.Info("[SettingsService] 测试 AI 连接: baseURL=%s model=%s header_count=%d", cfg.BaseURL, cfg.Model, len(cfg.Headers))
	client := ai.NewClient(&ai.Config{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Headers: cfg.Headers,
	})
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant."
	}
	result, usedResponses, err := client.TestResponsesCapability(context.Background(), systemPrompt, "Say 'Hello! AI connection successful.' in one short sentence.")
	if err != nil {
		logger.Error("[SettingsService] AI 测试失败: %v", err)
		return "", fmt.Errorf("AI 连接测试失败: %v", err)
	}
	if usedResponses {
		logger.Info("[SettingsService] AI 测试成功: responses capability ok")
		return result + "\n\nResponses API: available", nil
	}
	logger.Info("[SettingsService] AI 测试成功: chat fallback")
	return result + "\n\nResponses API: unavailable, using Chat Completions fallback", nil
}

func (s *SettingsService) GetAppInfo() AppInfo {
	return AppInfo{
		Name:       appversion.AppName,
		Company:    appversion.CompanyName,
		Version:    appversion.CurrentVersion(),
		Commit:     appversion.Commit,
		BuildDate:  appversion.BuildDate,
		Repository: appversion.Repository,
		ReleaseURL: appversion.ReleasePageURL(),
	}
}

// UpdateSettings 是用户可调的自动更新偏好。
type UpdateSettings struct {
	AutoCheck      bool   `json:"autoCheck"`
	AutoDownload   bool   `json:"autoDownload"`
	SkippedVersion string `json:"skippedVersion"`
}

const updateSettingsKey = "update_settings"

func (s *SettingsService) loadUpdateSettings() updater.Settings {
	defaults := updater.DefaultSettings()
	var stored UpdateSettings
	if err := s.store.Get("settings", updateSettingsKey, &stored); err != nil {
		return defaults
	}
	return updater.Settings{AutoCheck: stored.AutoCheck, AutoDownload: stored.AutoDownload, SkippedVersion: stored.SkippedVersion}
}

// GetUpdateSettings 获取自动更新偏好（默认：自动检查、自动下载）。
func (s *SettingsService) GetUpdateSettings() UpdateSettings {
	cfg := s.loadUpdateSettings()
	return UpdateSettings{AutoCheck: cfg.AutoCheck, AutoDownload: cfg.AutoDownload, SkippedVersion: cfg.SkippedVersion}
}

// SaveUpdateSettings 保存自动更新偏好。SkippedVersion 由 SkipUpdateVersion 维护，这里保留原值。
func (s *SettingsService) SaveUpdateSettings(cfg UpdateSettings) error {
	current := s.loadUpdateSettings()
	cfg.SkippedVersion = current.SkippedVersion
	return s.store.Put("settings", updateSettingsKey, cfg)
}

// SkipUpdateVersion 跳过某个版本：自动检查不再提示它，直到出现更新的版本。
func (s *SettingsService) SkipUpdateVersion(version string) error {
	current := s.loadUpdateSettings()
	if err := s.store.Put("settings", updateSettingsKey, UpdateSettings{
		AutoCheck: current.AutoCheck, AutoDownload: current.AutoDownload, SkippedVersion: version,
	}); err != nil {
		return err
	}
	if s.updater != nil {
		s.updater.SkipVersion(version)
	}
	return nil
}

func (s *SettingsService) CheckForUpdates() error {
	if s.updater == nil {
		return fmt.Errorf("updater not initialized")
	}
	s.updater.CheckNow()
	return nil
}

// DownloadUpdate 开始下载已发现的新版本（关闭自动下载时使用）。
func (s *SettingsService) DownloadUpdate() error {
	if s.updater == nil {
		return fmt.Errorf("updater not initialized")
	}
	return s.updater.DownloadUpdate()
}

func (s *SettingsService) GetUpdateStatus() updater.StatePayload {
	if s.updater == nil {
		return updater.StatePayload{State: "idle"}
	}
	return s.updater.Snapshot()
}

func (s *SettingsService) InstallReadyUpdate() error {
	if s.updater == nil {
		return fmt.Errorf("updater not initialized")
	}
	return s.updater.InstallReadyUpdate()
}

// GetAnalyticsConfig 获取匿名统计配置，并确保本机匿名安装 ID 已生成。
func (s *SettingsService) GetAnalyticsConfig() (*AnalyticsConfig, error) {
	cfg, err := s.loadAnalyticsConfig(true)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveAnalyticsConfig 保存匿名统计开关。安装 ID 只由后端生成并持久化。
func (s *SettingsService) SaveAnalyticsConfig(cfg AnalyticsConfig) error {
	current, err := s.loadAnalyticsConfig(true)
	if err != nil {
		return err
	}
	current.Enabled = cfg.Enabled
	return s.store.Put("settings", "analytics_config", analyticsStoredConfig{
		Enabled:        current.Enabled,
		InstallationID: current.InstallationID,
	})
}

type analyticsStoredConfig struct {
	Enabled        bool   `json:"enabled"`
	InstallationID string `json:"installationId"`
}

func (s *SettingsService) loadAnalyticsConfig(ensureID bool) (*AnalyticsConfig, error) {
	var stored analyticsStoredConfig
	if err := s.store.Get("settings", "analytics_config", &stored); err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		return nil, err
	}
	changed := false
	if ensureID && stored.InstallationID == "" {
		id, err := newInstallationID()
		if err != nil {
			return nil, err
		}
		stored.InstallationID = id
		changed = true
	}
	if changed {
		if err := s.store.Put("settings", "analytics_config", stored); err != nil {
			return nil, err
		}
	}
	return &AnalyticsConfig{
		Enabled:        stored.Enabled,
		InstallationID: stored.InstallationID,
		AppVersion:     appversion.CurrentVersion(),
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
	}, nil
}

func newInstallationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "minidb_" + hex.EncodeToString(b[:]), nil
}

func encryptAIConfig(cfg *AIConfig) error {
	apiKey, err := storage.EncryptString(cfg.APIKey)
	if err != nil {
		return err
	}
	cfg.APIKey = apiKey
	if len(cfg.Headers) > 0 {
		headers := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			encrypted, err := storage.EncryptString(v)
			if err != nil {
				return err
			}
			headers[k] = encrypted
		}
		cfg.Headers = headers
	}
	return nil
}

func decryptAIConfig(cfg *AIConfig) error {
	apiKey, err := storage.DecryptString(cfg.APIKey)
	if err != nil {
		return err
	}
	cfg.APIKey = apiKey
	if len(cfg.Headers) > 0 {
		headers := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			decrypted, err := storage.DecryptString(v)
			if err != nil {
				return err
			}
			headers[k] = decrypted
		}
		cfg.Headers = headers
	}
	return nil
}

func decryptAIClientConfig(cfg *ai.Config) error {
	apiKey, err := storage.DecryptString(cfg.APIKey)
	if err != nil {
		return err
	}
	cfg.APIKey = apiKey
	if len(cfg.Headers) > 0 {
		headers := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			decrypted, err := storage.DecryptString(v)
			if err != nil {
				return err
			}
			headers[k] = decrypted
		}
		cfg.Headers = headers
	}
	return nil
}

func encryptAIClientConfig(cfg *ai.Config) error {
	apiKey, err := storage.EncryptString(cfg.APIKey)
	if err != nil {
		return err
	}
	cfg.APIKey = apiKey
	if len(cfg.Headers) > 0 {
		headers := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			encrypted, err := storage.EncryptString(v)
			if err != nil {
				return err
			}
			headers[k] = encrypted
		}
		cfg.Headers = headers
	}
	return nil
}

func aiConfigNeedsEncryption(apiKey string, headers map[string]string) bool {
	if apiKey != "" && !storage.IsEncryptedString(apiKey) {
		return true
	}
	for _, v := range headers {
		if v != "" && !storage.IsEncryptedString(v) {
			return true
		}
	}
	return false
}

// GetLogContent 读取当前日志文件内容（最后 500 行）
// 从文件末尾向前只读最后一部分，避免把整个日志文件加载进内存。
func (s *SettingsService) GetLogContent() (string, error) {
	logPath := logger.CurrentPath()
	file, err := os.Open(logPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}
	defer file.Close()

	const tailBytes = 256 * 1024 // 256KB 窗口足够覆盖 500 行
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}
	buf := make([]byte, min(int64(tailBytes), info.Size()))
	start := info.Size() - int64(len(buf))
	if _, err := file.ReadAt(buf, start); err != nil && len(buf) > 0 {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	content := string(buf)
	lines := strings.Split(content, "\n")
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	return strings.Join(lines, "\n"), nil
}

// GetLogPath 获取日志文件路径
func (s *SettingsService) GetLogPath() string {
	return logger.CurrentPath()
}
