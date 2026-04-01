package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"tableplus-ai/internal/logger"

	openai "github.com/sashabaranov/go-openai"
)

// Config AI 配置
type Config struct {
	BaseURL      string            `json:"baseURL"`
	APIKey       string            `json:"apiKey"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"systemPrompt"`
	MaxTokens    int               `json:"maxTokens"`
	Temperature  float64           `json:"temperature"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// headerTransport 自定义 HTTP Transport，注入额外请求头
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
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
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	finalSystemPrompt := systemPrompt
	if strings.TrimSpace(c.config.SystemPrompt) != "" {
		// 关键节点：全局会话提示词统一注入，确保所有 AI 能力遵循同一约束
		finalSystemPrompt = finalSystemPrompt + "\n\n用户自定义会话提示词:\n" + strings.TrimSpace(c.config.SystemPrompt)
	}

	logger.Info("[AI] Chat 请求: model=%s baseURL=%s userMessage_len=%d", c.config.Model, c.config.BaseURL, len(userMessage))
	logger.Debug("[AI] Chat systemPrompt(截断): %s", truncateStr(finalSystemPrompt, 200))
	logger.Debug("[AI] Chat userMessage: %s", truncateStr(userMessage, 300))

	temp := float32(c.config.Temperature)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: finalSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		MaxTokens:   c.config.MaxTokens,
		Temperature: temp,
	})
	if err != nil {
		logger.Error("[AI] Chat 请求失败: %v", err)
		return "", fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		logger.Error("[AI] Chat 返回空结果")
		return "", fmt.Errorf("AI 返回空结果")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Info("[AI] Chat 成功: response_len=%d tokens={prompt=%d, completion=%d}",
		len(result), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	logger.Debug("[AI] Chat 响应: %s", truncateStr(result, 500))
	return result, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...(截断)"
	}
	return s
}

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatWithMessages 支持多轮对话的聊天接口
func (c *Client) ChatWithMessages(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	var msgs []openai.ChatCompletionMessage
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem, Content: systemPrompt,
	})
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    msgs,
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
	}

	// 记录请求详情
	logger.Info("[AI] ChatWithMessages 请求: model=%s baseURL=%s messages_count=%d maxTokens=%d temperature=%.2f",
		c.config.Model, c.config.BaseURL, len(msgs), c.config.MaxTokens, c.config.Temperature)
	for i, m := range msgs {
		contentPreview := m.Content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "...(截断)"
		}
		logger.Debug("[AI] 消息[%d] role=%s content=%s", i, m.Role, contentPreview)
	}

	// 序列化请求体用于调试
	if reqJSON, err := json.Marshal(req); err == nil {
		if len(reqJSON) > 2000 {
			logger.Debug("[AI] 请求体(截断): %s...", string(reqJSON[:2000]))
		} else {
			logger.Debug("[AI] 请求体: %s", string(reqJSON))
		}
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Error("[AI] ChatWithMessages 请求失败: %v", err)
		return "", fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		logger.Error("[AI] ChatWithMessages 返回空结果, usage=%+v", resp.Usage)
		return "", fmt.Errorf("AI 返回空结果")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Info("[AI] ChatWithMessages 成功: response_len=%d usage={prompt_tokens=%d, completion_tokens=%d, total_tokens=%d}",
		len(result), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	if len(result) > 500 {
		logger.Debug("[AI] 响应内容(截断): %s...", result[:500])
	} else {
		logger.Debug("[AI] 响应内容: %s", result)
	}

	return result, nil
}

// ChatWithMessagesStream 支持流式返回的聊天接口
func (c *Client) ChatWithMessagesStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	onDelta func(delta string),
) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	var msgs []openai.ChatCompletionMessage
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem, Content: systemPrompt,
	})
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    msgs,
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
		Stream:      true,
	}

	logger.Info("[AI] ChatWithMessagesStream 请求: model=%s baseURL=%s messages_count=%d",
		c.config.Model, c.config.BaseURL, len(msgs))

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		logger.Error("[AI] ChatWithMessagesStream 创建流失败: %v", err)
		return "", fmt.Errorf("AI 流式请求失败: %w", err)
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		response, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			logger.Error("[AI] ChatWithMessagesStream 接收失败: %v", recvErr)
			return "", fmt.Errorf("AI 流式响应失败: %w", recvErr)
		}
		if len(response.Choices) == 0 {
			continue
		}

		delta := response.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		sb.WriteString(delta)
		if onDelta != nil {
			onDelta(delta)
		}
	}

	result := strings.TrimSpace(sb.String())
	logger.Info("[AI] ChatWithMessagesStream 成功: response_len=%d", len(result))
	return result, nil
}
