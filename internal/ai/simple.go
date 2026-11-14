package ai

import (
	"context"
)

// Chat 发送一次不带工具的单轮请求。
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	out, _, err := c.chatSimple(ctx, systemPrompt, []ChatMessage{{Role: "user", Content: userMessage}})
	return out, err
}

// TestResponsesCapability 测试连接：返回回答文本与是否走了 Responses（false 表示回退到了 Chat Completions）。
func (c *Client) TestResponsesCapability(ctx context.Context, systemPrompt, userMessage string) (string, bool, error) {
	return c.chatSimple(ctx, systemPrompt, []ChatMessage{{Role: "user", Content: userMessage}})
}

// chatSimple 不带工具、不流式的请求，返回回答与"是否走了服务商的首选通道"
// （OpenAI 兼容通道里 false 表示回退到了 Chat Completions）。
func (c *Client) chatSimple(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, bool, error) {
	cfg, err := c.snapshot()
	if err != nil {
		return "", false, err
	}
	return c.providerFor(cfg).simple(ctx, cfg, withUserPrompt(cfg, systemPrompt), messages)
}
