package ai

import (
	"context"
	"strings"
	"time"

	"minidb/internal/logger"
)

// toolOutput 一次工具调用及其回传给模型的文本。
type toolOutput struct {
	call FunctionToolCall
	text string
}

// ChatWithToolsStream 运行"模型 → 工具 → 模型"循环直到得到最终回答，返回最终回答文本。
// Responses API 优先；网关明确不支持时回退 Chat Completions，并在一段时间内不再尝试 Responses。
//
// 参数：systemPrompt 系统提示词；messages 会话历史；tools 可用工具（为空则纯对话）；
// maxRounds 模型请求轮次上限；executor 工具执行回调；cb 事件回调。
func (c *Client) ChatWithToolsStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	cb ToolStreamCallbacks,
) (string, error) {
	cfg, err := c.snapshot()
	if err != nil {
		return "", err
	}
	if maxRounds <= 0 {
		maxRounds = 1
	}
	prompt := withUserPrompt(cfg, systemPrompt)
	cb, produced := trackOutput(cb)

	run := func() (string, error) {
		client := openAIClient(cfg)
		chat := &ChatCompatClient{client: client, config: cfg}
		if blocked, why := c.responsesBlocked(time.Now()); blocked {
			logger.Warn("[AI] Responses 熔断中，使用 Chat Completions: model=%s reason=%s", cfg.Model, why)
			return chat.Run(ctx, prompt, messages, tools, maxRounds, executor, cb)
		}
		logger.Info("[AI] 开始: provider=responses model=%s tools=%d messages=%d rounds_limit=%d", cfg.Model, len(tools), len(messages), maxRounds)
		out, err := (&ResponsesReActClient{client: client, config: cfg}).Run(ctx, prompt, messages, tools, maxRounds, executor, cb)
		if err == nil || !shouldFallbackToChat(err) || *produced {
			return out, err // 已经向界面输出过内容时不能换通道重来，否则内容会重复
		}
		c.blockResponses(err)
		logger.Warn("[AI] 网关不支持 Responses，回退 Chat Completions 并熔断 %s: %v", responsesCooldown, err)
		return chat.Run(ctx, prompt, messages, tools, maxRounds, executor, cb)
	}

	out, err := run()
	// 限流兜底：SDK 已对单次请求退避重试，这里只在首轮就被限流（还没有任何输出）时再等一会儿重来一次。
	if err != nil && IsRateLimit(err) && !*produced {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		return run()
	}
	return out, err
}

// trackOutput 包装回调，记录是否已向调用方输出过内容（用于判断重试/回退是否安全）。
func trackOutput(cb ToolStreamCallbacks) (ToolStreamCallbacks, *bool) {
	produced := new(bool)
	wrapped := cb
	wrapped.OnThinking = func(s string) {
		*produced = true
		if cb.OnThinking != nil {
			cb.OnThinking(s)
		}
	}
	wrapped.OnDelta = func(s string) {
		*produced = true
		if cb.OnDelta != nil {
			cb.OnDelta(s)
		}
	}
	wrapped.OnToolCall = func(call FunctionToolCall) {
		*produced = true
		if cb.OnToolCall != nil {
			cb.OnToolCall(call)
		}
	}
	return wrapped, produced
}

// runToolCalls 依次执行本轮的全部工具调用并通知回调，返回要回传给模型的输出。
func runToolCalls(executor ToolExecutor, calls []FunctionToolCall, cb ToolStreamCallbacks) []toolOutput {
	outs := make([]toolOutput, 0, len(calls))
	for _, call := range calls {
		if cb.OnToolCall != nil {
			cb.OnToolCall(call)
		}
		text, ms := executeToolCall(executor, call)
		if cb.OnToolResult != nil {
			cb.OnToolResult(call.ID, call.Name, text, ms)
		}
		outs = append(outs, toolOutput{call: call, text: text})
	}
	return outs
}

// executeToolCall 执行一次工具调用，失败时输出统一以 "ERROR:" 开头（系统提示词里约定了这个前缀）。
func executeToolCall(executor ToolExecutor, call FunctionToolCall) (string, int64) {
	if executor == nil {
		return "ERROR: 工具执行器不可用", 0
	}
	start := time.Now()
	res := executor(call)
	text := res.Output
	if res.Err != nil && !strings.HasPrefix(text, "ERROR") {
		text = "ERROR: " + res.Err.Error()
	}
	return text, time.Since(start).Milliseconds()
}
