package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Config AI 配置
type Config struct {
	BaseURL     string  `json:"baseURL"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
}

// Client AI 客户端
type Client struct {
	config *Config
}

// NewClient 创建 AI 客户端
func NewClient(cfg *Config) *Client {
	return &Client{config: cfg}
}

// UpdateConfig 更新配置
func (c *Client) UpdateConfig(cfg *Config) {
	c.config = cfg
}

// Chat 发送聊天请求
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	if c.config == nil || c.config.APIKey == "" {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 API Key")
	}

	clientConfig := openai.DefaultConfig(c.config.APIKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)

	temp := float32(c.config.Temperature)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		MaxTokens:   c.config.MaxTokens,
		Temperature: temp,
	})
	if err != nil {
		return "", fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 返回空结果")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
