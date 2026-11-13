package ai

import "strings"

// ChatMessage 一条会话消息（user / assistant / system / developer）。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FunctionToolDefinition 描述可供模型调用的函数工具。
type FunctionToolDefinition struct {
	Name        string
	Description string
	Parameters  any
}

// FunctionToolCall 模型返回的一次函数调用。
type FunctionToolCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments string
}

// responseCallID 返回回传工具输出时要用的调用标识（Responses 用 call_id）。
func (c FunctionToolCall) responseCallID() string {
	if strings.TrimSpace(c.CallID) != "" {
		return c.CallID
	}
	return c.ID
}

// ToolExecutionResult 工具执行结果：Output 回传给模型，Err 非空表示失败。
type ToolExecutionResult struct {
	Output string
	Err    error
}

// ToolExecutor 工具执行回调：接收模型发起的调用，返回执行结果。
type ToolExecutor func(call FunctionToolCall) ToolExecutionResult

// Usage 一次模型请求消耗的 token。
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// ToolStreamCallbacks 多轮工具循环的事件回调，所有回调均可为空。
//
// 文本与工具调用按真实发生的顺序到达：模型在调用工具前说的话先通过 OnDelta 流出，
// 随后才是 OnToolCall；由使用方决定哪一段文本算"最终回答"（最后一次工具调用之后的文本）。
type ToolStreamCallbacks struct {
	// OnRound 每开始一轮模型请求时触发，round 从 1 起。
	OnRound func(round int)
	// OnThinking 推理摘要/推理文本增量（Responses 的 reasoning summary，或网关的 reasoning_content）。
	OnThinking func(delta string)
	// OnDelta 模型可见文本的增量。
	OnDelta func(delta string)
	// OnToolCall 一次完整的函数调用已确认（参数完整），工具尚未执行。
	OnToolCall func(call FunctionToolCall)
	// OnToolResult 工具执行完成。
	OnToolResult func(callID, toolName, output string, durationMs int64)
	// OnUsage 一次模型请求结束后的 token 用量（网关返回时才触发）。
	OnUsage func(u Usage)
}

func (cb ToolStreamCallbacks) round(n int) {
	if cb.OnRound != nil {
		cb.OnRound(n)
	}
}

func (cb ToolStreamCallbacks) thinking(s string) {
	if s != "" && cb.OnThinking != nil {
		cb.OnThinking(s)
	}
}

func (cb ToolStreamCallbacks) delta(s string) {
	if s != "" && cb.OnDelta != nil {
		cb.OnDelta(s)
	}
}

func (cb ToolStreamCallbacks) usage(u Usage) {
	if u.TotalTokens > 0 && cb.OnUsage != nil {
		cb.OnUsage(u)
	}
}

// finalPrompt 工具轮次耗尽（或工具后模型没有给出回答）时追加的总结指令。
const finalPrompt = "你已达到工具调用轮次上限，禁止继续调用任何工具。请基于已有工具结果直接给出最终回答；" +
	"如果有工具返回 ERROR，先简要说明根因并给出修复后的 SQL，再给出结论与建议。"
