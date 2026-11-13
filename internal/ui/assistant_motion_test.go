package ui

import (
	"testing"
	"time"

	"minidb/internal/agent"
)

func TestDotOpacityLightsOneDotAtATime(t *testing.T) {
	for _, phase := range []float32{0, 0.1, 0.2, 0.34, 0.5, 0.7, 0.9, 0.99} {
		bright := 0
		for i := 0; i < 3; i++ {
			if o := dotOpacity(phase, i); o < 0.25 || o > 1 {
				t.Fatalf("phase %v dot %d: opacity %v out of range", phase, i, o)
			} else if o > 0.5 {
				bright++
			}
		}
		if bright > 1 {
			t.Fatalf("phase %v: %d dots are bright at once", phase, bright)
		}
	}
	if dotOpacity(0, 0) != 1 || dotOpacity(1.0/3, 1) != 1 {
		t.Fatal("每个点在轮到自己的那一刻最亮")
	}
}

func TestOnlyFreshMessagesAnimateIn(t *testing.T) {
	restored := &chatMsg{}
	if restored.fresh() {
		t.Fatal("恢复的旧会话不该放入场动画")
	}
	now := &chatMsg{born: time.Now()}
	if !now.fresh() {
		t.Fatal("刚产生的应放入场动画")
	}
	old := &chatMsg{born: time.Now().Add(-time.Minute)}
	if old.fresh() {
		t.Fatal("过了一会儿的不该再动（滚动回来重画时不要重新弹出）")
	}
}

// 动效只表达状态：回答在进行时持续出帧，结束后一帧不多——不能让一个收尾了的对话还在空转烧 CPU。
func TestMotionRunsOnlyWhileTheAnswerIsLive(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	w.ai.ready, w.ai.checked = true, true
	user := &chatMsg{id: 1, user: true, text: "有多少用户？", born: time.Now()}
	reply := &chatMsg{id: 2, born: time.Now()}
	w.ai.msgs = []*chatMsg{user, reply}
	w.ai.running = true
	e.tt.Frame()
	e.shot("32-flow-waiting") // 还没有任何动静：等待的三个点

	apply := func(typ agent.EventType, payload any) { reply.tr.Apply(agent.Event{Type: typ, Payload: payload}) }
	apply(agent.EventStepThinking, "先数一下用户表")
	apply(agent.EventToolRequested, agent.ToolCall{CallID: "c1", ToolName: "sql_readonly_execute", Arguments: map[string]any{"sql": "SELECT COUNT(*) FROM users"}})
	e.tt.Frame()
	e.shot("33-flow-tool-running")
	e.tt.Advance(400 * time.Millisecond)
	if !e.tt.FramePending() {
		t.Fatal("工具在运行时应持续出帧（图标块在呼吸）")
	}

	apply(agent.EventToolResult, agent.ToolResultEvent{CallID: "c1", ToolName: "sql_readonly_execute", OK: true, Kind: "rows", Summary: "1 行", DurationMs: 3,
		Columns: []string{"n"}, Rows: []map[string]any{{"n": 450}}})
	apply(agent.EventAnswerDelta, "共有 **450** 个用户")
	e.tt.Frame()
	e.shot("34-flow-streaming")
	if !e.tt.FramePending() {
		t.Fatal("回答还在流出来时应持续出帧（末尾的小点在呼吸）")
	}

	apply(agent.EventRunDone, agent.RunDone{Content: "共有 **450** 个用户"})
	w.ai.running = false
	e.tt.Frame()
	e.tt.Advance(time.Second) // 让入场、折叠的过渡走完
	e.tt.Frame()
	e.tt.Advance(time.Second)
	if e.tt.FramePending() {
		t.Fatal("回答结束后不该再有动效在出帧")
	}
	e.shot("35-flow-done")
}
