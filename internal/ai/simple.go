package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"minidb/internal/logger"
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

// chatSimple 不带工具、不流式的请求：Responses 优先，网关不支持时回退 Chat Completions。
func (c *Client) chatSimple(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, bool, error) {
	cfg, err := c.snapshot()
	if err != nil {
		return "", false, err
	}
	prompt := withUserPrompt(cfg, systemPrompt)
	client := openAIClient(cfg)

	params := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(cfg.Model),
		Instructions: openai.String(prompt),
		Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: buildResponseInputMessages(messages)},
	}
	applyGenerationConfig(&params, cfg)
	applyReasoningSummaryRequest(&params, cfg.Model)
	resp, err := client.Responses.New(ctx, params)
	if err == nil {
		return strings.TrimSpace(resp.OutputText()), true, nil
	}
	if !shouldFallbackToChat(err) {
		return "", true, fmt.Errorf("AI 请求失败: %w", err)
	}

	logger.Warn("[AI] Responses 不可用，回退 Chat Completions: %v", err)
	chatResp, err := client.Chat.Completions.New(ctx, (&ChatCompatClient{client: client, config: cfg}).request(buildChatMessages(prompt, messages)))
	if err != nil {
		return "", false, fmt.Errorf("AI 请求失败: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", false, fmt.Errorf("AI 返回空结果")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), false, nil
}
