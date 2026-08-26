// Package agent 提供 MiniDB Agent 执行器的核心抽象：
// 事件协议 v2、工具注册表（单一事实来源）、结构化工具结果。
package agent

// ProtocolVersion 事件协议版本号。前端 agentEvents.ts 镜像本文件。
const ProtocolVersion = 2

// EventType 结构化事件类型（run 生命周期 + 工具 + 答案）。
type EventType string

const (
	EventRunStarted     EventType = "run.started"      // payload: RunStarted
	EventStepThinking   EventType = "step.thinking"    // payload: string (delta)
	EventToolRequested  EventType = "tool.requested"   // payload: ToolCall
	EventToolResult     EventType = "tool.result"      // payload: ToolResultEvent
	EventAnswerDelta    EventType = "answer.delta"     // payload: string (delta)
	EventRunDone        EventType = "run.done"         // payload: RunDone
	EventRunError       EventType = "run.error"        // payload: RunError
	EventApproveRequest EventType = "approval.requested" // payload: ApprovalRequest
)

// Event 是跨前后端共享的事件载体。
type Event struct {
	Version int       `json:"v"`         // 协议版本（ProtocolVersion）
	RunID   string    `json:"runId"`     // 一次 run 的唯一标识（对接前端 requestId）
	Seq     int64     `json:"seq"`       // 单调递增序号，前端据此排序
	Type    EventType `json:"type"`      // 事件类型
	Phase   string    `json:"phase,omitempty"` // 可选：planning/tool/answer
	Payload any       `json:"payload"`   // 结构化载荷
}

// EventSink 推送事件的目标（Wails Event / 测试 recorder / 日志）。
type EventSink interface {
	Emit(ev Event)
}

// SinkFunc 便捷适配器：把函数当作 EventSink。
type SinkFunc func(ev Event)

func (f SinkFunc) Emit(ev Event) { f(ev) }

// RunStarted run 开始的载荷。
type RunStarted struct {
	Model      string `json:"model"`
	SchemaMode string `json:"schemaMode,omitempty"` // "full" | "summary" | ""
	DBType     string `json:"dbType,omitempty"`
}

// ToolCall 工具调用的载荷。
type ToolCall struct {
	CallID    string         `json:"callId"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResultEvent 工具执行结果的载荷（结构化，前端直接渲染）。
type ToolResultEvent struct {
	CallID     string            `json:"callId"`
	ToolName   string            `json:"toolName"`
	OK         bool              `json:"ok"`
	Kind       string            `json:"kind"` // "rows" | "text" | "error"
	Columns    []string          `json:"columns,omitempty"`
	Rows       []map[string]any  `json:"rows,omitempty"`
	Text       string            `json:"text,omitempty"`
	Truncated  bool              `json:"truncated,omitempty"`
	DurationMs int64             `json:"durationMs"`
	ErrorCode  string            `json:"errorCode,omitempty"`
	Data       map[string]any    `json:"data,omitempty"` // 额外元数据（select 语句等）
}

// RunDone run 完成的载荷。
type RunDone struct {
	Content     string       `json:"content"`
	Rounds      int          `json:"rounds"`
	Usage       Usage        `json:"usage"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// Usage 用量与成本统计。
type Usage struct {
	PromptTokens     int64 `json:"promptTokens"`
	CompletionTokens int64 `json:"completionTokens"`
	TotalTokens      int64 `json:"totalTokens"`
	DurationMs       int64 `json:"durationMs"`
}

// Suggestion 下一步建议按钮（结构化，来自 run.done）。
type Suggestion struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

// RunError run 失败的载荷。
type RunError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// ApprovalRequest 非只读/高成本操作请求用户确认的载荷（预留）。
type ApprovalRequest struct {
	Action string         `json:"action"`
	Meta   map[string]any `json:"meta,omitempty"`
}