package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"

	"minidb/internal/ai"
	"minidb/internal/appdata"
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

func (s *SettingsService) CheckForUpdates() error {
	if s.updater == nil {
		return fmt.Errorf("更新管理器未初始化")
	}
	s.updater.CheckNow(true)
	return nil
}

func (s *SettingsService) GetUpdateStatus() updater.StatePayload {
	if s.updater == nil {
		return updater.StatePayload{State: "idle"}
	}
	return s.updater.Snapshot()
}

func (s *SettingsService) InstallReadyUpdate() error {
	if s.updater == nil {
		return fmt.Errorf("更新管理器未初始化")
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
func (s *SettingsService) GetLogContent() (string, error) {
	logPath := appdata.LogFilePath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	content := string(data)
	// 只返回最后部分，避免内容过大
	lines := splitLines(content)
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	return joinLines(lines), nil
}

// GetLogPath 获取日志文件路径
func (s *SettingsService) GetLogPath() string {
	return appdata.LogFilePath()
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

// SavePageSize 保存分页大小设置
func (s *SettingsService) SavePageSize(size int) error {
	return s.store.Put("settings", "page_size", size)
}

// GetPageSize 获取分页大小设置
func (s *SettingsService) GetPageSize() int {
	var size int
	err := s.store.Get("settings", "page_size", &size)
	if err != nil || size <= 0 {
		return 100
	}
	return size
}
