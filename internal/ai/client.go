package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

// ModelName 返回当前配置模型名（小写、去空格）
func (c *Client) ModelName() string {
	if c.config == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.config.Model))
}

// IsReasoningModel 判断当前模型是否属于推理模型。
// 用于决定是否需要在服务层补充“模拟思考”事件。
func (c *Client) IsReasoningModel() bool {
	return isReasoningModelName(c.ModelName())
}

func isReasoningModelName(model string) bool {
	if model == "" {
		return false
	}
	// OpenAI o 系列推理模型
	if strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
		return true
	}
	// 常见推理模型/网关命名
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

	resp, err := client.CreateChatCompletion(ctx, c.newChatCompletionRequest(
		[]openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: finalSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		false,
	))
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

var dsmlFunctionCallsBlockRe = regexp.MustCompile(`(?is)<\s*[\|｜]\s*DSML\s*[\|｜]\s*(?:function_calls|tool_calls)\s*>[\s\S]*?<\s*/\s*[\|｜]\s*DSML\s*[\|｜]\s*(?:function_calls|tool_calls)\s*>`)
var dsmlFunctionCallsOpenToEndRe = regexp.MustCompile(`(?is)<\s*[\|｜]\s*DSML\s*[\|｜]\s*(?:function_calls|tool_calls)\s*>[\s\S]*$`)
var dsmlTagLineRe = regexp.MustCompile(`(?im)^\s*<\s*/?\s*[\|｜]\s*DSML\s*[\|｜].*$`)
var dsmlInvokeRe = regexp.MustCompile(`(?is)<\s*[\|｜]\s*DSML\s*[\|｜]\s*invoke\s+name\s*=\s*"([^"]+)"\s*>(.*?)<\s*/\s*[\|｜]\s*DSML\s*[\|｜]\s*invoke\s*>`)
var dsmlParameterRe = regexp.MustCompile(`(?is)<\s*[\|｜]\s*DSML\s*[\|｜]\s*parameter\s+name\s*=\s*"([^"]+)"\s+string\s*=\s*"(true|false)"\s*>(.*?)<\s*/\s*[\|｜]\s*DSML\s*[\|｜]\s*parameter\s*>`)

// sanitizeThinkingFallback 清洗工具轮次中可能混入的 DSML/函数调用协议文本，避免泄漏到思考展示。
func sanitizeThinkingFallback(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	text = dsmlFunctionCallsBlockRe.ReplaceAllString(text, "")
	text = dsmlFunctionCallsOpenToEndRe.ReplaceAllString(text, "")
	text = dsmlTagLineRe.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			cleaned = append(cleaned, line)
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "<") &&
			(strings.Contains(lower, "dsml") ||
				strings.Contains(lower, "function_calls") ||
				strings.Contains(lower, "tool_calls") ||
				strings.Contains(lower, "invoke name=") ||
				strings.Contains(lower, "parameter name=")) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// parseDSMLFunctionCalls 在模型未触发原生 tool_calls 时，兼容解析其输出的 DSML 协议函数调用。
func parseDSMLFunctionCalls(content string) []FunctionToolCall {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil
	}
	block := dsmlFunctionCallsBlockRe.FindString(text)
	if block == "" {
		block = dsmlFunctionCallsOpenToEndRe.FindString(text)
	}
	if block == "" {
		return nil
	}

	invokes := dsmlInvokeRe.FindAllStringSubmatch(block, -1)
	if len(invokes) == 0 {
		return nil
	}

	calls := make([]FunctionToolCall, 0, len(invokes))
	for idx, inv := range invokes {
		name := strings.TrimSpace(inv[1])
		body := inv[2]
		if name == "" {
			continue
		}

		args := map[string]any{}
		params := dsmlParameterRe.FindAllStringSubmatch(body, -1)
		for _, p := range params {
			paramName := strings.TrimSpace(p[1])
			isString := strings.EqualFold(strings.TrimSpace(p[2]), "true")
			rawValue := strings.TrimSpace(p[3])
			if paramName == "" {
				continue
			}
			if isString {
				args[paramName] = rawValue
				continue
			}
			var decoded any
			if err := json.Unmarshal([]byte(rawValue), &decoded); err == nil {
				args[paramName] = decoded
			} else {
				args[paramName] = rawValue
			}
		}

		argBytes, err := json.Marshal(args)
		if err != nil {
			argBytes = []byte("{}")
		}
		callID := fmt.Sprintf("dsml_call_%d_%d", time.Now().UnixNano(), idx+1)
		calls = append(calls, FunctionToolCall{
			ID:        callID,
			Name:      name,
			Arguments: string(argBytes),
		})
	}
	return calls
}

func emitDeltaProgressively(ctx context.Context, content string, callbacks ToolStreamCallbacks) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}
	if callbacks.OnFinalAnswer != nil {
		callbacks.OnFinalAnswer()
	}
	if callbacks.OnDelta == nil {
		return
	}

	// 按小块推送，避免最终答案“整段突然出现”，恢复逐步流式观感。
	runes := []rune(trimmed)
	const chunkSize = 20
	const frameDelay = 5 * time.Millisecond
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		callbacks.OnDelta(string(runes[i:end]))
		if end < len(runes) {
			time.Sleep(frameDelay)
		}
	}
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

	req := c.newChatCompletionRequest(msgs, false)

	// 记录请求详情
	logger.Info("[AI] ChatWithMessages 请求: model=%s baseURL=%s messages_count=%d provider_params=default",
		c.config.Model, c.config.BaseURL, len(msgs))
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

	req := c.newChatCompletionRequest(msgs, true)

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

		req := c.newChatCompletionRequest(msgs, true)
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

		// 收集本轮流式响应中的 content / reasoning_content / tool_calls
		var contentBuf strings.Builder
		var reasoningBuf strings.Builder
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
			// 处理推理内容 delta（reasoning model / 兼容网关）
			if choice.Delta.ReasoningContent != "" {
				reasoningBuf.WriteString(choice.Delta.ReasoningContent)
				if callbacks.OnThinking != nil {
					callbacks.OnThinking(choice.Delta.ReasoningContent)
				}
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
		reasoning := strings.TrimSpace(reasoningBuf.String())
		logger.Info("[AI] ChatWithToolsStream 第 %d 轮完成: finish=%s content_len=%d tool_calls=%d",
			round, finishReason, len(content), len(toolCallMap))

		// finish_reason == tool_calls：AI 要求调用工具
		if finishReason == openai.FinishReasonToolCalls && len(toolCallMap) > 0 {
			// 兜底：若模型未提供 reasoning_content，则尝试把本轮 content 作为思考内容
			if reasoning == "" {
				if trimmed := strings.TrimSpace(content); trimmed != "" {
					logger.Debug("[AI] ChatWithToolsStream 第 %d 轮思考内容(兜底): %s", round, truncateStr(trimmed, 200))
					if callbacks.OnThinking != nil {
						callbacks.OnThinking(trimmed)
					}
				}
			} else {
				logger.Debug("[AI] ChatWithToolsStream 第 %d 轮 reasoning_content_len=%d", round, len(reasoning))
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

		req := c.newChatCompletionRequest(msgs, true)
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
		var reasoningBuf strings.Builder
		toolCallMap := make(map[int]*openai.ToolCall)
		var finishReason openai.FinishReason
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

			// 真流式优先：只要当前尚未出现 tool_calls，就立即把 delta 下发给前端
			// 注意：若该轮最终变为 tool_calls，少量规划文本可能先行展示（由前端过滤器兜底协议噪音）
			if choice.Delta.Content != "" {
				contentBuf.WriteString(choice.Delta.Content)
				if len(toolCallMap) == 0 {
					if !finalAnswerEmitted {
						finalAnswerEmitted = true
						if callbacks.OnFinalAnswer != nil {
							callbacks.OnFinalAnswer()
						}
					}
					if callbacks.OnDelta != nil {
						callbacks.OnDelta(choice.Delta.Content)
					}
				}
			}
			// 处理推理内容 delta（reasoning model / 兼容网关）
			if choice.Delta.ReasoningContent != "" {
				reasoningBuf.WriteString(choice.Delta.ReasoningContent)
				if callbacks.OnThinking != nil {
					callbacks.OnThinking(choice.Delta.ReasoningContent)
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
		reasoning := strings.TrimSpace(reasoningBuf.String())
		dsmlCalls := parseDSMLFunctionCalls(content)
		if len(dsmlCalls) > 0 {
			logger.Warn("[AI] ChatWithToolsStreamRealtime 第 %d 轮检测到 DSML 函数调用文本，回退执行: count=%d", round, len(dsmlCalls))
		}
		logger.Info("[AI] ChatWithToolsStreamRealtime 第 %d 轮完成: finish=%s content_len=%d tool_calls=%d",
			round, finishReason, len(content), len(toolCallMap))

		// tool_calls 轮次
		if (finishReason == openai.FinishReasonToolCalls && len(toolCallMap) > 0) || len(dsmlCalls) > 0 {
			// 兜底：若模型未提供 reasoning_content，则尝试把本轮 content 作为思考内容
			if reasoning == "" {
				if trimmed := sanitizeThinkingFallback(content); trimmed != "" {
					if callbacks.OnThinking != nil {
						callbacks.OnThinking(trimmed)
					}
				}
			} else {
				logger.Debug("[AI] ChatWithToolsStreamRealtime 第 %d 轮 reasoning_content_len=%d", round, len(reasoning))
			}

			var sortedCalls []openai.ToolCall
			if len(dsmlCalls) > 0 {
				for _, call := range dsmlCalls {
					sortedCalls = append(sortedCalls, openai.ToolCall{
						ID:   call.ID,
						Type: "function",
						Function: openai.FunctionCall{
							Name:      call.Name,
							Arguments: call.Arguments,
						},
					})
				}
			} else {
				for i := 0; i < len(toolCallMap); i++ {
					if tc, ok := toolCallMap[i]; ok {
						sortedCalls = append(sortedCalls, *tc)
					}
				}
			}
			assistantMsg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   sanitizeThinkingFallback(content),
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
			// 已到达最大轮次但仍在调用工具：强制进入“无工具最终总结”兜底，避免空回答
			if round == maxRounds {
				logger.Warn("[AI] ChatWithToolsStreamRealtime 已达工具轮次上限(%d)，触发无工具总结兜底", maxRounds)
				return c.finalizeAfterToolLimit(ctx, msgs, callbacks)
			}
			continue
		}

		// 兜底：若本轮是最终回答但流中没有成功逐段推送，则补发完整内容
		if !finalAnswerEmitted && content != "" {
			emitDeltaProgressively(ctx, content, callbacks)
		}

		logger.Info("[AI] ChatWithToolsStreamRealtime 完成: 共 %d 轮, 最终回答长度=%d", round, len(content))
		return strings.TrimSpace(content), nil
	}

	logger.Warn("[AI] ChatWithToolsStreamRealtime 超过最大轮次限制 %d", maxRounds)
	return c.finalizeAfterToolLimit(ctx, msgs, callbacks)
}

// finalizeAfterToolLimit 在工具轮次耗尽时，强制模型“停止继续调工具并给出最终结论”。
func (c *Client) finalizeAfterToolLimit(
	ctx context.Context,
	msgs []openai.ChatCompletionMessage,
	callbacks ToolStreamCallbacks,
) (string, error) {
	client := c.buildOpenAIClient()
	finalizeHint := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: "你已达到工具调用轮次上限。禁止继续调用任何工具。请基于已有工具结果直接给出最终回答。" +
			"如果已有工具返回 ERROR，请先简要说明根因并给出修复后的 SQL，再给出结论与建议。",
	}
	finalMsgs := append(append([]openai.ChatCompletionMessage{}, msgs...), finalizeHint)

	req := c.newChatCompletionRequest(finalMsgs, false)

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Error("[AI] finalizeAfterToolLimit 请求失败: %v", err)
		return "", nil
	}
	if len(resp.Choices) == 0 {
		logger.Warn("[AI] finalizeAfterToolLimit 返回空 choices")
		return "", nil
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		logger.Warn("[AI] finalizeAfterToolLimit 返回空内容")
		return "", nil
	}

	emitDeltaProgressively(ctx, content, callbacks)
	logger.Info("[AI] finalizeAfterToolLimit 成功: len=%d", len(content))
	return content, nil
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

// newChatCompletionRequest 使用服务端默认采样参数，避免本地硬编码影响兼容网关。
func (c *Client) newChatCompletionRequest(msgs []openai.ChatCompletionMessage, stream bool) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:    c.config.Model,
		Messages: msgs,
		Stream:   stream,
	}
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
