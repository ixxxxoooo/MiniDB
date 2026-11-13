package app

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"minidb/internal/agent"
	"minidb/internal/ai"
)

func TestChatRunStreamsOrderedPartsAndPersistsTheTranscript(t *testing.T) {
	h := newAIHarness(t)
	srv, bodies := gateway(t, func(n int, body string, s *sse) {
		if n == 1 {
			s.text("我先看看状态的真实取值。")
			s.call("call_1", "column_values", `{"table_name":"users","column":"status","limit":null}`)
			s.done("resp_1")
			return
		}
		s.text("状态以 **paid** 居多。\n\n```minidb-next-steps\n{\"choices\":[{\"label\":\"按月看\",\"prompt\":\"按月统计已支付用户\"}]}\n```")
		s.done("resp_2")
	})
	h.useGateway(t, srv.URL)
	events := h.record()

	out, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "用户状态怎么分布？"}}, "run-1", "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if out["content"] != "状态以 **paid** 居多。" {
		t.Fatalf("the returned answer has no suggestion block: %q", out["content"])
	}

	evs := events()
	var types []string
	for i, ev := range evs {
		if ev.RunID != "run-1" || ev.Seq != int64(i+1) || ev.Version != agent.ProtocolVersion {
			t.Fatalf("event %d: %+v", i, ev)
		}
		types = append(types, string(ev.Type))
	}
	if got := strings.Join(types, " "); !strings.HasPrefix(got, "run.started answer.delta tool.requested tool.result answer.delta") || !strings.HasSuffix(got, "run.done") {
		t.Fatalf("events arrive in the order things happened: %s", got)
	}
	call := evs[2].Payload.(agent.ToolCall)
	if call.CallID != "fc_call_1" || call.ToolName != "column_values" || call.Arguments["table_name"] != "users" {
		t.Fatalf("requested: %+v", call)
	}
	if _, present := call.Arguments["limit"]; present {
		t.Fatal("a null optional argument counts as not given")
	}
	res := evs[3].Payload.(agent.ToolResultEvent)
	if !res.OK || res.Kind != "rows" || res.Rows[0]["value"] != "paid" || res.Summary == "" || res.SQL == "" {
		t.Fatalf("result: %+v", res)
	}

	tr := fold(evs)
	var kinds []string
	for _, p := range tr.Parts {
		kinds = append(kinds, string(p.Type))
	}
	if strings.Join(kinds, ",") != "text,tool,text" || tr.Answer() != "状态以 **paid** 居多。" || !tr.Done {
		t.Fatalf("parts=%v answer=%q", kinds, tr.Answer())
	}
	if len(tr.Suggestions) != 1 || tr.Suggestions[0].Prompt != "按月统计已支付用户" {
		t.Fatalf("suggestions: %+v", tr.Suggestions)
	}
	if tr.Usage.TotalTokens != 240 || tr.Rounds != 2 || tr.Usage.DurationMs < 0 {
		t.Fatalf("usage across both rounds: %+v rounds=%d", tr.Usage, tr.Rounds)
	}
	if len(*bodies) != 2 || !strings.Contains((*bodies)[1], "function_call_output") || !strings.Contains((*bodies)[1], "paid") {
		t.Fatalf("the model must receive the tool output: %v", (*bodies)[1:])
	}
	if !strings.Contains((*bodies)[0], "库结构总览") || !strings.Contains((*bodies)[0], "users") {
		t.Fatal("the first request carries the schema overview")
	}

	// 会话：只在本连接下可见；助手消息带完整过程，可原样还原
	if mine, _ := h.a.AI.ListAISessions("c1", 10); len(mine) != 1 || mine[0].Title != "用户状态怎么分布？" || mine[0].MessageCount != 2 {
		t.Fatalf("sessions: %+v", mine)
	}
	if other, _ := h.a.AI.ListAISessions("another-connection", 10); len(other) != 0 {
		t.Fatalf("sessions of another connection must not show up: %+v", other)
	}
	msgs, err := h.a.AI.GetAISessionMessages("sess-1", 0)
	if err != nil || len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Transcript == nil {
		t.Fatalf("messages: %v %+v", err, msgs)
	}
	restored := msgs[1].Transcript
	if msgs[1].Content != "状态以 **paid** 居多。" || len(restored.Parts) != 3 || restored.Parts[1].Tool.Result.Rows[0]["value"] != "paid" || !restored.Done {
		t.Fatalf("a restored answer looks exactly like the live one: %+v", restored)
	}

	// 重新生成：先丢掉上一轮问答，再问一遍，保存的会话里不会重复
	if err := h.a.AI.DropAISessionTail("sess-1", 2); err != nil {
		t.Fatal(err)
	}
	if left, _ := h.a.AI.GetAISessionMessages("sess-1", 0); len(left) != 0 {
		t.Fatalf("tail dropped: %d", len(left))
	}
}

func TestChatRunFailureModes(t *testing.T) {
	t.Run("upstream error is kept in the saved session", func(t *testing.T) {
		h := newAIHarness(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"message":"bad key"}}`)
		}))
		defer srv.Close()
		h.useGateway(t, srv.URL)
		events := h.record()
		_, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "hi"}}, "r", "s")
		if err == nil {
			t.Fatal("expected an error")
		}
		tr := fold(events())
		if !tr.Done || tr.Error == "" || tr.ErrorCode != agent.ErrCodeFailed || tr.Stopped {
			t.Fatalf("%+v", tr)
		}
		msgs, _ := h.a.AI.GetAISessionMessages("s", 0)
		if len(msgs) != 2 || msgs[1].Transcript == nil || msgs[1].Transcript.Error == "" {
			t.Fatalf("the failed answer is saved too, so a restored chat shows what went wrong: %+v", msgs)
		}
	})

	t.Run("rate limit is explained", func(t *testing.T) {
		h := newAIHarness(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"message":"slow down"}}`)
		}))
		defer srv.Close()
		h.useGateway(t, srv.URL)
		events := h.record()
		_, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "hi"}}, "r", "")
		if err == nil || !strings.Contains(err.Error(), "繁忙") {
			t.Fatalf("%v", err)
		}
		if tr := fold(events()); tr.ErrorCode != agent.ErrCodeRateLimited {
			t.Fatalf("%+v", tr)
		}
	})

	t.Run("a model that never answers is an error, not an empty message", func(t *testing.T) {
		h := newAIHarness(t)
		srv, _ := gateway(t, func(n int, body string, s *sse) { s.done(fmt.Sprint("resp", n)) })
		h.useGateway(t, srv.URL)
		events := h.record()
		_, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "hi"}}, "r", "")
		if err == nil || fold(events()).ErrorCode != agent.ErrCodeEmptyAnswer {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("stop keeps the partial answer and is not an error", func(t *testing.T) {
		h := newAIHarness(t)
		release := make(chan struct{})
		srv, _ := gateway(t, func(n int, body string, s *sse) {
			s.text("写到一半")
			<-release
		})
		defer close(release)
		h.useGateway(t, srv.URL)
		events := h.record()
		go func() {
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				for _, ev := range events() {
					if ev.Type == agent.EventAnswerDelta {
						h.a.AI.CancelChatStream("r")
						return
					}
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
		_, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "hi"}}, "r", "stop-sess")
		if err == nil {
			t.Fatal("a stopped run reports an error to its caller")
		}
		tr := fold(events())
		if !tr.Stopped || tr.Error != "" || tr.Parts[0].Text != "写到一半" {
			t.Fatalf("%+v", tr)
		}
		msgs, _ := h.a.AI.GetAISessionMessages("stop-sess", 0)
		if len(msgs) != 2 || !msgs[1].Transcript.Stopped || msgs[1].Content != "写到一半" {
			t.Fatalf("the partial answer is saved: %+v", msgs)
		}
	})

	t.Run("garbage tool arguments are reported to the model and the run goes on", func(t *testing.T) {
		h := newAIHarness(t)
		srv, bodies := gateway(t, func(n int, body string, s *sse) {
			if n == 1 {
				s.call("c1", "table_list", `{not json`)
				s.done("r1")
				return
			}
			s.text("好的")
			s.done("r2")
		})
		h.useGateway(t, srv.URL)
		events := h.record()
		if _, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "list"}}, "r", ""); err != nil {
			t.Fatal(err)
		}
		var res agent.ToolResultEvent
		for _, ev := range events() {
			if r, ok := ev.Payload.(agent.ToolResultEvent); ok {
				res = r
			}
		}
		if res.OK || res.ErrorCode != "invalid_arguments" || !strings.Contains((*bodies)[1], "ERROR") {
			t.Fatalf("%+v", res)
		}
	})

	t.Run("an empty question is refused before anything happens", func(t *testing.T) {
		h := newAIHarness(t)
		if _, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "  "}}, "r", "s"); err == nil {
			t.Fatal("expected an error")
		}
		if sessions, _ := h.a.AI.ListAISessions("", 10); len(sessions) != 0 {
			t.Fatal("nothing is saved for a refused question")
		}
	})
}
