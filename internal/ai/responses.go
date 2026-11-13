package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"minidb/internal/logger"
)

// ResponsesReActClient 用 OpenAI Responses API 驱动多轮工具循环。
//
// 采用"无状态"续接：每一轮都把完整输入（会话历史 + 之前各轮的说明文字、函数调用与工具输出）重新发给网关，
// 而不是靠 previous_response_id 让服务端回忆上一轮。原因是 DeepSeek 等兼容网关并不保存响应，
// 带 previous_response_id 回传 function_call_output 会报 400 "No tool call found for tool output"。
// 稳定的前缀仍然可以命中各家的提示词缓存，所以代价很小。
type ResponsesReActClient struct {
	client openai.Client
	config *Config
}

// roundResult 一轮 Responses 流式请求的产出。
type roundResult struct {
	text       string
	calls      []FunctionToolCall
	responseID string
}

// Run 执行多轮工具循环，返回最终回答文本。
func (r *ResponsesReActClient) Run(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	cb ToolStreamCallbacks,
) (string, error) {
	// history 是每一轮都完整发送的输入：先是会话消息，之后每轮追加模型的说明文字、函数调用和工具输出。
	history := buildResponseInputMessages(messages)
	responseTools := buildResponsesTools(tools)

	for round := 1; round <= maxRounds; round++ {
		cb.round(round)
		params := r.params(systemPrompt, history)
		if len(responseTools) > 0 {
			params.Tools = responseTools
			params.ParallelToolCalls = openai.Bool(false) // strict 模式与并行调用不兼容
		}
		res, err := r.consume(r.client.Responses.NewStreaming(ctx, params), cb)
		if err != nil {
			return "", err
		}
		logger.Info("[AI] Responses 第 %d 轮完成: response=%s text_len=%d tool_calls=%d", round, res.responseID, len(res.text), len(res.calls))
		if len(res.calls) == 0 {
			// 工具跑完后模型有时只"想"了一下没写回答：用不带工具的总结请求再要一次
			if strings.TrimSpace(res.text) == "" && round > 1 {
				return r.finalize(ctx, systemPrompt, history, cb)
			}
			return strings.TrimSpace(res.text), nil
		}
		history = append(history, assistantCallItems(res.text, res.calls)...)
		for _, out := range runToolCalls(executor, res.calls, cb) {
			history = append(history, responses.ResponseInputItemParamOfFunctionCallOutput(out.call.responseCallID(), out.text))
		}
		if round == maxRounds {
			logger.Warn("[AI] 已达工具轮次上限(%d)，触发无工具总结", maxRounds)
			return r.finalize(ctx, systemPrompt, history, cb)
		}
	}
	return "", nil
}

// assistantCallItems 把模型一轮的输出还原成输入项：先是调用前的说明文字，再是各个函数调用。
// 函数调用只带 call_id、不带服务端分配的 item id：无状态回放时这些 id 对网关毫无意义，
// 带上反而会让 OpenAI 要求配套的 reasoning 项。
func assistantCallItems(text string, calls []FunctionToolCall) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(calls)+1)
	if strings.TrimSpace(text) != "" {
		items = append(items, responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleAssistant))
	}
	for _, call := range calls {
		args := call.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		items = append(items, responses.ResponseInputItemParamOfFunctionCall(args, call.responseCallID(), call.Name))
	}
	return items
}

// finalize 不带工具地再请求一次，让模型基于已有工具结果给出最终回答。
func (r *ResponsesReActClient) finalize(ctx context.Context, systemPrompt string, history responses.ResponseInputParam, cb ToolStreamCallbacks) (string, error) {
	input := append(responses.ResponseInputParam{}, history...)
	input = append(input, responses.ResponseInputItemParamOfMessage("请基于已有工具结果给出最终回答，不要继续调用工具。", responses.EasyInputMessageRoleUser))
	res, err := r.consume(r.client.Responses.NewStreaming(ctx, r.params(systemPrompt+"\n\n"+finalPrompt, input)), cb)
	if err != nil {
		return "", fmt.Errorf("AI 最终总结失败: %w", err)
	}
	return strings.TrimSpace(res.text), nil
}

// params 构造一次 Responses 请求的公共参数。
func (r *ResponsesReActClient) params(instructions string, input responses.ResponseInputParam) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(r.config.Model),
		Instructions: openai.String(instructions),
		Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: input},
	}
	applyGenerationConfig(&params, r.config)
	applyReasoningSummaryRequest(&params, r.config.Model)
	return params
}

// consume 读完一轮流式响应：推理与文本增量实时转给回调，函数调用在流成功结束后才返回，
// 这样半截的流不会触发工具执行。
func (r *ResponsesReActClient) consume(stream *ssestream.Stream[responses.ResponseStreamEventUnion], cb ToolStreamCallbacks) (roundResult, error) {
	defer stream.Close()
	var res roundResult
	var text strings.Builder
	summarySeen, reasoningSeen := false, false
	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "response.reasoning_summary_text.delta":
			summarySeen = true
			cb.thinking(event.Delta)
		case "response.reasoning_summary_text.done":
			if !summarySeen {
				cb.thinking(event.AsResponseReasoningSummaryTextDone().Text)
			}
		case "response.reasoning_summary_part.done":
			if !summarySeen {
				cb.thinking(event.AsResponseReasoningSummaryPartDone().Part.Text)
			}
		case "response.reasoning_text.delta":
			reasoningSeen = true
			cb.thinking(event.Delta)
		case "response.reasoning_text.done":
			if !reasoningSeen {
				cb.thinking(event.AsResponseReasoningTextDone().Text)
			}
		case "response.output_text.delta":
			text.WriteString(event.Delta)
			cb.delta(event.Delta)
		case "response.output_item.done":
			if call, ok := responseFunctionCall(event.Item); ok {
				res.calls = append(res.calls, call)
			}
		case "response.completed":
			res.responseID = event.Response.ID
			u := event.Response.Usage
			cb.usage(Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, TotalTokens: u.TotalTokens})
		case "response.failed", "error":
			msg := strings.TrimSpace(event.Message)
			if msg == "" {
				msg = strings.TrimSpace(event.Response.Error.Message)
			}
			if msg == "" {
				msg = "Responses 流失败"
			}
			return roundResult{}, fmt.Errorf("%s", msg)
		}
	}
	if err := stream.Err(); err != nil {
		return roundResult{}, fmt.Errorf("AI 流式响应失败: %w", err)
	}
	res.text = text.String()
	return res, nil
}

// applyGenerationConfig 把用户配置的 maxTokens/temperature 应用到请求；
// 推理模型不接受 temperature，传了会被 API 拒绝。
func applyGenerationConfig(params *responses.ResponseNewParams, cfg *Config) {
	if params == nil || cfg == nil {
		return
	}
	if cfg.MaxTokens > 0 {
		params.MaxOutputTokens = openai.Int(int64(cfg.MaxTokens))
	}
	if cfg.Temperature != 0 && !isReasoningModel(cfg.Model) {
		params.Temperature = openai.Float(cfg.Temperature)
	}
}

// applyReasoningSummaryRequest 让支持的推理模型返回推理摘要，返回是否已启用。
func applyReasoningSummaryRequest(params *responses.ResponseNewParams, model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if params == nil || !(strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") || strings.Contains(model, "gpt-5")) {
		return false
	}
	params.Reasoning = shared.ReasoningParam{Effort: shared.ReasoningEffortLow, Summary: shared.ReasoningSummaryAuto}
	if strings.Contains(model, "gpt-5-pro") {
		params.Reasoning.Effort = shared.ReasoningEffortHigh
	}
	return true
}
