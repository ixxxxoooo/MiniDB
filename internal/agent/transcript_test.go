package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func apply(t *Transcript, typ EventType, payload any) { t.Apply(Event{Type: typ, Payload: payload}) }

func TestTranscriptKeepsTextAndToolsInOrder(t *testing.T) {
	var tr Transcript
	apply(&tr, EventStepThinking, "先看")
	apply(&tr, EventStepThinking, "表结构")
	apply(&tr, EventAnswerDelta, "我先查一下")
	apply(&tr, EventToolRequested, ToolCall{CallID: "c1", ToolName: "table_describe", Arguments: map[string]any{"table_names": []any{"users"}}})
	if !tr.Parts[len(tr.Parts)-1].Tool.Running() {
		t.Fatal("a requested tool is running until its result arrives")
	}
	apply(&tr, EventToolResult, ToolResultEvent{CallID: "c1", ToolName: "table_describe", OK: true, Kind: "text", Text: "users(id)"})
	apply(&tr, EventAnswerDelta, "共 ")
	apply(&tr, EventAnswerDelta, "450 行")
	if got := tr.Answer(); got != "共 450 行" {
		t.Fatalf("the answer is only the text after the last tool, not the narration before it: %q", got)
	}
	var kinds []string
	for _, p := range tr.Parts {
		kinds = append(kinds, string(p.Type))
	}
	if strings.Join(kinds, ",") != "thinking,text,tool,text" {
		t.Fatalf("parts = %v", kinds)
	}
	if tr.Parts[0].Text != "先看表结构" {
		t.Fatalf("consecutive thinking deltas merge: %q", tr.Parts[0].Text)
	}

	apply(&tr, EventRunDone, RunDone{Content: "共 **450** 行", Rounds: 2, Usage: Usage{TotalTokens: 99}, Suggestions: []Suggestion{{Label: "a", Prompt: "b"}}})
	if !tr.Done || tr.Answer() != "共 **450** 行" || tr.Usage.TotalTokens != 99 || len(tr.Suggestions) != 1 || tr.Rounds != 2 {
		t.Fatalf("done: %+v", tr)
	}
}

func TestTranscriptTerminalStates(t *testing.T) {
	var stopped Transcript
	apply(&stopped, EventAnswerDelta, "写到一半")
	apply(&stopped, EventRunError, RunError{Code: ErrCodeCancelled, Message: "context canceled"})
	if !stopped.Done || !stopped.Stopped || stopped.Error != "" {
		t.Fatalf("a user stop is not an error: %+v", stopped)
	}

	var failed Transcript
	apply(&failed, EventRunError, RunError{Code: ErrCodeRateLimited, Message: "繁忙"})
	if failed.Stopped || failed.Error != "繁忙" || failed.ErrorCode != ErrCodeRateLimited {
		t.Fatalf("%+v", failed)
	}

	var orphan Transcript
	apply(&orphan, EventToolResult, ToolResultEvent{CallID: "x", ToolName: "table_list", OK: true})
	if len(orphan.Parts) != 1 || orphan.Parts[0].Tool.Result == nil {
		t.Fatal("a result without its call is still kept")
	}

	var stripped Transcript
	apply(&stripped, EventAnswerDelta, "好的\n```minidb-next-steps\n{\"choices\":[]}\n```")
	apply(&stripped, EventRunDone, RunDone{})
	if stripped.Answer() != "好的" {
		t.Fatalf("streamed suggestion block removed on done: %q", stripped.Answer())
	}
}

func TestTranscriptCompactBoundsPersistedSize(t *testing.T) {
	rows := make([]map[string]any, 500)
	for i := range rows {
		rows[i] = map[string]any{"id": i, "blob": strings.Repeat("长", 1000)}
	}
	var tr Transcript
	apply(&tr, EventToolRequested, ToolCall{CallID: "c", ToolName: "sql_readonly_execute"})
	apply(&tr, EventToolResult, ToolResultEvent{CallID: "c", OK: true, Kind: "rows", Columns: []string{"id", "blob"}, Rows: rows})
	small := tr.Compact(100, 50)
	res := small.Parts[0].Tool.Result
	if len(res.Rows) != 100 || !res.Truncated || len([]rune(res.Rows[0]["blob"].(string))) != 51 || res.Rows[7]["id"] != 7 {
		t.Fatalf("rows=%d truncated=%v", len(res.Rows), res.Truncated)
	}
	if len(tr.Parts[0].Tool.Result.Rows) != 500 {
		t.Fatal("compacting must not modify the live transcript")
	}
	data, _ := json.Marshal(small)
	var back Transcript
	if err := json.Unmarshal(data, &back); err != nil || len(back.Parts) != 1 || back.Parts[0].Tool.Name != "sql_readonly_execute" {
		t.Fatalf("a transcript round-trips through JSON: %v %+v", err, back)
	}
}
