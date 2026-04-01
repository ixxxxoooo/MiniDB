package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
	"time"
)

// AIConfig AI 配置
type AIConfig struct {
	BaseURL     string            `json:"baseURL"`
	APIKey      string            `json:"apiKey"`
	Model       string            `json:"model"`
	MaxTokens   int               `json:"maxTokens"`
	Temperature float64           `json:"temperature"`
	Headers     map[string]string `json:"headers,omitempty"`
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
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-4o",
			MaxTokens:   4096,
			Temperature: 0.3,
		}, nil
	}
	return &cfg, nil
}

// SaveAIConfig 保存 AI 配置到后端存储
func (s *SettingsService) SaveAIConfig(cfg AIConfig) error {
	// 兼容前端仅提交部分字段的场景，避免把 maxTokens/temperature 覆盖成 0
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.3
	}
	logger.Info("[SettingsService] 保存 AI 配置: baseURL=%s model=%s", cfg.BaseURL, cfg.Model)
	return s.store.Put("settings", "ai_config", cfg)
}

// TestAI 测试 AI 连接是否可用
func (s *SettingsService) TestAI(cfg AIConfig) (string, error) {
	logger.Info("[SettingsService] 测试 AI 连接: baseURL=%s model=%s headers=%v", cfg.BaseURL, cfg.Model, cfg.Headers)
	client := ai.NewClient(&ai.Config{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		MaxTokens:   100,
		Temperature: float64(cfg.Temperature),
		Headers:     cfg.Headers,
	})
	result, err := client.Chat(context.Background(), "You are a helpful assistant.", "Say 'Hello! AI connection successful.' in one short sentence.")
	if err != nil {
		logger.Error("[SettingsService] AI 测试失败: %v", err)
		return "", fmt.Errorf("AI 连接测试失败: %v", err)
	}
	logger.Info("[SettingsService] AI 测试成功: %s", result)
	return result, nil
}

// GetLogContent 读取当前日志文件内容（最后 500 行）
func (s *SettingsService) GetLogContent() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %w", err)
	}

	logPath := filepath.Join(homeDir, ".tableplus-ai", "logs", time.Now().Format("2006-01-02")+".log")
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
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".tableplus-ai", "logs", time.Now().Format("2006-01-02")+".log")
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
