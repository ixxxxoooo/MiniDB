package services

import (
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
		// 返回默认配置
		return &AIConfig{
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-4o",
			MaxTokens:   4096,
			Temperature: 0.3,
		}, nil
	}
	return &cfg, nil
}

// SaveAIConfig 保存 AI 配置
func (s *SettingsService) SaveAIConfig(cfg AIConfig) error {
	return s.store.Put("settings", "ai_config", cfg)
}
