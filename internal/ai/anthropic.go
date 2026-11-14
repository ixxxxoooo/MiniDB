package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"minidb/internal/logger"
)

const (
	anthropicDefaultBase = "https://api.anthropic.com/v1"
	anthropicVersion     = "2023-06-01"
	anthropicMaxTokens   = 8192 // Messages API 必须带 max_tokens；用户没配置时用它
)

// anthropicProvider Anthropic 原生 Messages API：流式输出，工具调用走 tool_use / tool_result 内容块。
type anthropicProvider struct{}

func (anthropicProvider) run(ctx context.Context, cfg *Config, req chatRequest) (string, error) {
	logger.Info("[AI] 开始: provider=anthropic model=%s tools=%d messages=%d rounds_limit=%d", cfg.Model, len(req.tools), len(req.messages), req.maxRounds)
	l, err := newAnthropicLoop(cfg, req.prompt, req.messages, req.tools, req.cb)
	if err != nil {
		return "", err
	}
	return runToolLoop(ctx, l, req.maxRounds, req.executor, req.cb)
}

func (p anthropicProvider) simple(ctx context.Context, cfg *Config, prompt string, messages []ChatMessage) (string, bool, error) {
	out, err := p.run(ctx, cfg, chatRequest{prompt: prompt, messages: messages, maxRounds: 1})
	if err != nil {
		return "", true, fmt.Errorf("AI 请求失败: %w", err)
	}
	return out, true, nil
}

// anthropicMsg 一条 Messages API 消息；Content 是字符串或内容块列表。
type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// anthropicBlock 内容块：text、tool_use（助手发起调用）、tool_result（用户回传工具输出）。
type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

// anthropicLoop Anthropic 通道的传输层：实现 toolLoop，停止规则由 runToolLoop 统一处理。
type anthropicLoop struct {
	cfg    *Config
	system string
	msgs   []anthropicMsg
	tools  []anthropicTool
	cb     ToolStreamCallbacks
}

func newAnthropicLoop(cfg *Config, prompt string, messages []ChatMessage, tools []FunctionToolDefinition, cb ToolStreamCallbacks) (*anthropicLoop, error) {
	system, msgs := buildAnthropicMessages(prompt, messages)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("没有可发送的消息")
	}
	l := &anthropicLoop{cfg: cfg, system: system, msgs: msgs, cb: cb}
	for _, t := range tools {
		l.tools = append(l.tools, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: strictSchema(t.Parameters)})
	}
	return l, nil
}

// buildAnthropicMessages 把系统提示词与会话历史转成 Messages API 的格式：
// system / developer 消息并入系统提示词；相邻同角色消息合并；开头必须是用户消息，所以丢弃开头的助手消息。
func buildAnthropicMessages(prompt string, messages []ChatMessage) (string, []anthropicMsg) {
	system := []string{strings.TrimSpace(prompt)}
	var msgs []anthropicMsg
	for _, m := range messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		role := "user"
		switch strings.ToLower(strings.TrimSpace(m.Role)) {
		case "assistant":
			role = "assistant"
		case "system", "developer":
			system = append(system, content)
			continue
		}
		if len(msgs) == 0 && role == "assistant" {
			continue
		}
		if n := len(msgs); n > 0 && msgs[n-1].Role == role {
			msgs[n-1].Content = msgs[n-1].Content.(string) + "\n\n" + content
			continue
		}
		msgs = append(msgs, anthropicMsg{Role: role, Content: content})
	}
	return strings.TrimSpace(strings.Join(system, "\n\n")), msgs
}

func (l *anthropicLoop) next(ctx context.Context, round int) (turn, error) {
	res, err := streamAnthropic(ctx, l.cfg, l.body(l.system, l.msgs, false), l.cb)
	if err != nil {
		return turn{}, err
	}
	logger.Info("[AI] Anthropic 第 %d 轮完成: stop=%s text_len=%d tool_calls=%d", round, res.stopReason, len(res.text), len(res.calls))
	calls := res.calls
	if res.stopReason == "max_tokens" {
		calls = nil // 被长度截断，工具参数可能不完整，不执行
	}
	return turn{text: res.text, calls: calls}, nil
}

func (l *anthropicLoop) record(t turn, outs []toolOutput) {
	var blocks []anthropicBlock
	if strings.TrimSpace(t.text) != "" {
		blocks = append(blocks, anthropicBlock{Type: "text", Text: t.text})
	}
	for _, call := range t.calls {
		blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: call.ID, Name: call.Name, Input: toolInput(call.Arguments)})
	}
	l.msgs = append(l.msgs, anthropicMsg{Role: "assistant", Content: blocks})
	results := make([]anthropicBlock, 0, len(outs))
	for _, out := range outs {
		results = append(results, anthropicBlock{
			Type: "tool_result", ToolUseID: out.call.ID, Content: out.text, IsError: strings.HasPrefix(out.text, "ERROR"),
		})
	}
	l.msgs = append(l.msgs, anthropicMsg{Role: "user", Content: results})
}

func (l *anthropicLoop) finalize(ctx context.Context) (string, error) {
	// 同一条用户消息里 tool_result 必须排在文字前面
	msgs := append([]anthropicMsg{}, l.msgs...)
	if last := &msgs[len(msgs)-1]; last.Role == "user" {
		if blocks, ok := last.Content.([]anthropicBlock); ok {
			last.Content = append(append([]anthropicBlock{}, blocks...), anthropicBlock{Type: "text", Text: "请基于已有工具结果给出最终回答，不要继续调用工具。"})
		}
	}
	res, err := streamAnthropic(ctx, l.cfg, l.body(l.system+"\n\n"+finalPrompt, msgs, true), l.cb)
	if err != nil {
		return "", fmt.Errorf("AI 最终总结失败: %w", err)
	}
	return strings.TrimSpace(res.text), nil
}

// body 构造一次请求。历史里带 tool_use 时 API 要求必须声明 tools，所以总结轮不去掉工具，而是用 tool_choice=none 禁用。
func (l *anthropicLoop) body(system string, msgs []anthropicMsg, noTools bool) map[string]any {
	body := map[string]any{
		"model": l.cfg.Model, "max_tokens": anthropicMaxTokens, "system": system, "messages": msgs, "stream": true,
	}
	if l.cfg.MaxTokens > 0 {
		body["max_tokens"] = l.cfg.MaxTokens
	}
	if l.cfg.Temperature != 0 {
		body["temperature"] = min(l.cfg.Temperature, 1) // Anthropic 的温度上限是 1
	}
	if len(l.tools) > 0 {
		body["tools"] = l.tools
		if noTools {
			body["tool_choice"] = map[string]any{"type": "none"}
		} else {
			body["tool_choice"] = map[string]any{"type": "auto", "disable_parallel_tool_use": true}
		}
	}
	return body
}

// toolInput 工具参数必须是 JSON 对象；模型给出空串或非法 JSON 时回传空对象。
func toolInput(args string) json.RawMessage {
	args = strings.TrimSpace(args)
	var obj map[string]any
	if args == "" || json.Unmarshal([]byte(args), &obj) != nil || obj == nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(args)
}
