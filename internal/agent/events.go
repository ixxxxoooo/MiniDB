// Package agent 提供 AI 助手的核心抽象：事件协议、工具注册表（单一事实来源）、
// 结构化工具结果，以及把事件流折叠成"有序消息片段"的 Transcript。
package agent

// ProtocolVersion 事件协议版本号。
const ProtocolVersion = 3

// EventType 结构化事件类型：一次 run 的生命周期 + 推理 + 工具 + 文本。
type EventType string

const (
	EventRunStarted    EventType = "run.started"    // payload: RunStarted
	EventStepThinking  EventType = "step.thinking"  // payload: string（推理增量）
	EventToolRequested EventType = "tool.requested" // payload: ToolCall（参数完整，工具尚未执行）
	EventToolResult    EventType = "tool.result"    // payload: ToolResultEvent
	EventAnswerDelta   EventType = "answer.delta"   // payload: string（文本增量，按发生顺序与工具事件交错）
	EventRunDone       EventType = "run.done"       // payload: RunDone
	EventRunError      EventType = "run.error"      // payload: RunError
)

// 错误码：RunError.Code。
const (
	ErrCodeCancelled   = "cancelled"
	ErrCodeRateLimited = "rate_limited"
	ErrCodeEmptyAnswer = "empty_answer"
	ErrCodeFailed      = "stream_failed"
)

// Event 是跨层共享的事件载体。
type Event struct {
	Version int       `json:"v"`
	RunID   string    `json:"runId"`
	Seq     int64     `json:"seq"` // 单调递增序号
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// RunStarted run 开始的载荷。
type RunStarted struct {
	Model  string `json:"model"`
	DBType string `json:"dbType,omitempty"`
}

// ToolCall 工具调用的载荷。
type ToolCall struct {
	CallID    string         `json:"callId"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResultEvent 工具执行结果的载荷（结构化，界面直接渲染）。
type ToolResultEvent struct {
	CallID     string           `json:"callId"`
	ToolName   string           `json:"toolName"`
	OK         bool             `json:"ok"`
	Kind       string           `json:"kind"`              // rows | text | error
	Summary    string           `json:"summary,omitempty"` // 一句话摘要，如 "3 行"、"共 12 张表"
	Columns    []string         `json:"columns,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	Text       string           `json:"text,omitempty"`
	Notes      string           `json:"notes,omitempty"` // 补充说明，显示在表格下方
	Hint       string           `json:"hint,omitempty"`  // 失败时给模型（和用户）的下一步建议
	SQL        string           `json:"sql,omitempty"`   // 触发本次结果的 SQL
	Truncated  bool             `json:"truncated,omitempty"`
	DurationMs int64            `json:"durationMs"`
	ErrorCode  string           `json:"errorCode,omitempty"`
}

// RunDone run 完成的载荷。Content 是最终回答（已去掉下一步建议块）。
type RunDone struct {
	Content     string       `json:"content"`
	Rounds      int          `json:"rounds"`
	Usage       Usage        `json:"usage"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// Usage 用量统计。
type Usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
	DurationMs   int64 `json:"durationMs"`
}

// Suggestion 下一步建议按钮。
type Suggestion struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

// RunError run 失败的载荷。
type RunError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}
