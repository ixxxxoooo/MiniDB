package ai

import (
	"context"
	"errors"
	"testing"
)

// fakeLoop 按预设脚本逐轮返回，记录 finalize 与 record 的调用。
type fakeLoop struct {
	turns     []turn
	recorded  int
	finalized int
}

func (f *fakeLoop) next(_ context.Context, round int) (turn, error) {
	if round > len(f.turns) {
		return turn{}, errors.New("脚本用完")
	}
	return f.turns[round-1], nil
}
func (f *fakeLoop) record(turn, []toolOutput) { f.recorded++ }
func (f *fakeLoop) finalize(context.Context) (string, error) {
	f.finalized++
	return "总结", nil
}

func TestRunToolLoopStopRules(t *testing.T) {
	call := FunctionToolCall{ID: "1", CallID: "1", Name: "t", Arguments: "{}"}
	exec := func(FunctionToolCall) ToolExecutionResult { return ToolExecutionResult{Output: "ok"} }
	cases := []struct {
		name          string
		turns         []turn
		max           int
		want          string
		recorded, fin int
	}{
		{"没有调用直接结束", []turn{{text: " 答案 "}}, 3, "答案", 0, 0},
		{"调用后给出回答", []turn{{calls: []FunctionToolCall{call}}, {text: "好"}}, 3, "好", 1, 0},
		{"工具之后只想不答，改用总结", []turn{{calls: []FunctionToolCall{call}}, {text: "  "}}, 3, "总结", 1, 1},
		{"到达轮次上限，改用总结", []turn{{calls: []FunctionToolCall{call}}, {calls: []FunctionToolCall{call}}}, 2, "总结", 2, 1},
		{"第一轮就空回答不总结", []turn{{text: ""}}, 3, "", 0, 0},
	}
	for _, c := range cases {
		f := &fakeLoop{turns: c.turns}
		got, err := runToolLoop(context.Background(), f, c.max, exec, ToolStreamCallbacks{})
		if err != nil || got != c.want || f.recorded != c.recorded || f.finalized != c.fin {
			t.Errorf("%s: got=%q err=%v recorded=%d finalized=%d", c.name, got, err, f.recorded, f.finalized)
		}
	}
}
