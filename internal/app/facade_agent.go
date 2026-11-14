package app

import (
	"path/filepath"

	"minidb/internal/agent"
	"minidb/internal/appdata"
)

// 助手事件模型的界面侧入口。
//
// 界面层不直接依赖 agent 包：事件、对话记录（Transcript）这些约定从这里以别名的方式取，
// 命名统一带 Agent 前缀。别名与原类型完全等价，所以 app 发出的事件界面可以直接用，没有拷贝。
// 将来 agent 的内部结构要调整时，影响面止于这一个文件。
type (
	AgentEvent      = agent.Event
	AgentEventType  = agent.EventType
	AgentTranscript = agent.Transcript
	AgentPart       = agent.Part
	AgentToolPart   = agent.ToolPart
	AgentToolCall   = agent.ToolCall
	AgentToolResult = agent.ToolResultEvent
	AgentRunDone    = agent.RunDone
	AgentUsage      = agent.Usage
)

// 事件类型、片段类型与其它界面要用到的常量。
const (
	AgentEventToolRequested = agent.EventToolRequested
	AgentEventToolResult    = agent.EventToolResult
	AgentEventStepThinking  = agent.EventStepThinking
	AgentEventAnswerDelta   = agent.EventAnswerDelta
	AgentEventRunDone       = agent.EventRunDone

	AgentPartText     = agent.PartText
	AgentPartThinking = agent.PartThinking

	AgentResultKindRows = agent.ResultKindRows
	AgentSuggestionTag  = agent.SuggestionTag
)

// DataDir 是应用数据目录（设置页里展示和打开）。
func DataDir() string { return appdata.RootDir() }

// LogDir 是日志文件所在目录。
func LogDir() string { return filepath.Dir(appdata.LogFilePath()) }
