package ai

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Config AI 服务配置。
type Config struct {
	BaseURL      string            `json:"baseURL"`
	APIKey       string            `json:"apiKey"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"systemPrompt"`
	MaxTokens    int               `json:"maxTokens"`
	Temperature  float64           `json:"temperature"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// Client AI 客户端：主链路用 OpenAI Responses API，仅在网关不支持 /responses 时回退 Chat Completions。
//
// @author ixxxxoooo
type Client struct {
	mu                  sync.Mutex
	config              *Config
	responsesDisabledAt time.Time // 熔断开始时间；零值表示 Responses 可用
	responsesDisabledBy string    // 触发熔断的原因，仅用于日志
}

// responsesCooldown 判定网关不支持 Responses 后，暂时不再尝试它的时长。
const responsesCooldown = 10 * time.Minute

// NewClient 创建 AI 客户端。
func NewClient(cfg *Config) *Client {
	return &Client{config: cfg}
}

// UpdateConfig 更新配置；服务地址、密钥、模型或请求头变化时清除 Responses 熔断状态。
func (c *Client) UpdateConfig(cfg *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !sameProvider(c.config, cfg) {
		c.responsesDisabledAt, c.responsesDisabledBy = time.Time{}, ""
	}
	c.config = cfg
}

// ModelName 返回当前配置的模型名（小写、去空格）。
func (c *Client) ModelName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.config == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.config.Model))
}

// snapshot 返回配置的副本，调用方在一次请求里始终看到同一份配置。
func (c *Client) snapshot() (*Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.config == nil {
		return nil, fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}
	if strings.TrimSpace(c.config.Model) == "" {
		return nil, fmt.Errorf("AI 模型未配置，请先在设置中配置模型")
	}
	cfg := *c.config
	return &cfg, nil
}

// responsesBlocked 报告 Responses 是否仍处于熔断期，以及熔断原因。
func (c *Client) responsesBlocked(now time.Time) (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.responsesDisabledAt.IsZero() {
		return false, ""
	}
	if now.Sub(c.responsesDisabledAt) < responsesCooldown {
		return true, c.responsesDisabledBy
	}
	c.responsesDisabledAt, c.responsesDisabledBy = time.Time{}, ""
	return false, ""
}

// blockResponses 记录网关不支持 Responses，之后一段时间直接走 Chat Completions。
func (c *Client) blockResponses(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responsesDisabledAt = time.Now()
	c.responsesDisabledBy = "responses_unavailable"
	if err != nil {
		c.responsesDisabledBy = err.Error()
	}
}

// sameProvider 判断两份配置指向同一个服务端点（地址、密钥、模型、请求头都相同）。
func sameProvider(a, b *Config) bool {
	if a == nil || b == nil {
		return a == b
	}
	if strings.TrimSpace(a.BaseURL) != strings.TrimSpace(b.BaseURL) ||
		strings.TrimSpace(a.APIKey) != strings.TrimSpace(b.APIKey) ||
		strings.TrimSpace(a.Model) != strings.TrimSpace(b.Model) ||
		len(a.Headers) != len(b.Headers) {
		return false
	}
	for k, v := range a.Headers {
		if bv, ok := b.Headers[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// isReasoningModel 判断模型名是否属于推理模型（不接受 temperature，可返回推理摘要）。
func isReasoningModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	for _, marker := range []string{"reasoner", "reasoning", "deepseek-r1", "qwq", "gpt-5"} {
		if strings.Contains(model, marker) {
			return true
		}
	}
	return false
}

// openAIClient 按配置构造 SDK 客户端：请求超时防止网关无响应时挂起，
// 重试交给 SDK（429/5xx 指数退避）。
func openAIClient(cfg *Config) openai.Client {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = "ollama" // 本地网关不校验密钥，但 SDK 要求非空
	}
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(10 * time.Minute),
		option.WithMaxRetries(4),
	}
	if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	keys := make([]string, 0, len(cfg.Headers))
	for k := range cfg.Headers {
		if strings.TrimSpace(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		opts = append(opts, option.WithHeader(k, cfg.Headers[k]))
	}
	return openai.NewClient(opts...)
}

// withUserPrompt 把用户在设置里自定义的会话提示词追加到系统提示词之后。
func withUserPrompt(cfg *Config, systemPrompt string) string {
	prompt := strings.TrimSpace(systemPrompt)
	if custom := strings.TrimSpace(cfg.SystemPrompt); custom != "" {
		prompt += "\n\n用户自定义会话提示词:\n" + custom
	}
	return prompt
}
