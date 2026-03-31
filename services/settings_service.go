package services

import (
	"context"
	"fmt"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
)

// AIConfig AI 配置
type AIConfig struct {
	BaseURL     string  `json:"baseURL"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
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
	logger.Info("[SettingsService] 保存 AI 配置: baseURL=%s model=%s", cfg.BaseURL, cfg.Model)
	return s.store.Put("settings", "ai_config", cfg)
}

// TestAI 测试 AI 连接是否可用
func (s *SettingsService) TestAI(cfg AIConfig) (string, error) {
	logger.Info("[SettingsService] 测试 AI 连接: baseURL=%s model=%s", cfg.BaseURL, cfg.Model)
	client := ai.NewClient(&ai.Config{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		MaxTokens:   100,
		Temperature: float64(cfg.Temperature),
	})
	result, err := client.Chat(context.Background(), "You are a helpful assistant.", "Say 'Hello! AI connection successful.' in one short sentence.")
	if err != nil {
		logger.Error("[SettingsService] AI 测试失败: %v", err)
		return "", fmt.Errorf("AI 连接测试失败: %v", err)
	}
	logger.Info("[SettingsService] AI 测试成功: %s", result)
	return result, nil
}
