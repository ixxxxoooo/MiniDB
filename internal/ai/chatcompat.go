package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"minidb/internal/logger"
)

// ChatCompatClient 用 Chat Completions 驱动多轮工具循环，仅作为不支持 Responses 的网关的兼容通道。
type ChatCompatClient struct {
	client openai.Client
	config *Config
}

// Run 执行多轮工具循环，返回最终回答文本。
func (c *ChatCompatClient) Run(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	cb ToolStreamCallbacks,
) (string, error) {
	l := &chatLoop{c: c, msgs: buildChatMessages(systemPrompt, messages), tools: buildChatTools(tools), cb: cb}
	return runToolLoop(ctx, l, maxRounds, executor, cb)
}

// chatLoop Chat Completions 通道的传输层。
type chatLoop struct {
	c     *ChatCompatClient
	msgs  []openai.ChatCompletionMessageParamUnion
	tools []openai.ChatCompletionToolUnionParam
	cb    ToolStreamCallbacks
}

func (l *chatLoop) next(ctx context.Context, round int) (turn, error) {
	req := l.c.request(l.msgs)
	if len(l.tools) > 0 {
		req.Tools = l.tools
		req.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
		req.ParallelToolCalls = openai.Bool(false)
	}
	content, reasoning, calls, finish, err := l.c.consume(ctx, req, l.cb)
	if err != nil {
		return turn{}, err
	}
	logger.Info("[AI] Chat 第 %d 轮完成: finish=%s text_len=%d reasoning_len=%d tool_calls=%d", round, finish, len(content), len(reasoning), len(calls))
	// 部分网关带着工具调用却返回 finish_reason=stop，所以以"有没有调用"为准；
	// 只有因长度被截断（参数可能不完整）时才不执行。
	if finish == "length" {
		calls = nil
	}
	return turn{text: content, reasoning: reasoning, calls: calls}, nil
}

func (l *chatLoop) record(t turn, outs []toolOutput) {
	l.msgs = append(l.msgs, assistantToolMessage(t.text, t.reasoning, t.calls))
	for _, out := range outs {
		l.msgs = append(l.msgs, openai.ToolMessage(out.text, out.call.ID))
	}
}

func (l *chatLoop) finalize(ctx context.Context) (string, error) {
	return l.c.finalize(ctx, l.msgs, l.cb)
}

// consume 读完一轮流式响应，返回文本、函数调用与结束原因；文本与推理增量实时转给回调。
func (c *ChatCompatClient) consume(ctx context.Context, req openai.ChatCompletionNewParams, cb ToolStreamCallbacks) (string, string, []FunctionToolCall, string, error) {
	stream := c.client.Chat.Completions.NewStreaming(ctx, req)
	defer stream.Close()
	var text, reasoning strings.Builder
	var split thinkSplitter // 行内的 <think>…</think> 也算推理
	take := func(th, tx string) {
		if th != "" {
			reasoning.WriteString(th)
			cb.thinking(th)
		}
		if tx != "" {
			text.WriteString(tx)
			cb.delta(tx)
		}
	}
	byIndex := map[int64]*FunctionToolCall{}
	finish := ""
	for stream.Next() {
		chunk := stream.Current()
		cb.usage(Usage{InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens, TotalTokens: chunk.Usage.TotalTokens})
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
			if delta := chatReasoningDelta(choice.Delta.RawJSON()); delta != "" {
				reasoning.WriteString(delta)
				cb.thinking(delta)
			}
			if choice.Delta.Content != "" {
				take(split.feed(choice.Delta.Content))
			}
			for _, tc := range choice.Delta.ToolCalls {
				call := byIndex[tc.Index]
				if call == nil {
					call = &FunctionToolCall{}
					byIndex[tc.Index] = call
				}
				if tc.ID != "" {
					call.ID, call.CallID = tc.ID, tc.ID
				}
				call.Name += tc.Function.Name
				call.Arguments += tc.Function.Arguments
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", "", nil, "", fmt.Errorf("AI 流式响应失败: %w", err)
	}
	take(split.flush())
	return text.String(), reasoning.String(), sortedToolCalls(byIndex), finish, nil
}

// finalize 不带工具地再请求一次，让模型基于已有工具结果给出最终回答。
func (c *ChatCompatClient) finalize(ctx context.Context, msgs []openai.ChatCompletionMessageParamUnion, cb ToolStreamCallbacks) (string, error) {
	final := append(append([]openai.ChatCompletionMessageParamUnion{}, msgs...), openai.SystemMessage(finalPrompt))
	content, _, _, _, err := c.consume(ctx, c.request(final), cb)
	if err != nil {
		return "", fmt.Errorf("AI 最终总结失败: %w", err)
	}
	return strings.TrimSpace(content), nil
}

// request 构造一次 Chat Completions 请求的公共参数。
func (c *ChatCompatClient) request(msgs []openai.ChatCompletionMessageParamUnion) openai.ChatCompletionNewParams {
	req := openai.ChatCompletionNewParams{Model: shared.ChatModel(c.config.Model), Messages: msgs}
	if c.config.MaxTokens > 0 {
		req.MaxCompletionTokens = openai.Int(int64(c.config.MaxTokens))
	}
	if c.config.Temperature != 0 && !isReasoningModel(c.config.Model) {
		req.Temperature = openai.Float(c.config.Temperature)
	}
	return req
}
