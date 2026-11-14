package ai

import (
	"context"
	"strings"

	"minidb/internal/logger"
)

// turn 模型一轮请求的产出：说明文字、推理文字，以及要执行的函数调用。
type turn struct {
	text      string
	reasoning string
	calls     []FunctionToolCall
}

// toolLoop 两条通道（Responses / Chat Completions）各自实现的传输层：
// 只负责"怎么把历史发出去、怎么把结果接回来"，"什么时候停、什么时候总结"全在 runToolLoop 里。
type toolLoop interface {
	// next 发起第 round 轮带工具的请求。
	next(ctx context.Context, round int) (turn, error)
	// record 把模型这一轮的输出和各工具的执行结果追加进下一轮的输入。
	record(t turn, outs []toolOutput)
	// finalize 不带工具地再请求一次，让模型基于已有工具结果给出最终回答。
	finalize(ctx context.Context) (string, error)
}

// runToolLoop 驱动"模型 → 工具 → 模型"循环直到得到最终回答。
// 停止规则只有这一份：没有函数调用就结束；工具跑完后模型没写回答，或到达轮次上限，改用无工具总结。
func runToolLoop(ctx context.Context, l toolLoop, maxRounds int, executor ToolExecutor, cb ToolStreamCallbacks) (string, error) {
	for round := 1; round <= maxRounds; round++ {
		cb.round(round)
		t, err := l.next(ctx, round)
		if err != nil {
			return "", err
		}
		if len(t.calls) == 0 {
			// 工具跑完后模型有时只"想"了一下没写回答：用不带工具的总结请求再要一次
			if strings.TrimSpace(t.text) == "" && round > 1 {
				return l.finalize(ctx)
			}
			return strings.TrimSpace(t.text), nil
		}
		l.record(t, runToolCalls(executor, t.calls, cb))
		if round == maxRounds {
			logger.Warn("[AI] 已达工具轮次上限(%d)，触发无工具总结", maxRounds)
			return l.finalize(ctx)
		}
	}
	return l.finalize(ctx)
}
