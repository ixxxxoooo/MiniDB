package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/logger"
)

// maxEventRows 一条 tool.result 事件带给界面的行数上限；模型看到的是更小的 agent.ModelRowsDefault。
const maxEventRows = 200

// persistRows / persistCell 会话里保存的工具结果上限，避免一次大查询撑大存储。
const (
	persistRows = 100
	persistCell = 300
)

// chatRun 一次对话 run 的状态：事件序号、折叠出的 Transcript、用量。
// 事件先折叠进 Transcript（用于落盘）再发给界面，所以保存的内容与界面实时看到的一致。
//
// @author ixxxxoooo
type chatRun struct {
	s         *AIService
	id        string
	sessionID string
	startedAt time.Time

	mu     sync.Mutex
	seq    int64
	tr     agent.Transcript
	usage  ai.Usage
	rounds int
}

func newChatRun(s *AIService, id, sessionID string) *chatRun {
	return &chatRun{s: s, id: id, sessionID: sessionID, startedAt: time.Now()}
}

// emit 折叠并发出一个事件。
func (r *chatRun) emit(typ agent.EventType, payload any) {
	r.mu.Lock()
	r.seq++
	ev := agent.Event{Version: agent.ProtocolVersion, RunID: r.id, Seq: r.seq, Type: typ, Payload: payload}
	r.tr.Apply(ev)
	r.mu.Unlock()
	if r.s.env == nil {
		logger.Warn("[AIService] 事件总线未注入，丢弃事件: run=%s type=%s", r.id, typ)
		return
	}
	r.s.env.Emit(TopicAgent, ev)
}

// callbacks 把模型循环的回调映射成事件。
func (r *chatRun) callbacks() ai.ToolStreamCallbacks {
	return ai.ToolStreamCallbacks{
		OnRound:    func(n int) { r.mu.Lock(); r.rounds = n; r.mu.Unlock() },
		OnThinking: func(d string) { r.emit(agent.EventStepThinking, d) },
		OnDelta:    func(d string) { r.emit(agent.EventAnswerDelta, d) },
		OnToolCall: func(call ai.FunctionToolCall) {
			args, _ := parseToolArguments(call.Arguments)
			r.emit(agent.EventToolRequested, agent.ToolCall{CallID: call.ID, ToolName: call.Name, Arguments: args})
		},
		OnToolResult: func(id, name, _ string, ms int64) {
			logger.Info("[AIService] 工具完成: tool=%s callID=%s duration=%dms", name, id, ms)
		},
		OnUsage: func(u ai.Usage) {
			r.mu.Lock()
			r.usage.InputTokens += u.InputTokens
			r.usage.OutputTokens += u.OutputTokens
			r.usage.TotalTokens += u.TotalTokens
			r.mu.Unlock()
		},
	}
}

// executor 构造工具执行回调：解析参数 → 注册表校验并执行 → 发 tool.result → 返回给模型的文本。
func (r *chatRun) executor(ctx context.Context, reg *agent.ToolRegistry) ai.ToolExecutor {
	return func(call ai.FunctionToolCall) ai.ToolExecutionResult {
		var res *agent.ToolResult
		if input, err := parseToolArguments(call.Arguments); err != nil {
			res = agent.Fail("invalid_arguments", err.Error(), "参数必须是一个合法的 JSON 对象")
		} else {
			res = reg.Invoke(ctx, call.Name, input)
		}
		r.emit(agent.EventToolResult, toolResultEvent(call, res))
		out := ai.ToolExecutionResult{Output: res.FormatForModel(agent.ModelRowsDefault, agent.ModelCellDefault)}
		if !res.OK {
			out.Err = errors.New(res.Text)
		}
		return out
	}
}

// toolResultEvent 把工具结果转成事件载荷，行数封顶。
func toolResultEvent(call ai.FunctionToolCall, res *agent.ToolResult) agent.ToolResultEvent {
	ev := agent.ToolResultEvent{
		CallID: call.ID, ToolName: call.Name, OK: res.OK, Kind: string(res.Kind), Summary: res.Summary,
		Columns: res.Columns, Rows: res.Rows, Text: res.Text, Notes: res.Notes, Hint: res.Hint, SQL: res.SQL,
		Truncated: res.Truncated, DurationMs: res.DurationMs, DurationUs: res.DurationUs, ErrorCode: res.ErrorCode,
	}
	if len(ev.Rows) > maxEventRows {
		ev.Rows, ev.Truncated = ev.Rows[:maxEventRows], true
	}
	return ev
}

// parseToolArguments 解析模型给的工具参数 JSON 并规整键名；空串视为没有参数。
func parseToolArguments(arguments string) (agent.ToolInput, error) {
	if arguments == "" {
		return agent.ToolInput{}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(arguments), &raw); err != nil {
		return nil, errors.New("工具参数不是合法的 JSON 对象: " + err.Error())
	}
	return agent.NormalizeArguments(raw), nil
}

// snapshot 返回当前 Transcript 的压缩副本与用量，用于落盘。
func (r *chatRun) snapshot() *agent.Transcript {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tr.Compact(persistRows, persistCell)
}

// usageSnapshot 返回累计用量与轮次。
func (r *chatRun) usageSnapshot() (agent.Usage, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return agent.Usage{
		InputTokens: r.usage.InputTokens, OutputTokens: r.usage.OutputTokens, TotalTokens: r.usage.TotalTokens,
		DurationMs: time.Since(r.startedAt).Milliseconds(),
	}, r.rounds
}
