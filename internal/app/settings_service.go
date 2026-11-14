package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"minidb/internal/ai"
	"minidb/internal/logger"
	"minidb/internal/storage"
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

// SettingsService 设置服务
type SettingsService struct {
	store *storage.Store
}

// NewSettingsService 创建设置服务
func NewSettingsService(store *storage.Store) *SettingsService {
	return &SettingsService{store: store}
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
	if err := decryptAIConfig(s.store, &cfg); err != nil {
		return nil, err
	}
	if needsMigration {
		if encryptedCfg := cfg; encryptAIConfig(s.store, &encryptedCfg) == nil {
			if err := s.store.Put("settings", "ai_config", encryptedCfg); err != nil {
				logger.Warn("[SettingsService] 迁移 AI 配置为密文失败（下次读取会重试）: %v", err)
			}
		}
	}
	return &cfg, nil
}

// SaveAIConfig 保存 AI 配置到后端存储
func (s *SettingsService) SaveAIConfig(cfg AIConfig) error {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if err := ValidateAIBaseURL(cfg.BaseURL); err != nil {
		return err
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "请使用简体中文回答。对于数据库问题优先给出可执行 SQL，并简要说明关键风险与注意事项。"
	}
	logger.Info("[SettingsService] 保存 AI 配置: baseURL=%s model=%s systemPrompt_len=%d", cfg.BaseURL, cfg.Model, len(cfg.SystemPrompt))
	encryptedCfg := cfg
	if err := encryptAIConfig(s.store, &encryptedCfg); err != nil {
		return err
	}
	return s.store.Put("settings", "ai_config", encryptedCfg)
}

// TestAI 测试 AI 连接是否可用
func (s *SettingsService) TestAI(cfg AIConfig) (string, error) {
	if err := ValidateAIBaseURL(cfg.BaseURL); err != nil {
		return "", err
	}
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

// mapAISecrets 对 API Key 与每个请求头的值依次套用 f（加密或解密），返回新值；
// 加解密、AIConfig 与 ai.Config 共用这一处遍历，新增敏感字段时只需要改这里。
func mapAISecrets(apiKey string, headers map[string]string, f func(string) (string, error)) (string, map[string]string, error) {
	key, err := f(apiKey)
	if err != nil {
		return "", nil, err
	}
	if len(headers) == 0 {
		return key, headers, nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if out[k], err = f(v); err != nil {
			return "", nil, err
		}
	}
	return key, out, nil
}

func encryptAIConfig(store *storage.Store, cfg *AIConfig) (err error) {
	cfg.APIKey, cfg.Headers, err = mapAISecrets(cfg.APIKey, cfg.Headers, store.EncryptString)
	return err
}

func decryptAIConfig(store *storage.Store, cfg *AIConfig) (err error) {
	cfg.APIKey, cfg.Headers, err = mapAISecrets(cfg.APIKey, cfg.Headers, store.DecryptString)
	return err
}

func decryptAIClientConfig(store *storage.Store, cfg *ai.Config) (err error) {
	cfg.APIKey, cfg.Headers, err = mapAISecrets(cfg.APIKey, cfg.Headers, store.DecryptString)
	return err
}

func encryptAIClientConfig(store *storage.Store, cfg *ai.Config) (err error) {
	cfg.APIKey, cfg.Headers, err = mapAISecrets(cfg.APIKey, cfg.Headers, store.EncryptString)
	return err
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
