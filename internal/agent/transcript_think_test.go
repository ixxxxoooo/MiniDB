package agent

import (
	"testing"
	"time"
)

// 推理片段在下一个片段开始时记下耗时，并随会话保存。
func TestTranscriptThinkingDuration(t *testing.T) {
	var tr Transcript
	tr.Apply(Event{Type: EventStepThinking, Payload: "先看"})
	tr.Apply(Event{Type: EventStepThinking, Payload: "结构"})
	time.Sleep(5 * time.Millisecond)
	if tr.Parts[0].DurationMs != 0 {
		t.Fatal("还在想的时候没有耗时")
	}
	tr.Apply(Event{Type: EventToolRequested, Payload: ToolCall{CallID: "c", ToolName: "table_list"}})
	if len(tr.Parts) != 2 || tr.Parts[0].Text != "先看结构" || tr.Parts[0].DurationMs < 5 {
		t.Fatalf("parts=%+v", tr.Parts)
	}
	// 最后一段思考在 run 结束时收尾
	tr.Apply(Event{Type: EventStepThinking, Payload: "再想想"})
	tr.Apply(Event{Type: EventRunDone, Payload: RunDone{Content: "好"}})
	if tr.Parts[2].DurationMs < 1 || tr.Compact(10, 10).Parts[2].DurationMs != tr.Parts[2].DurationMs {
		t.Fatalf("收尾和持久化: %+v", tr.Parts[2])
	}
}
