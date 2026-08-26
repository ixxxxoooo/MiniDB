package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type recordingSink struct {
	events []Event
}

func (r *recordingSink) Emit(ev Event) { r.events = append(r.events, ev) }

func (r *recordingSink) last() *Event {
	if len(r.events) == 0 {
		return nil
	}
	return &r.events[len(r.events)-1]
}

func (r *recordingSink) ofType(t EventType) []Event {
	var out []Event
	for _, ev := range r.events {
		if ev.Type == t {
			out = append(out, ev)
		}
	}
	return out
}

var sampleBudgets = RunBudget{MaxRounds: 5, MaxDuration: time.Minute}

func TestExecutorSingleAnswerNoTools(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	exec := NewExecutor("run-1", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		return StepOutcome{AnswerDeltas: []string{"1", "2", "3"}, Finish: true, Usage: Usage{TotalTokens: 10}}, nil
	}, reg, sink, sampleBudgets)

	res, err := exec.Run(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if res.Content != "123" {
		t.Fatalf("expected answer 123, got %q", res.Content)
	}
	if res.Rounds != 1 {
		t.Fatalf("expected 1 round, got %d", res.Rounds)
	}
	if len(sink.ofType(EventRunDone)) != 1 {
		t.Fatalf("expected one run.done")
	}
	if len(sink.ofType(EventToolRequested)) != 0 {
		t.Fatalf("no tools should be requested")
	}
}

func TestExecutorToolLoopAndResult(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "double",
		Parameters: map[string]any{"type": "object"},
		ReadOnly:   true,
		Handler: func(ctx context.Context, in ToolInput) *ToolResult {
			v := in.Int("n")
			return TextResult(string(rune('0' + v*2)))
		},
	})

	var calls int64
	exec := NewExecutor("run-2", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			return StepOutcome{Calls: []StepCall{{ID: "c1", Name: "double", Args: map[string]any{"n": 2}}}}, nil
		}
		return StepOutcome{AnswerDeltas: []string{"ok"}, Finish: true}, nil
	}, reg, sink, sampleBudgets)

	res, err := exec.Run(context.Background(), "sys", []Message{{Role: "user", Content: "x"}})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("expected ok, got %q", res.Content)
	}
	if len(sink.ofType(EventToolRequested)) != 1 {
		t.Fatalf("expected one tool.requested")
	}
	results := sink.ofType(EventToolResult)
	if len(results) != 1 {
		t.Fatalf("expected one tool.result, got %d", len(results))
	}
	payload, ok := results[0].Payload.(ToolResultEvent)
	if !ok || !payload.OK {
		t.Fatalf("tool.result payload malformed: %+v", results[0].Payload)
	}
}

func TestExecutorGuardrailDeniesTool(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "danger", Parameters: map[string]any{"type": "object"}, ReadOnly: false,
		Handler: func(ctx context.Context, in ToolInput) *ToolResult { return TextResult("bad") },
	})

	var calls int64
	exec := NewExecutor("run-3", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			return StepOutcome{Calls: []StepCall{{ID: "c1", Name: "danger", Args: map[string]any{}}}}, nil
		}
		return StepOutcome{AnswerDeltas: []string{"ok"}, Finish: true}, nil
	}, reg, sink, sampleBudgets)
	exec.GuardrailFuncs = []GuardFn{
		func(ctx context.Context, rc *RunContext, call StepCall) GuardResult {
			if call.Name == "danger" {
				return GuardResult{Denied: true, Code: "guarded", Reason: "blocked"}
			}
			return GuardResult{}
		},
	}

	if _, err := exec.Run(context.Background(), "sys", nil); err != nil {
		t.Fatalf("run error: %v", err)
	}
	results := sink.ofType(EventToolResult)
	if len(results) != 1 {
		t.Fatalf("expected one denied tool.result")
	}
	payload, _ := results[0].Payload.(ToolResultEvent)
	if payload.OK || payload.ErrorCode != "guarded" {
		t.Fatalf("denied result should be not-ok guarded: %+v", payload)
	}
}

func TestExecutorMaxRoundsTermsLoop(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "loop", Parameters: map[string]any{"type": "object"}, ReadOnly: true,
		Handler: func(ctx context.Context, in ToolInput) *ToolResult { return TextResult("x") },
	})

	exec := NewExecutor("run-4", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		// 每轮都要求再调工具，直至轮数被预算切断
		return StepOutcome{Calls: []StepCall{{ID: "c", Name: "loop", Args: map[string]any{}}}}, nil
	}, reg, sink, RunBudget{MaxRounds: 2, MaxDuration: time.Minute})

	res, err := exec.Run(context.Background(), "sys", nil)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if res.Rounds != 2 {
		t.Fatalf("expected loop cut at round 2, got %d", res.Rounds)
	}
}

func TestExecutorRunErrorPropagates(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	exec := NewExecutor("run-5", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		return StepOutcome{}, errors.New("boom")
	}, reg, sink, sampleBudgets)

	_, err := exec.Run(context.Background(), "sys", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(sink.ofType(EventRunError)) != 1 {
		t.Fatalf("expected one run.error")
	}
}

func TestExecutorTokenBudgetStops(t *testing.T) {
	sink := &recordingSink{}
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "loop", Parameters: map[string]any{"type": "object"}, ReadOnly: true,
		Handler: func(ctx context.Context, in ToolInput) *ToolResult { return TextResult("x") },
	})

	exec := NewExecutor("run-6", func(ctx context.Context, sys string, msgs []Message, tools []Tool, sink EventSink) (StepOutcome, error) {
		return StepOutcome{Calls: []StepCall{{ID: "c", Name: "loop", Args: map[string]any{}}}, Usage: Usage{TotalTokens: 60}}, nil
	}, reg, sink, RunBudget{MaxRounds: 10, MaxTokens: 100, MaxDuration: time.Minute})

	res, _ := exec.Run(context.Background(), "sys", nil)
	if res.Usage.TotalTokens < 100 {
		t.Fatalf("expected tokens used >= budget threshold: %d", res.Usage.TotalTokens)
	}
	if len(sink.ofType(EventRunDone)) != 1 {
		t.Fatalf("expected run.done after budget stop")
	}
}