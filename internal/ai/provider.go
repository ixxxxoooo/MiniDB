package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"minidb/internal/logger"
)

// Config.Provider 的取值。空值等同 ProviderOpenAI。
const (
	ProviderOpenAI    = "openai"    // OpenAI 兼容接口（OpenAI、DeepSeek、OpenRouter、Ollama、Gemini 兼容端点等）
	ProviderAnthropic = "anthropic" // Anthropic 原生 Messages API
)

// chatRequest 是一次带工具循环请求的全部入参，与具体服务商无关。
type chatRequest struct {
	prompt    string // 已拼好用户自定义提示词的系统提示词
	messages  []ChatMessage
	tools     []FunctionToolDefinition
	maxRounds int
	executor  ToolExecutor
	cb        ToolStreamCallbacks
	produced  *bool // 是否已向调用方输出过内容：为 true 时不能换通道或重试，否则内容会重复
}

// chatProvider 一种服务商协议的实现：所有"怎么和模型服务说话"的差异都收在这里，
// 工具循环的停止规则则统一在 runToolLoop。
type chatProvider interface {
	// run 执行多轮工具循环，返回最终回答。
	run(ctx context.Context, cfg *Config, req chatRequest) (string, error)
	// simple 不带工具的单次请求；第二个返回值见 Client.chatSimple。
	simple(ctx context.Context, cfg *Config, prompt string, messages []ChatMessage) (string, bool, error)
}

// providerFor 按配置选择服务商实现。
func (c *Client) providerFor(cfg *Config) chatProvider {
	if strings.EqualFold(strings.TrimSpace(cfg.Provider), ProviderAnthropic) {
		return anthropicProvider{}
	}
	return openaiCompatProvider{c: c}
}

// openaiCompatProvider OpenAI 兼容协议：Responses API 优先，网关不支持时回退 Chat Completions 并熔断一段时间。
type openaiCompatProvider struct{ c *Client }

func (p openaiCompatProvider) run(ctx context.Context, cfg *Config, req chatRequest) (string, error) {
	client := openAIClient(cfg)
	chat := &ChatCompatClient{client: client, config: cfg}
	if blocked, why := p.c.responsesBlocked(time.Now()); blocked {
		logger.Warn("[AI] Responses 熔断中，使用 Chat Completions: model=%s reason=%s", cfg.Model, why)
		return chat.Run(ctx, req.prompt, req.messages, req.tools, req.maxRounds, req.executor, req.cb)
	}
	logger.Info("[AI] 开始: provider=responses model=%s tools=%d messages=%d rounds_limit=%d", cfg.Model, len(req.tools), len(req.messages), req.maxRounds)
	out, err := (&ResponsesReActClient{client: client, config: cfg}).Run(ctx, req.prompt, req.messages, req.tools, req.maxRounds, req.executor, req.cb)
	if err == nil || !shouldFallbackToChat(err) || *req.produced {
		return out, err // 已经向界面输出过内容时不能换通道重来，否则内容会重复
	}
	p.c.blockResponses(err)
	logger.Warn("[AI] 网关不支持 Responses，回退 Chat Completions 并熔断 %s: %v", responsesCooldown, err)
	return chat.Run(ctx, req.prompt, req.messages, req.tools, req.maxRounds, req.executor, req.cb)
}

func (p openaiCompatProvider) simple(ctx context.Context, cfg *Config, prompt string, messages []ChatMessage) (string, bool, error) {
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
