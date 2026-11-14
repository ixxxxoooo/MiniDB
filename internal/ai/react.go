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
// 具体怎么和服务商通信由 Config.Provider 决定：默认走 OpenAI 兼容通道（Responses 优先，
// 网关明确不支持时回退 Chat Completions），也可以走 Anthropic 原生 Messages API。
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
	cb, produced := trackOutput(cb)
	req := chatRequest{
		prompt: withUserPrompt(cfg, systemPrompt), messages: messages, tools: tools,
		maxRounds: maxRounds, executor: executor, cb: cb, produced: produced,
	}
	provider := c.providerFor(cfg)

	out, err := provider.run(ctx, cfg, req)
	// 限流兜底：SDK 已对单次请求退避重试，这里只在首轮就被限流（还没有任何输出）时再等一会儿重来一次。
	if err != nil && IsRateLimit(err) && !*produced {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		return provider.run(ctx, cfg, req)
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
	end := logger.Perf("ai.tool", "name", call.Name) // 工具里会查库，慢的话先看同一时间的 db.* 记录
	res := executor(call)
	end()
	text := res.Output
	if res.Err != nil && !strings.HasPrefix(text, "ERROR") {
		text = "ERROR: " + res.Err.Error()
	}
	return text, time.Since(start).Milliseconds()
}
