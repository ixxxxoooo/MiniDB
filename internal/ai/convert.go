package ai

import (
	"fmt"
	"sort"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/tidwall/gjson"
)

// responseRole 把角色名映射为 Responses 的消息角色。
func responseRole(role string) responses.EasyInputMessageRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return responses.EasyInputMessageRoleAssistant
	case "system":
		return responses.EasyInputMessageRoleSystem
	case "developer":
		return responses.EasyInputMessageRoleDeveloper
	}
	return responses.EasyInputMessageRoleUser
}

// buildResponseInputMessages 把会话消息转成 Responses 的输入项（跳过空消息）。
func buildResponseInputMessages(messages []ChatMessage) responses.ResponseInputParam {
	input := make(responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		if content := strings.TrimSpace(msg.Content); content != "" {
			input = append(input, responses.ResponseInputItemParamOfMessage(content, responseRole(msg.Role)))
		}
	}
	return input
}

// buildChatMessages 把系统提示词与会话消息转成 Chat Completions 的消息列表。
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

// buildResponsesTools 把工具定义转成 Responses 的函数工具（strict 模式）。
func buildResponsesTools(tools []FunctionToolDefinition) []responses.ToolUnionParam {
	out := make([]responses.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, responses.ToolUnionParam{OfFunction: &responses.FunctionToolParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  strictSchema(t.Parameters),
			Strict:      openai.Bool(true),
		}})
	}
	return out
}

// buildChatTools 把工具定义转成 Chat Completions 的函数工具（strict 模式）。
func buildChatTools(tools []FunctionToolDefinition) []openai.ChatCompletionToolUnionParam {
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  shared.FunctionParameters(strictSchema(t.Parameters)),
			Strict:      openai.Bool(true),
		}))
	}
	return out
}

// strictSchema 保证参数 schema 满足 strict 模式：顶层是 object、禁止多余字段。
// "required" 是否齐全由工具定义方负责（agent 包的 Schema 构造器会把可选参数声明为 nullable）。
func strictSchema(raw any) map[string]any {
	params, _ := raw.(map[string]any)
	if params == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}, "additionalProperties": false}
	}
	out := make(map[string]any, len(params)+2)
	for k, v := range params {
		out[k] = v
	}
	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}
	props, _ := out["properties"].(map[string]any)
	if props == nil {
		out["properties"] = map[string]any{}
	}
	if _, ok := out["required"]; !ok {
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out["required"] = keys
	}
	out["additionalProperties"] = false
	return out
}

// responseFunctionCall 从 Responses 输出项里取函数调用；不是函数调用时返回 ok=false。
func responseFunctionCall(item responses.ResponseOutputItemUnion) (FunctionToolCall, bool) {
	if item.Type != "function_call" {
		return FunctionToolCall{}, false
	}
	call := item.AsFunctionCall()
	id := strings.TrimSpace(call.ID)
	if id == "" {
		id = call.CallID
	}
	return FunctionToolCall{ID: id, CallID: call.CallID, Name: call.Name, Arguments: call.Arguments}, call.Name != ""
}

// assistantToolMessage 构造"助手发起了这些工具调用"的 Chat 消息，供回传工具结果前加入历史。
// reasoning 是网关随流返回的推理文本：DeepSeek 的思考模式要求在工具循环里把 reasoning_content 原样回传，
// 否则下一轮会报 400；没有推理文本（例如 OpenAI 官方接口）时不附加这个非标准字段。
func assistantToolMessage(content, reasoning string, calls []FunctionToolCall) openai.ChatCompletionMessageParamUnion {
	assistant := openai.ChatCompletionAssistantMessageParam{}
	if strings.TrimSpace(reasoning) != "" {
		assistant.SetExtraFields(map[string]any{"reasoning_content": reasoning})
	}
	if strings.TrimSpace(content) != "" {
		assistant.Content.OfString = openai.String(content)
	}
	for _, call := range calls {
		assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID:       call.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{Name: call.Name, Arguments: call.Arguments},
			},
		})
	}
	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

// sortedToolCalls 把按下标累积的流式工具调用整理成有序列表，补齐缺失的 ID。
func sortedToolCalls(byIndex map[int64]*FunctionToolCall) []FunctionToolCall {
	keys := make([]int64, 0, len(byIndex))
	for k := range byIndex {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	calls := make([]FunctionToolCall, 0, len(keys))
	for _, key := range keys {
		call := byIndex[key]
		if call == nil || strings.TrimSpace(call.Name) == "" {
			continue
		}
		if call.ID == "" {
			call.ID = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), key)
		}
		if call.CallID == "" {
			call.CallID = call.ID
		}
		calls = append(calls, *call)
	}
	return calls
}

// chatReasoningDelta 取 Chat 流式分片里网关附带的推理文本（各家字段名不同）。
func chatReasoningDelta(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	for _, path := range []string{"reasoning_content", "reasoning.delta", "reasoning"} {
		if v := gjson.Get(rawJSON, path); v.Exists() && v.String() != "" {
			return v.String()
		}
	}
	return ""
}
