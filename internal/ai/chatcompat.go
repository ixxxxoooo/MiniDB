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
	msgs := buildChatMessages(systemPrompt, messages)
	chatTools := buildChatTools(tools)

	for round := 1; round <= maxRounds; round++ {
		cb.round(round)
		req := c.request(msgs)
		if len(chatTools) > 0 {
			req.Tools = chatTools
			req.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
			req.ParallelToolCalls = openai.Bool(false)
		}
		content, reasoning, calls, finish, err := c.consume(ctx, req, cb)
		if err != nil {
			return "", err
		}
		logger.Info("[AI] Chat 第 %d 轮完成: finish=%s text_len=%d tool_calls=%d", round, finish, len(content), len(calls))

		// 部分网关带着工具调用却返回 finish_reason=stop，所以以"有没有调用"为准；
		// 只有因长度被截断（参数可能不完整）时才不执行。
		if len(calls) > 0 && finish != "length" {
			msgs = append(msgs, assistantToolMessage(content, reasoning, calls))
			for _, out := range runToolCalls(executor, calls, cb) {
				msgs = append(msgs, openai.ToolMessage(out.text, out.call.ID))
			}
			if round == maxRounds {
				logger.Warn("[AI] 已达工具轮次上限(%d)，触发无工具总结", maxRounds)
				return c.finalize(ctx, msgs, cb)
			}
			continue
		}
		if strings.TrimSpace(content) == "" && round > 1 {
			return c.finalize(ctx, msgs, cb)
		}
		return strings.TrimSpace(content), nil
	}
	return c.finalize(ctx, msgs, cb)
}

// consume 读完一轮流式响应，返回文本、函数调用与结束原因；文本与推理增量实时转给回调。
func (c *ChatCompatClient) consume(ctx context.Context, req openai.ChatCompletionNewParams, cb ToolStreamCallbacks) (string, string, []FunctionToolCall, string, error) {
	stream := c.client.Chat.Completions.NewStreaming(ctx, req)
	defer stream.Close()
	var text, reasoning strings.Builder
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
				text.WriteString(choice.Delta.Content)
				cb.delta(choice.Delta.Content)
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
