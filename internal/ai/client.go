package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"tableplus-ai/internal/logger"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/tidwall/gjson"
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

// Client AI 客户端。主链路优先使用 OpenAI Responses API；仅在网关不支持 /responses 时回退 Chat Completions。
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

// ModelName 返回当前配置模型名（小写、去空格）
func (c *Client) ModelName() string {
	if c.config == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.config.Model))
}

// IsReasoningModel 判断当前模型是否属于推理模型。
func (c *Client) IsReasoningModel() bool {
	return isReasoningModelName(c.ModelName())
}

func isReasoningModelName(model string) bool {
	if model == "" {
		return false
	}
	if strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
		return true
	}
	indicators := []string{
		"reasoner",
		"reasoning",
		"deepseek-r1",
		"deepseek-reasoner",
		"qwq",
		"gpt-5",
	}
	for _, item := range indicators {
		if strings.Contains(model, item) {
			return true
		}
	}
	return false
}

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FunctionToolDefinition 描述可供模型调用的函数工具
type FunctionToolDefinition struct {
	Name        string
	Description string
	Parameters  any
}

// FunctionToolCall 表示模型返回的一次函数调用
type FunctionToolCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments string
}

func (c FunctionToolCall) responseCallID() string {
	if strings.TrimSpace(c.CallID) != "" {
		return c.CallID
	}
	return c.ID
}

// FunctionToolPlanResult 表示一次工具规划响应
type FunctionToolPlanResult struct {
	Content      string
	FinishReason string
	ToolCalls    []FunctionToolCall
}

// ToolExecutionResult 工具执行结果，Output 会回传模型，SQL/错误用于 UI 与日志。
type ToolExecutionResult struct {
	Output string
	SQL    string
	Err    error
}

// ToolExecutor 工具执行回调：接收 AI 返回的 tool_call，返回执行结果。
type ToolExecutor func(call FunctionToolCall) ToolExecutionResult

// ToolStreamCallbacks ReAct 多轮流式对话的回调集合
type ToolStreamCallbacks struct {
	// OnThinking 模型实际返回的 reasoning summary/text 或兼容网关 reasoning_content
	OnThinking func(content string)
	// OnToolCall 模型决定调用某个工具时触发（完整 function_call 已确认，工具尚未执行）
	OnToolCall func(call FunctionToolCall)
	// OnToolArgumentsDone 工具参数完整可用时触发
	OnToolArgumentsDone func(call FunctionToolCall)
	// OnToolSQL 工具执行得到 SQL/元数据语句时触发
	OnToolSQL func(callID, toolName, sql string)
	// OnToolResult 工具执行完成后触发
	OnToolResult func(callID, toolName, result string, durationMs int64)
	// OnDelta 最终回答阶段的流式文本片段
	OnDelta func(delta string)
	// OnFinalAnswer 最终回答阶段开始时触发
	OnFinalAnswer func()
}

type normalizedEventType string

const (
	eventReasoningDelta    normalizedEventType = "reasoning_delta"
	eventToolCallStarted   normalizedEventType = "tool_call_started"
	eventToolArgumentsDone normalizedEventType = "tool_call_arguments_done"
	eventToolResult        normalizedEventType = "tool_result"
	eventAnswerStarted     normalizedEventType = "answer_started"
	eventAnswerDelta       normalizedEventType = "answer_delta"
	eventCompleted         normalizedEventType = "completed"
	eventFailed            normalizedEventType = "failed"
)

// NormalizedStreamEvent 是 provider adapter 内部统一事件，便于测试和服务层映射。
type NormalizedStreamEvent struct {
	Type       normalizedEventType
	Delta      string
	ToolCall   FunctionToolCall
	ToolOutput string
	DurationMs int64
	Error      error
}

func (c *Client) ensureConfigured() error {
	if c.config == nil {
		return fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}
	if strings.TrimSpace(c.config.Model) == "" {
		return fmt.Errorf("AI 模型未配置，请先在设置中配置模型")
	}
	return nil
}

func (c *Client) openAIClient() openai.Client {
	apiKey := strings.TrimSpace(c.config.APIKey)
	if apiKey == "" {
		apiKey = "ollama"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL := strings.TrimSpace(c.config.BaseURL); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	keys := make([]string, 0, len(c.config.Headers))
	for k := range c.config.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		opts = append(opts, option.WithHeader(k, c.config.Headers[k]))
	}
	return openai.NewClient(opts...)
}

func (c *Client) finalSystemPrompt(systemPrompt string) string {
	finalSystemPrompt := strings.TrimSpace(systemPrompt)
	if strings.TrimSpace(c.config.SystemPrompt) != "" {
		finalSystemPrompt += "\n\n用户自定义会话提示词:\n" + strings.TrimSpace(c.config.SystemPrompt)
	}
	return finalSystemPrompt
}

// Chat 发送聊天请求，优先 Responses，网关不支持时回退 Chat Completions。
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	result, usedResponses, err := c.chatSimple(ctx, systemPrompt, []ChatMessage{{Role: "user", Content: userMessage}})
	if err != nil {
		return "", err
	}
	logger.Info("[AI] Chat 成功: provider=%s response_len=%d", providerName(usedResponses), len(result))
	return result, nil
}

// TestResponsesCapability 测试 /responses 能力，失败时自动回退 chat/completions。
func (c *Client) TestResponsesCapability(ctx context.Context, systemPrompt, userMessage string) (string, bool, error) {
	return c.chatSimple(ctx, systemPrompt, []ChatMessage{{Role: "user", Content: userMessage}})
}

// ChatWithMessages 支持多轮对话的聊天接口
func (c *Client) ChatWithMessages(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error) {
	result, usedResponses, err := c.chatSimple(ctx, systemPrompt, messages)
	if err != nil {
		return "", err
	}
	logger.Info("[AI] ChatWithMessages 成功: provider=%s response_len=%d", providerName(usedResponses), len(result))
	return result, nil
}

// ChatWithMessagesStream 支持流式返回的聊天接口（无工具）。
func (c *Client) ChatWithMessagesStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	onDelta func(delta string),
) (string, error) {
	if err := c.ensureConfigured(); err != nil {
		return "", err
	}
	callbacks := ToolStreamCallbacks{
		OnDelta:       onDelta,
		OnFinalAnswer: func() {},
	}
	return c.ChatWithToolsStreamRealtime(ctx, systemPrompt, messages, nil, 1, nil, callbacks)
}

func (c *Client) chatSimple(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, bool, error) {
	if err := c.ensureConfigured(); err != nil {
		return "", false, err
	}
	finalSystemPrompt := c.finalSystemPrompt(systemPrompt)
	client := c.openAIClient()
	params := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(c.config.Model),
		Instructions: openai.String(finalSystemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: buildResponseInputMessages(messages),
		},
		ParallelToolCalls: openai.Bool(false),
	}
	logger.Info("[AI] Chat simple Responses 请求: model=%s baseURL=%s messages=%d", c.config.Model, c.config.BaseURL, len(messages))
	resp, err := client.Responses.New(ctx, params)
	if err == nil {
		return strings.TrimSpace(resp.OutputText()), true, nil
	}
	if !shouldFallbackToChat(err) {
		logger.Error("[AI] Responses 请求失败: %v", err)
		return "", true, fmt.Errorf("AI 请求失败: %w", err)
	}

	logger.Warn("[AI] Responses 不可用，回退 Chat Completions: %v", err)
	chatResp, chatErr := client.Chat.Completions.New(ctx, c.newChatCompletionRequest(buildChatMessages(finalSystemPrompt, messages)))
	if chatErr != nil {
		logger.Error("[AI] Chat fallback 请求失败: %v", chatErr)
		return "", false, fmt.Errorf("AI 请求失败: %w", chatErr)
	}
	if len(chatResp.Choices) == 0 {
		return "", false, fmt.Errorf("AI 返回空结果")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), false, nil
}

// ChatWithToolsStream 兼容旧入口，内部使用 Realtime 实现。
func (c *Client) ChatWithToolsStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	return c.ChatWithToolsStreamRealtime(ctx, systemPrompt, messages, tools, maxRounds, executor, callbacks)
}

// ChatWithToolsStreamRealtime ReAct 多轮流式对话：Responses API 优先，Chat Completions 只作为兼容回退。
func (c *Client) ChatWithToolsStreamRealtime(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	if err := c.ensureConfigured(); err != nil {
		return "", err
	}
	if maxRounds <= 0 {
		maxRounds = 1
	}
	finalSystemPrompt := c.finalSystemPrompt(systemPrompt)
	logger.Info("[AI] ChatWithToolsStreamRealtime 开始: provider=responses model=%s baseURL=%s rounds_limit=%d tools=%d messages=%d",
		c.config.Model, c.config.BaseURL, maxRounds, len(tools), len(messages))

	respClient := &ResponsesReActClient{client: c.openAIClient(), config: c.config}
	result, err := respClient.Run(ctx, finalSystemPrompt, messages, tools, maxRounds, executor, callbacks)
	if err == nil {
		return result, nil
	}
	if !shouldFallbackToChat(err) {
		return "", err
	}

	logger.Warn("[AI] Responses ReAct 不可用，回退 Chat Completions: %v", err)
	chatClient := &ChatCompatClient{client: c.openAIClient(), config: c.config}
	return chatClient.Run(ctx, finalSystemPrompt, messages, tools, maxRounds, executor, callbacks)
}

type ResponsesReActClient struct {
	client openai.Client
	config *Config
}

func (r *ResponsesReActClient) Run(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	input := buildResponseInputMessages(messages)
	responseTools := buildResponsesTools(tools)
	previousResponseID := ""
	var finalAnswer strings.Builder

	for round := 1; round <= maxRounds; round++ {
		roundID := makeRoundID(round)
		params := responses.ResponseNewParams{
			Model:             shared.ResponsesModel(r.config.Model),
			Instructions:      openai.String(systemPrompt),
			Input:             responses.ResponseNewParamsInputUnion{OfInputItemList: input},
			ParallelToolCalls: openai.Bool(false),
		}
		if previousResponseID != "" {
			params.PreviousResponseID = openai.String(previousResponseID)
		}
		if len(responseTools) > 0 {
			params.Tools = responseTools
		}
		logger.Info("[AI] Responses round 开始: roundID=%s model=%s tools=%d previous=%v", roundID, r.config.Model, len(responseTools), previousResponseID != "")

		stream := r.client.Responses.NewStreaming(ctx, params)
		var contentBuf strings.Builder
		var calls []FunctionToolCall
		reasoningEvents := 0
		answerTextEvents := 0
		var responseID string
		streamAnswerDirectly := len(responseTools) == 0
		answerStarted := false
		for stream.Next() {
			event := stream.Current()
			switch event.Type {
			case "response.reasoning_summary_text.delta":
				if event.Delta != "" && callbacks.OnThinking != nil {
					reasoningEvents++
					callbacks.OnThinking(event.Delta)
				}
			case "response.reasoning_text.delta":
				if event.Delta != "" && callbacks.OnThinking != nil {
					reasoningEvents++
					callbacks.OnThinking(event.Delta)
				}
			case "response.output_text.delta":
				if event.Delta != "" {
					answerTextEvents++
					contentBuf.WriteString(event.Delta)
					if streamAnswerDirectly {
						emitStreamingResponseDelta(event.Delta, &answerStarted, callbacks)
					}
				}
			case "response.output_item.done":
				call := responseFunctionCallFromItem(event.Item)
				if call.Name != "" {
					calls = append(calls, call)
					if callbacks.OnToolCall != nil {
						callbacks.OnToolCall(call)
					}
					if callbacks.OnToolArgumentsDone != nil {
						callbacks.OnToolArgumentsDone(call)
					}
				}
			case "response.completed":
				if event.Response.ID != "" {
					responseID = event.Response.ID
				}
			case "response.failed", "error":
				msg := strings.TrimSpace(event.Message)
				if msg == "" {
					msg = "Responses stream failed"
				}
				stream.Close()
				return "", fmt.Errorf("%s", msg)
			}
		}
		if err := stream.Err(); err != nil {
			stream.Close()
			return "", fmt.Errorf("AI Responses 流式响应失败: %w", err)
		}
		_ = stream.Close()
		logger.Info("[AI] Responses round 完成: roundID=%s response=%s reasoning_events=%d text_events=%d tool_calls=%d",
			roundID, responseID, reasoningEvents, answerTextEvents, len(calls))
		if responseID != "" {
			previousResponseID = responseID
		}

		if len(calls) == 0 {
			content := contentBuf.String()
			finalAnswer.WriteString(content)
			if !streamAnswerDirectly {
				emitBufferedResponseOutput(content, false, callbacks)
			}
			return strings.TrimSpace(finalAnswer.String()), nil
		}

		outputItems := make([]responses.ResponseInputItemUnionParam, 0, len(calls))
		for _, call := range calls {
			result, durationMs := executeToolCall(executor, call)
			if callbacks.OnToolSQL != nil && strings.TrimSpace(result.SQL) != "" {
				callbacks.OnToolSQL(call.ID, call.Name, result.SQL)
			}
			toolOutput := result.Output
			if result.Err != nil {
				toolOutput = "ERROR: " + result.Err.Error()
			}
			if callbacks.OnToolResult != nil {
				callbacks.OnToolResult(call.ID, call.Name, toolOutput, durationMs)
			}
			outputItems = append(outputItems, responses.ResponseInputItemParamOfFunctionCallOutput(call.responseCallID(), toolOutput))
		}

		input = outputItems
		if round == maxRounds {
			logger.Warn("[AI] Responses 已达工具轮次上限(%d)，触发无工具总结", maxRounds)
			return r.finalizeAfterToolLimit(ctx, systemPrompt, previousResponseID, callbacks)
		}
	}
	return strings.TrimSpace(finalAnswer.String()), nil
}

func (r *ResponsesReActClient) finalizeAfterToolLimit(ctx context.Context, systemPrompt, previousResponseID string, callbacks ToolStreamCallbacks) (string, error) {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(r.config.Model),
		Instructions: openai.String(systemPrompt + "\n\n你已达到工具调用轮次上限。禁止继续调用任何工具。请基于已有工具结果直接给出最终回答。" +
			"如果已有工具返回 ERROR，请先简要说明根因并给出修复后的 SQL，再给出结论与建议。"),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("请基于已有工具结果给出最终回答，不要继续调用工具。"),
		},
		ParallelToolCalls: openai.Bool(false),
	}
	if previousResponseID != "" {
		params.PreviousResponseID = openai.String(previousResponseID)
	}
	stream := r.client.Responses.NewStreaming(ctx, params)
	var content strings.Builder
	started := false
	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if callbacks.OnThinking != nil && event.Delta != "" {
				callbacks.OnThinking(event.Delta)
			}
		case "response.output_text.delta":
			if event.Delta == "" {
				continue
			}
			if !started {
				started = true
				if callbacks.OnFinalAnswer != nil {
					callbacks.OnFinalAnswer()
				}
			}
			content.WriteString(event.Delta)
			if callbacks.OnDelta != nil {
				callbacks.OnDelta(event.Delta)
			}
		}
	}
	if err := stream.Err(); err != nil {
		stream.Close()
		return "", fmt.Errorf("AI Responses 最终总结失败: %w", err)
	}
	_ = stream.Close()
	return strings.TrimSpace(content.String()), nil
}

func emitBufferedResponseOutput(content string, hasToolCalls bool, callbacks ToolStreamCallbacks) {
	if hasToolCalls {
		return
	}
	emitAnswerContent(content, callbacks)
}

func emitStreamingResponseDelta(delta string, started *bool, callbacks ToolStreamCallbacks) {
	if delta == "" {
		return
	}
	if started != nil && !*started {
		*started = true
		if callbacks.OnFinalAnswer != nil {
			callbacks.OnFinalAnswer()
		}
	}
	if callbacks.OnDelta != nil {
		callbacks.OnDelta(delta)
	}
}

func emitAnswerContent(content string, callbacks ToolStreamCallbacks) {
	if callbacks.OnFinalAnswer != nil {
		callbacks.OnFinalAnswer()
	}
	if callbacks.OnDelta != nil && content != "" {
		callbacks.OnDelta(content)
	}
}

type ChatCompatClient struct {
	client openai.Client
	config *Config
}

func (c *ChatCompatClient) Run(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	msgs := buildChatMessages(systemPrompt, messages)
	openaiTools := buildChatTools(tools)

	for round := 1; round <= maxRounds; round++ {
		roundID := makeRoundID(round)
		req := c.newChatRequest(msgs)
		if len(openaiTools) > 0 {
			req.Tools = openaiTools
			req.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
			req.ParallelToolCalls = openai.Bool(false)
		}
		logger.Info("[AI] Chat fallback round 开始: roundID=%s model=%s tools=%d messages=%d", roundID, c.config.Model, len(openaiTools), len(msgs))

		stream := c.client.Chat.Completions.NewStreaming(ctx, req)
		var contentBuf strings.Builder
		var reasoningBuf strings.Builder
		toolCallMap := make(map[int64]*FunctionToolCall)
		finishReason := ""

		for stream.Next() {
			chunk := stream.Current()
			for _, choice := range chunk.Choices {
				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}
				if choice.Delta.Content != "" {
					contentBuf.WriteString(choice.Delta.Content)
				}
				reasoning := extractChatReasoningDelta(choice.Delta.RawJSON())
				if reasoning != "" {
					reasoningBuf.WriteString(reasoning)
					if callbacks.OnThinking != nil {
						callbacks.OnThinking(reasoning)
					}
				}
				for _, tc := range choice.Delta.ToolCalls {
					existing := toolCallMap[tc.Index]
					if existing == nil {
						existing = &FunctionToolCall{}
						toolCallMap[tc.Index] = existing
					}
					if tc.ID != "" {
						existing.ID = tc.ID
						existing.CallID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Name += tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			stream.Close()
			return "", fmt.Errorf("AI Chat fallback 流式响应失败: %w", err)
		}
		_ = stream.Close()

		content := contentBuf.String()
		calls := sortedToolCalls(toolCallMap)
		logger.Info("[AI] Chat fallback round 完成: roundID=%s finish=%s content_len=%d reasoning_len=%d tool_calls=%d",
			roundID, finishReason, len(content), reasoningBuf.Len(), len(calls))

		if finishReason == "tool_calls" && len(calls) > 0 {
			msgs = append(msgs, assistantToolMessage("", calls))
			for _, call := range calls {
				if callbacks.OnToolCall != nil {
					callbacks.OnToolCall(call)
				}
				if callbacks.OnToolArgumentsDone != nil {
					callbacks.OnToolArgumentsDone(call)
				}
				result, durationMs := executeToolCall(executor, call)
				if callbacks.OnToolSQL != nil && strings.TrimSpace(result.SQL) != "" {
					callbacks.OnToolSQL(call.ID, call.Name, result.SQL)
				}
				toolOutput := result.Output
				if result.Err != nil {
					toolOutput = "ERROR: " + result.Err.Error()
				}
				if callbacks.OnToolResult != nil {
					callbacks.OnToolResult(call.ID, call.Name, toolOutput, durationMs)
				}
				msgs = append(msgs, openai.ToolMessage(toolOutput, call.ID))
			}
			if round == maxRounds {
				logger.Warn("[AI] Chat fallback 已达工具轮次上限(%d)，触发无工具总结", maxRounds)
				return c.finalizeAfterToolLimit(ctx, msgs, callbacks)
			}
			continue
		}

		emitAnswerContent(content, callbacks)
		return strings.TrimSpace(content), nil
	}
	return c.finalizeAfterToolLimit(ctx, msgs, callbacks)
}

func (c *ChatCompatClient) newChatRequest(msgs []openai.ChatCompletionMessageParamUnion) openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.config.Model),
		Messages: msgs,
	}
}

func (c *ChatCompatClient) finalizeAfterToolLimit(ctx context.Context, msgs []openai.ChatCompletionMessageParamUnion, callbacks ToolStreamCallbacks) (string, error) {
	finalMsgs := append(append([]openai.ChatCompletionMessageParamUnion{}, msgs...),
		openai.SystemMessage("你已达到工具调用轮次上限。禁止继续调用任何工具。请基于已有工具结果直接给出最终回答。如果已有工具返回 ERROR，请先简要说明根因并给出修复后的 SQL，再给出结论与建议。"))
	req := c.newChatRequest(finalMsgs)
	stream := c.client.Chat.Completions.NewStreaming(ctx, req)
	var content strings.Builder
	started := false
	for stream.Next() {
		chunk := stream.Current()
		for _, choice := range chunk.Choices {
			reasoning := extractChatReasoningDelta(choice.Delta.RawJSON())
			if reasoning != "" && callbacks.OnThinking != nil {
				callbacks.OnThinking(reasoning)
			}
			if choice.Delta.Content == "" {
				continue
			}
			if !started {
				started = true
				if callbacks.OnFinalAnswer != nil {
					callbacks.OnFinalAnswer()
				}
			}
			content.WriteString(choice.Delta.Content)
			if callbacks.OnDelta != nil {
				callbacks.OnDelta(choice.Delta.Content)
			}
		}
	}
	if err := stream.Err(); err != nil {
		stream.Close()
		return "", fmt.Errorf("AI Chat fallback 最终总结失败: %w", err)
	}
	_ = stream.Close()
	return strings.TrimSpace(content.String()), nil
}

func (c *Client) newChatCompletionRequest(msgs []openai.ChatCompletionMessageParamUnion) openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.config.Model),
		Messages: msgs,
	}
}

func buildResponseInputMessages(messages []ChatMessage) responses.ResponseInputParam {
	input := make(responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		input = append(input, responses.ResponseInputItemParamOfMessage(content, responseRole(msg.Role)))
	}
	return input
}

func responseRole(role string) responses.EasyInputMessageRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return responses.EasyInputMessageRoleAssistant
	case "system":
		return responses.EasyInputMessageRoleSystem
	case "developer":
		return responses.EasyInputMessageRoleDeveloper
	default:
		return responses.EasyInputMessageRoleUser
	}
}

func buildChatMessages(systemPrompt string, messages []ChatMessage) []openai.ChatCompletionMessageParamUnion {
	msgs := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}
	for _, m := range messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(m.Role)) {
		case "assistant":
			msgs = append(msgs, openai.AssistantMessage(content))
		case "system":
			msgs = append(msgs, openai.SystemMessage(content))
		case "developer":
			msgs = append(msgs, openai.DeveloperMessage(content))
		default:
			msgs = append(msgs, openai.UserMessage(content))
		}
	}
	return msgs
}

func buildResponsesTools(tools []FunctionToolDefinition) []responses.ToolUnionParam {
	out := make([]responses.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		params := normalizeToolParameters(tool.Parameters)
		out = append(out, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  params,
				Strict:      openai.Bool(true),
			},
		})
	}
	return out
}

func buildChatTools(tools []FunctionToolDefinition) []openai.ChatCompletionToolUnionParam {
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		params := normalizeToolParameters(tool.Parameters)
		out = append(out, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  shared.FunctionParameters(params),
			Strict:      openai.Bool(true),
		}))
	}
	return out
}

func normalizeToolParameters(raw any) map[string]any {
	params, ok := raw.(map[string]any)
	if !ok || params == nil {
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		}
	}
	out := deepCopyMap(params)
	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}
	props, _ := out["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
		out["properties"] = props
	}
	if _, ok := out["required"]; !ok {
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out["required"] = keys
	}
	out["additionalProperties"] = false
	return out
}

func deepCopyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if nested, ok := v.(map[string]any); ok {
			out[k] = deepCopyMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}

func responseFunctionCallFromItem(item responses.ResponseOutputItemUnion) FunctionToolCall {
	if item.Type != "function_call" {
		return FunctionToolCall{}
	}
	call := item.AsFunctionCall()
	id := strings.TrimSpace(call.ID)
	if id == "" {
		id = call.CallID
	}
	return FunctionToolCall{
		ID:        id,
		CallID:    call.CallID,
		Name:      call.Name,
		Arguments: call.Arguments,
	}
}

func assistantToolMessage(content string, calls []FunctionToolCall) openai.ChatCompletionMessageParamUnion {
	assistant := openai.ChatCompletionAssistantMessageParam{}
	if strings.TrimSpace(content) != "" {
		assistant.Content.OfString = openai.String(content)
	}
	for _, call := range calls {
		assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: call.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      call.Name,
					Arguments: call.Arguments,
				},
			},
		})
	}
	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func sortedToolCalls(toolCallMap map[int64]*FunctionToolCall) []FunctionToolCall {
	if len(toolCallMap) == 0 {
		return nil
	}
	keys := make([]int64, 0, len(toolCallMap))
	for k := range toolCallMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	calls := make([]FunctionToolCall, 0, len(keys))
	for _, key := range keys {
		if call := toolCallMap[key]; call != nil && strings.TrimSpace(call.Name) != "" {
			if call.ID == "" {
				call.ID = fmt.Sprintf("tool_call_%d_%d", time.Now().UnixNano(), key)
			}
			if call.CallID == "" {
				call.CallID = call.ID
			}
			calls = append(calls, *call)
		}
	}
	return calls
}

func executeToolCall(executor ToolExecutor, call FunctionToolCall) (ToolExecutionResult, int64) {
	if executor == nil {
		return ToolExecutionResult{Output: "ERROR: tool executor unavailable"}, 0
	}
	start := timeNow()
	result := executor(call)
	return result, timeNow() - start
}

func extractChatReasoningDelta(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	for _, path := range []string{"reasoning_content", "reasoning.delta", "reasoning"} {
		if val := gjson.Get(rawJSON, path); val.Exists() && val.String() != "" {
			return val.String()
		}
	}
	return ""
}

func shouldFallbackToChat(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "unsupported") ||
		strings.Contains(msg, "unknown url") ||
		strings.Contains(msg, "responses") ||
		strings.Contains(msg, "invalid_request_error")
}

func providerName(usedResponses bool) string {
	if usedResponses {
		return "responses"
	}
	return "chat_fallback"
}

func makeRoundID(round int) string {
	return fmt.Sprintf("round_%d_%d", round, time.Now().UnixNano())
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...(截断)"
	}
	return s
}

// timeNow 返回当前毫秒时间戳（便于计算工具执行耗时）
func timeNow() int64 {
	return time.Now().UnixMilli()
}
