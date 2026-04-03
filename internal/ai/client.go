package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"tableplus-ai/internal/logger"
	"time"

	openai "github.com/sashabaranov/go-openai"
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

// headerTransport 自定义 HTTP Transport，注入额外请求头
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// Client AI 客户端
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

// Chat 发送聊天请求
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	finalSystemPrompt := systemPrompt
	if strings.TrimSpace(c.config.SystemPrompt) != "" {
		// 关键节点：全局会话提示词统一注入，确保所有 AI 能力遵循同一约束
		finalSystemPrompt = finalSystemPrompt + "\n\n用户自定义会话提示词:\n" + strings.TrimSpace(c.config.SystemPrompt)
	}

	logger.Info("[AI] Chat 请求: model=%s baseURL=%s userMessage_len=%d", c.config.Model, c.config.BaseURL, len(userMessage))
	logger.Debug("[AI] Chat systemPrompt(截断): %s", truncateStr(finalSystemPrompt, 200))
	logger.Debug("[AI] Chat userMessage: %s", truncateStr(userMessage, 300))

	temp := float32(c.config.Temperature)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: finalSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		MaxTokens:   c.config.MaxTokens,
		Temperature: temp,
	})
	if err != nil {
		logger.Error("[AI] Chat 请求失败: %v", err)
		return "", fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		logger.Error("[AI] Chat 返回空结果")
		return "", fmt.Errorf("AI 返回空结果")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Info("[AI] Chat 成功: response_len=%d tokens={prompt=%d, completion=%d}",
		len(result), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	logger.Debug("[AI] Chat 响应: %s", truncateStr(result, 500))
	return result, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...(截断)"
	}
	return s
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
	Name      string
	Arguments string
}

// FunctionToolPlanResult 表示一次工具规划响应
type FunctionToolPlanResult struct {
	Content      string
	FinishReason string
	ToolCalls    []FunctionToolCall
}

// ChatWithMessages 支持多轮对话的聊天接口
func (c *Client) ChatWithMessages(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	var msgs []openai.ChatCompletionMessage
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem, Content: systemPrompt,
	})
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    msgs,
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
	}

	// 记录请求详情
	logger.Info("[AI] ChatWithMessages 请求: model=%s baseURL=%s messages_count=%d maxTokens=%d temperature=%.2f",
		c.config.Model, c.config.BaseURL, len(msgs), c.config.MaxTokens, c.config.Temperature)
	for i, m := range msgs {
		contentPreview := m.Content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "...(截断)"
		}
		logger.Debug("[AI] 消息[%d] role=%s content=%s", i, m.Role, contentPreview)
	}

	// 序列化请求体用于调试
	if reqJSON, err := json.Marshal(req); err == nil {
		if len(reqJSON) > 2000 {
			logger.Debug("[AI] 请求体(截断): %s...", string(reqJSON[:2000]))
		} else {
			logger.Debug("[AI] 请求体: %s", string(reqJSON))
		}
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Error("[AI] ChatWithMessages 请求失败: %v", err)
		return "", fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		logger.Error("[AI] ChatWithMessages 返回空结果, usage=%+v", resp.Usage)
		return "", fmt.Errorf("AI 返回空结果")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Info("[AI] ChatWithMessages 成功: response_len=%d usage={prompt_tokens=%d, completion_tokens=%d, total_tokens=%d}",
		len(result), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	if len(result) > 500 {
		logger.Debug("[AI] 响应内容(截断): %s...", result[:500])
	} else {
		logger.Debug("[AI] 响应内容: %s", result)
	}

	return result, nil
}

// ChatWithMessagesStream 支持流式返回的聊天接口
func (c *Client) ChatWithMessagesStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	onDelta func(delta string),
) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}

	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}

	client := openai.NewClientWithConfig(clientConfig)

	var msgs []openai.ChatCompletionMessage
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem, Content: systemPrompt,
	})
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    msgs,
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
		Stream:      true,
	}

	logger.Info("[AI] ChatWithMessagesStream 请求: model=%s baseURL=%s messages_count=%d",
		c.config.Model, c.config.BaseURL, len(msgs))

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		logger.Error("[AI] ChatWithMessagesStream 创建流失败: %v", err)
		return "", fmt.Errorf("AI 流式请求失败: %w", err)
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		response, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			logger.Error("[AI] ChatWithMessagesStream 接收失败: %v", recvErr)
			return "", fmt.Errorf("AI 流式响应失败: %w", recvErr)
		}
		if len(response.Choices) == 0 {
			continue
		}

		delta := response.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		sb.WriteString(delta)
		if onDelta != nil {
			onDelta(delta)
		}
	}

	result := strings.TrimSpace(sb.String())
	logger.Info("[AI] ChatWithMessagesStream 成功: response_len=%d", len(result))
	return result, nil
}

// ToolExecutor 工具执行回调：接收 AI 返回的 tool_call，返回执行结果字符串
type ToolExecutor func(call FunctionToolCall) string

// ToolStreamCallbacks ReAct 多轮流式对话的回调集合
type ToolStreamCallbacks struct {
	// OnThinking AI 在工具调用间输出的思考/分析内容
	OnThinking func(content string)
	// OnToolCall AI 决定调用某个工具时触发（工具尚未执行）
	OnToolCall func(call FunctionToolCall)
	// OnToolResult 工具执行完成后触发
	OnToolResult func(callID, toolName, result string, durationMs int64)
	// OnDelta 最终回答阶段的流式文本片段
	OnDelta func(delta string)
	// OnFinalAnswer 最终回答阶段开始时触发（finish_reason=stop 的那一轮开始）
	OnFinalAnswer func()
}

// ChatWithToolsStream ReAct 多轮流式对话：AI 在同一上下文中边思考边调用工具边分析
// tools 为空时退化为普通流式对话（不带工具定义）
func (c *Client) ChatWithToolsStream(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	client := c.buildOpenAIClient()

	// 构建初始消息列表
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	// 构建 OpenAI 工具定义
	openaiTools := buildOpenAITools(tools)

	logger.Info("[AI] ChatWithToolsStream 开始: model=%s rounds_limit=%d tools=%d messages=%d",
		c.config.Model, maxRounds, len(openaiTools), len(msgs))

	var finalContent strings.Builder

	for round := 1; round <= maxRounds; round++ {
		logger.Info("[AI] ChatWithToolsStream 第 %d 轮开始, messages=%d", round, len(msgs))

		req := openai.ChatCompletionRequest{
			Model:       c.config.Model,
			Messages:    msgs,
			MaxTokens:   c.config.MaxTokens,
			Temperature: float32(c.config.Temperature),
			Stream:      true,
		}
		// 仅在有工具定义时传入 tools 参数，避免空工具列表导致 API 报错
		if len(openaiTools) > 0 {
			req.Tools = openaiTools
			req.ToolChoice = "auto"
			req.ParallelToolCalls = false
		}

		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			logger.Error("[AI] ChatWithToolsStream 第 %d 轮创建流失败: %v", round, err)
			return "", fmt.Errorf("AI 流式请求失败: %w", err)
		}

		// 收集本轮流式响应中的 content 和 tool_calls
		var contentBuf strings.Builder
		toolCallMap := make(map[int]*openai.ToolCall)
		var finishReason openai.FinishReason

		for {
			response, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				stream.Close()
				logger.Error("[AI] ChatWithToolsStream 第 %d 轮接收失败: %v", round, recvErr)
				return "", fmt.Errorf("AI 流式响应失败: %w", recvErr)
			}
			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			finishReason = choice.FinishReason

			// 处理文本内容 delta
			if choice.Delta.Content != "" {
				contentBuf.WriteString(choice.Delta.Content)
			}

			// 处理 tool_calls delta（增量拼接：流式中每个 delta 可能只包含 tool name/arguments 的一部分）
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				existing, ok := toolCallMap[idx]
				if !ok {
					// 新的 tool_call 开始
					newCall := openai.ToolCall{
						Index: tc.Index,
						ID:    tc.ID,
						Type:  tc.Type,
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
					toolCallMap[idx] = &newCall
				} else {
					// 增量拼接 ID、函数名、参数
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Function.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		stream.Close()

		content := contentBuf.String()
		logger.Info("[AI] ChatWithToolsStream 第 %d 轮完成: finish=%s content_len=%d tool_calls=%d",
			round, finishReason, len(content), len(toolCallMap))

		// finish_reason == tool_calls：AI 要求调用工具
		if finishReason == openai.FinishReasonToolCalls && len(toolCallMap) > 0 {
			// AI 在调用工具前可能输出了思考内容，推送给前端
			if trimmed := strings.TrimSpace(content); trimmed != "" {
				logger.Debug("[AI] ChatWithToolsStream 第 %d 轮思考内容: %s", round, truncateStr(trimmed, 200))
				if callbacks.OnThinking != nil {
					callbacks.OnThinking(trimmed)
				}
			}

			// 将 assistant 消息（含 tool_calls）追加到对话历史
			var sortedCalls []openai.ToolCall
			for i := 0; i < len(toolCallMap); i++ {
				if tc, ok := toolCallMap[i]; ok {
					sortedCalls = append(sortedCalls, *tc)
				}
			}
			assistantMsg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   content,
				ToolCalls: sortedCalls,
			}
			msgs = append(msgs, assistantMsg)

			// 逐个执行工具并追加 tool 消息
			for _, tc := range sortedCalls {
				call := FunctionToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
				logger.Info("[AI] ChatWithToolsStream 执行工具: round=%d tool=%s id=%s", round, call.Name, call.ID)
				if callbacks.OnToolCall != nil {
					callbacks.OnToolCall(call)
				}

				// 执行工具（由调用方提供的 executor 实际执行）
				toolResult := ""
				var durationMs int64
				if executor != nil {
					start := timeNow()
					toolResult = executor(call)
					durationMs = timeNow() - start
				}

				if callbacks.OnToolResult != nil {
					callbacks.OnToolResult(call.ID, call.Name, toolResult, durationMs)
				}

				// 追加 tool 角色消息到对话历史
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}
			// 继续下一轮
			continue
		}

		// finish_reason == stop 或其他：AI 输出最终回答
		if callbacks.OnFinalAnswer != nil {
			callbacks.OnFinalAnswer()
		}

		// 如果最终轮有 content 但还没有通过 onDelta 推送过（非流式场景兜底），直接推送
		// 正常流式场景下 content 已经在上面的循环中逐 delta 推送了
		// 但我们需要重新流式获取最终回答，因为上面收集的是完整 content
		// 对于 stop 轮次：content 就是最终回答，需要通过 onDelta 推送
		if content != "" && callbacks.OnDelta != nil {
			callbacks.OnDelta(content)
		}
		finalContent.WriteString(content)

		logger.Info("[AI] ChatWithToolsStream 完成: 共 %d 轮, 最终回答长度=%d", round, finalContent.Len())
		return strings.TrimSpace(finalContent.String()), nil
	}

	// 超过最大轮次限制
	logger.Warn("[AI] ChatWithToolsStream 超过最大轮次限制 %d", maxRounds)
	return strings.TrimSpace(finalContent.String()), nil
}

// ChatWithToolsStreamRealtime 与 ChatWithToolsStream 类似，但最终回答阶段采用真正的流式推送
// 在工具调用轮次收集完整内容，在最终回答轮次逐 delta 推送
func (c *Client) ChatWithToolsStreamRealtime(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []FunctionToolDefinition,
	maxRounds int,
	executor ToolExecutor,
	callbacks ToolStreamCallbacks,
) (string, error) {
	if c.config == nil {
		return "", fmt.Errorf("AI 未配置，请先在设置中配置 AI 服务")
	}

	client := c.buildOpenAIClient()

	// 构建初始消息列表
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	for _, m := range messages {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role: m.Role, Content: m.Content,
		})
	}

	openaiTools := buildOpenAITools(tools)

	logger.Info("[AI] ChatWithToolsStreamRealtime 开始: model=%s rounds_limit=%d tools=%d messages=%d",
		c.config.Model, maxRounds, len(openaiTools), len(msgs))

	for round := 1; round <= maxRounds; round++ {
		logger.Info("[AI] ChatWithToolsStreamRealtime 第 %d 轮开始, messages=%d", round, len(msgs))

		req := openai.ChatCompletionRequest{
			Model:       c.config.Model,
			Messages:    msgs,
			MaxTokens:   c.config.MaxTokens,
			Temperature: float32(c.config.Temperature),
			Stream:      true,
		}
		if len(openaiTools) > 0 {
			req.Tools = openaiTools
			req.ToolChoice = "auto"
			req.ParallelToolCalls = false
		}

		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			logger.Error("[AI] ChatWithToolsStreamRealtime 第 %d 轮创建流失败: %v", round, err)
			return "", fmt.Errorf("AI 流式请求失败: %w", err)
		}

		var contentBuf strings.Builder
		toolCallMap := make(map[int]*openai.ToolCall)
		var finishReason openai.FinishReason
		isFinalRound := false
		finalAnswerEmitted := false

		for {
			response, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				stream.Close()
				logger.Error("[AI] ChatWithToolsStreamRealtime 第 %d 轮接收失败: %v", round, recvErr)
				return "", fmt.Errorf("AI 流式响应失败: %w", recvErr)
			}
			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			finishReason = choice.FinishReason

			// 处理文本 delta
			if choice.Delta.Content != "" {
				contentBuf.WriteString(choice.Delta.Content)

				// 如果没有 tool_calls 在累积中，说明这可能是最终回答轮次，实时推送 delta
				if len(toolCallMap) == 0 {
					if !finalAnswerEmitted {
						finalAnswerEmitted = true
						isFinalRound = true
						if callbacks.OnFinalAnswer != nil {
							callbacks.OnFinalAnswer()
						}
					}
					if callbacks.OnDelta != nil {
						callbacks.OnDelta(choice.Delta.Content)
					}
				}
			}

			// 处理 tool_calls delta
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				existing, ok := toolCallMap[idx]
				if !ok {
					newCall := openai.ToolCall{
						Index: tc.Index,
						ID:    tc.ID,
						Type:  tc.Type,
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
					toolCallMap[idx] = &newCall
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Function.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		stream.Close()

		content := contentBuf.String()
		logger.Info("[AI] ChatWithToolsStreamRealtime 第 %d 轮完成: finish=%s content_len=%d tool_calls=%d final=%v",
			round, finishReason, len(content), len(toolCallMap), isFinalRound)

		// tool_calls 轮次
		if finishReason == openai.FinishReasonToolCalls && len(toolCallMap) > 0 {
			// 推送工具调用前的思考内容
			if trimmed := strings.TrimSpace(content); trimmed != "" {
				if callbacks.OnThinking != nil {
					callbacks.OnThinking(trimmed)
				}
			}

			var sortedCalls []openai.ToolCall
			for i := 0; i < len(toolCallMap); i++ {
				if tc, ok := toolCallMap[i]; ok {
					sortedCalls = append(sortedCalls, *tc)
				}
			}
			assistantMsg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   content,
				ToolCalls: sortedCalls,
			}
			msgs = append(msgs, assistantMsg)

			for _, tc := range sortedCalls {
				call := FunctionToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
				logger.Info("[AI] ChatWithToolsStreamRealtime 执行工具: round=%d tool=%s id=%s", round, call.Name, call.ID)
				if callbacks.OnToolCall != nil {
					callbacks.OnToolCall(call)
				}

				toolResult := ""
				var durationMs int64
				if executor != nil {
					start := timeNow()
					toolResult = executor(call)
					durationMs = timeNow() - start
				}

				if callbacks.OnToolResult != nil {
					callbacks.OnToolResult(call.ID, call.Name, toolResult, durationMs)
				}

				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}
			continue
		}

		// 最终回答轮次（如果之前没通过 delta 推送过，兜底推送）
		if !finalAnswerEmitted && content != "" {
			if callbacks.OnFinalAnswer != nil {
				callbacks.OnFinalAnswer()
			}
			if callbacks.OnDelta != nil {
				callbacks.OnDelta(content)
			}
		}

		logger.Info("[AI] ChatWithToolsStreamRealtime 完成: 共 %d 轮, 最终回答长度=%d", round, len(content))
		return strings.TrimSpace(content), nil
	}

	logger.Warn("[AI] ChatWithToolsStreamRealtime 超过最大轮次限制 %d", maxRounds)
	return "", nil
}

// buildOpenAIClient 构建 OpenAI 客户端实例（提取公共逻辑）
func (c *Client) buildOpenAIClient() *openai.Client {
	apiKey := c.config.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}
	clientConfig := openai.DefaultConfig(apiKey)
	if c.config.BaseURL != "" {
		clientConfig.BaseURL = c.config.BaseURL
	}
	if len(c.config.Headers) > 0 {
		clientConfig.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.config.Headers,
			},
		}
	}
	return openai.NewClientWithConfig(clientConfig)
}

// buildOpenAITools 将内部工具定义转换为 OpenAI SDK 格式
func buildOpenAITools(tools []FunctionToolDefinition) []openai.Tool {
	openaiTools := make([]openai.Tool, 0, len(tools))
	for _, tool := range tools {
		openaiTools = append(openaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return openaiTools
}

// timeNow 返回当前毫秒时间戳（便于计算工具执行耗时）
func timeNow() int64 {
	return time.Now().UnixMilli()
}
