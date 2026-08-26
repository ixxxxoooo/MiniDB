package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Message 会话消息（执行器与 LLM 交互的最小模型）。
type Message struct {
	Role    string `json:"role"` // user | assistant | system | tool
	Content string `json:"content"`
}

// StepCall 一轮中模型请求的一次工具调用。
type StepCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// StepOutcome 一次 LLM 步骤的结果（thinking 增量 / 工具调用 / 回答增量）。
type StepOutcome struct {
	ThinkingDeltas []string
	Calls          []StepCall
	AnswerDeltas   []string
	Finish         bool // 无工具调用，已进入最终回答（可结束 run）
	Usage          Usage
}

// LLMStep LLM 单步函数（由 services 注入；测试用 fake 驱动）。
// sink 用于转发 thinking/answer 增量事件。
type LLMStep func(ctx context.Context, sysPrompt string, messages []Message, tools []Tool, sink EventSink) (StepOutcome, error)

// RunBudget 单次 run 的预算（护栏之一）。
type RunBudget struct {
	MaxRounds     int
	MaxTokens     int64
	MaxDuration   time.Duration
	StatementTimeout time.Duration // DB 语句超时（透传给工具）
}

// GuardResult 护栏检查结果。
type GuardResult struct {
	Denied bool
	Code   string // guarded | max_rounds | max_tokens | timeout
	Reason string
}

// GuardFn 工具调用前/后的护栏函数。
type GuardFn func(ctx context.Context, runCtx *RunContext, call StepCall) GuardResult

// RunContext 提供 run 级上下文给护栏与工具。
type RunContext struct {
	RunID    string
	Budget   RunBudget
	ToolArgs map[string]any // 解析后的参数（已规范化）
}

// Executor Agent 执行器状态机（纯逻辑，可单测）。
type Executor struct {
	RunID string
	Step  LLMStep
	Tools *ToolRegistry
	Sink  EventSink

	// 可注入护栏：按顺序执行，任一拒绝即终止工具执行
	GuardrailFuncs []GuardFn

	Budget RunBudget

	// 内部状态（一次 Execute 有效）
	messages    []Message
	sysPrompt   string
	toolResults map[string]string // callID -> 工具结果文本（回灌模型）
	answerBuf   strings.Builder
	tokensUsed  int64
	rounds      int
	startedAt   time.Time
	seq         int64
}

// RunResult 一次 run 的最终结果。
type RunResult struct {
	Content string
	Rounds  int
	Usage   Usage
	Cancelled bool
	SkippedCodes []string // 被护栏拒绝的工具 errorCode 集合
}

// NewExecutor 构造执行器。
func NewExecutor(runID string, step LLMStep, tools *ToolRegistry, sink EventSink, budget RunBudget) *Executor {
	if budget.MaxRounds <= 0 {
		budget.MaxRounds = 6
	}
	if budget.MaxDuration <= 0 {
		budget.MaxDuration = 10 * time.Minute
	}
	return &Executor{
		RunID:       runID,
		Step:        step,
		Tools:       tools,
		Sink:        sink,
		Budget:      budget,
		toolResults: make(map[string]string),
	}
}

// Run 执行 run 状态机：budget → step → tools → repeat → answer。
// model 名称用于 run.started。
func (e *Executor) Run(ctx context.Context, sysPrompt string, messages []Message) (*RunResult, error) {
	e.sysPrompt = sysPrompt
	e.messages = append([]Message(nil), messages...)
	e.startedAt = time.Now()

	var overallTimeout time.Duration = e.Budget.MaxDuration
	if overallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, overallTimeout)
		defer cancel()
	}

	for round := 1; round <= e.Budget.MaxRounds; round++ {
		e.rounds = round
		if err := ctx.Err(); err != nil {
			return e.finish(err)
		}

		outcome, err := e.Step(ctx, e.sysPrompt, e.messages, e.Tools.Visible(), e.Sink)
		if err != nil {
			return e.finish(err)
		}
		e.tokensUsed += outcome.Usage.TotalTokens

		// 预算检查：token 超限 → 直接结束（finish 内会发 run.done）
		if e.Budget.MaxTokens > 0 && e.tokensUsed > e.Budget.MaxTokens {
			return e.finish(nil)
		}

		if len(outcome.Calls) == 0 {
			// 无工具调用：最终回答
			for _, d := range outcome.AnswerDeltas {
				e.answerBuf.WriteString(d)
			}
			return e.finish(nil)
		}

		// 有工具调用：逐个执行（护栏检查 → 工具调用 → 观察结果回灌）
		for _, call := range outcome.Calls {
			if err := ctx.Err(); err != nil {
				return e.finish(err)
			}
			if reason := e.checkGuards(ctx, call); reason != "" {
				// checkGuards 内已向 sink 派发被拒的 tool.result 事件
				e.toolResults[call.ID] = "ERROR: 工具调用被护栏拒绝(" + reason + ")"
				continue
			}
			if !e.toolIsReadOnly(call.Name) {
				e.emitApprovalRequested(call)
				// 预留：等 approval 事件；当前仅记录并继续（保持只读约束）
			}
			e.emitToolRequested(call)
			res := e.Tools.Invoke(ctx, call.Name, call.Args)
			text := "..."
			if res != nil {
				text = res.FormatForModel(20, 120)
			}
			e.toolResults[call.ID] = text
			e.emitToolResult(call, res)
		}

		// 组装下一轮消息：assistant tool_calls + tool 结果
		e.appendToolRoundMessages(outcome.Calls)
	}

	// 达轮次上限：触发无工具总结
	return e.finish(nil)
}

func (e *Executor) checkGuards(ctx context.Context, call StepCall) string {
	rc := &RunContext{RunID: e.RunID, Budget: e.Budget, ToolArgs: call.Args}
	for _, guard := range e.GuardrailFuncs {
		r := guard(ctx, rc, call)
		if r.Denied {
			e.emitToolResult(StepCall{ID: call.ID, Name: call.Name}, &ToolResult{
				OK: false, Kind: ResultKindErr, ErrorCode: r.Code,
				Text: "工具调用被护栏拒绝: " + r.Reason,
			})
			return r.Code
		}
	}
	return ""
}

func (e *Executor) toolIsReadOnly(name string) bool {
	t, ok := e.Tools.GetByName(name)
	return ok && t.ReadOnly
}

func (e *Executor) appendToolRoundMessages(calls []StepCall) {
	if len(calls) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("工具调用请求：")
	for i, call := range calls {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(fmt.Sprintf("%s(%s)", call.Name, jsonArgs(call.Args)))
	}
	e.messages = append(e.messages, Message{Role: "assistant", Content: sb.String()})
	for _, call := range calls {
		resultText := e.toolResults[call.ID]
		if resultText == "" {
			resultText = "（空结果）"
		}
		e.messages = append(e.messages, Message{Role: "tool", Content: resultText})
	}
}

func (e *Executor) emitToolRequested(call StepCall) {
	if e.Sink == nil {
		return
	}
	e.Sink.Emit(Event{
		Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventToolRequested,
		Phase: "tool", Payload: ToolCall{CallID: call.ID, ToolName: call.Name, Arguments: call.Args},
	})
}

func (e *Executor) emitToolResult(call StepCall, res *ToolResult) {
	if e.Sink == nil {
		return
	}
	ev := ToolResultEvent{
		CallID: call.ID, ToolName: call.Name,
		DurationMs: 0, ErrorCode: "", Data: nil,
	}
	if res != nil {
		ev.OK = res.OK
		ev.Kind = string(res.Kind)
		ev.Columns = res.Columns
		ev.Rows = res.Rows
		ev.Text = res.Text
		ev.Truncated = res.Truncated
		ev.DurationMs = res.DurationMs
		ev.ErrorCode = res.ErrorCode
		if res.Data != nil {
			ev.Data = res.Data
		}
	}
	e.Sink.Emit(Event{
		Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventToolResult,
		Phase: "tool", Payload: ev,
	})
}

func (e *Executor) emitToolDenied(call StepCall, reason string) {
	e.emitToolResult(call, &ToolResult{OK: false, Kind: ResultKindErr, ErrorCode: "guarded", Text: "工具调用被护栏拒绝: " + reason})
}

func (e *Executor) emitApprovalRequested(call StepCall) {
	if e.Sink == nil {
		return
	}
	e.Sink.Emit(Event{
		Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventApproveRequest,
		Phase: "tool", Payload: ApprovalRequest{Action: call.Name, Meta: call.Args},
	})
}

func (e *Executor) emitDoneWithNotice(notice string) {
	if e.Sink == nil {
		return
	}
	payload := RunDone{Content: e.answerBuf.String(), Rounds: e.rounds, Usage: e.usage()}
	_ = notice
	e.Sink.Emit(Event{
		Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventRunDone,
		Phase: "answer", Payload: payload,
	})
}

// finish 统一收尾：回填最终答案、发 run.done/run.error、返回结果。
func (e *Executor) finish(runErr error) (*RunResult, error) {
	result := &RunResult{
		Content: e.answerBuf.String(),
		Rounds:  e.rounds,
		Usage:   e.usage(),
	}
	if runErr != nil {
		if ctxCancelErr(runErr) {
			result.Cancelled = true
		}
		if e.Sink != nil {
			e.Sink.Emit(Event{
				Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventRunError,
				Phase: "answer", Payload: RunError{Code: errorCode(runErr), Message: runErr.Error()},
			})
		}
		return result, runErr
	}
	if e.Sink != nil {
		e.Sink.Emit(Event{
			Version: ProtocolVersion, RunID: e.RunID, Seq: e.nextSeq(), Type: EventRunDone,
			Phase: "answer", Payload: RunDone{Content: result.Content, Rounds: result.Rounds, Usage: result.Usage},
		})
	}
	return result, nil
}

func (e *Executor) usage() Usage {
	return Usage{
		TotalTokens: e.tokensUsed,
		DurationMs:  time.Since(e.startedAt).Milliseconds(),
	}
}

func (e *Executor) nextSeq() int64 {
	e.seq++
	return e.seq
}

func ctxCancelErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "deadline exceeded")
}

func errorCode(err error) string {
	switch {
	case ctxCancelErr(err):
		return "cancelled"
	case err != nil:
		return "failed"
	default:
		return ""
	}
}

func jsonArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}