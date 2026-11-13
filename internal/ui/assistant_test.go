package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"minidb/internal/app"
)

// fakeLLM speaks just enough of the Chat Completions API: the first request
// of a conversation asks for a tool, the next answers in text.
func fakeLLM(t *testing.T, sql string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var chatCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/responses") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"message":"not found","type":"invalid_request_error"}}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta map[string]any, finish any) {
			chunk := map[string]any{"id": "c", "object": "chat.completion.chunk", "model": "m",
				"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}}
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		chatCalls.Add(1)
		if strings.Contains(string(body), `"role":"tool"`) {
			send(map[string]any{"role": "assistant", "content": "共有 **450** 个用户。\n\n```sql\n" + sql + "\n```"}, nil)
			send(map[string]any{}, "stop")
		} else {
			send(map[string]any{"role": "assistant", "reasoning_content": "先数一下用户表"}, nil)
			args, _ := json.Marshal(map[string]any{"sql": sql, "table_names": []string{"users"}, "limit": 10})
			send(map[string]any{"tool_calls": []map[string]any{{"index": 0, "id": "call_1", "type": "function",
				"function": map[string]any{"name": "sql_readonly_execute", "arguments": string(args)}}}}, nil)
			send(map[string]any{}, "tool_calls")
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &chatCalls
}

func TestAssistantAnswersWithToolAndTable(t *testing.T) {
	e := newEnv(t)
	sql := "SELECT COUNT(*) AS n FROM users"
	srv, calls := fakeLLM(t, sql)
	if err := e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	if !w.ai.ready {
		t.Fatal("assistant should see the saved configuration")
	}

	w.ai.send("有多少用户？")
	if w.ai.running {
		t.Fatal("run should have finished")
	}
	if len(w.ai.msgs) != 2 {
		t.Fatalf("msgs=%d", len(w.ai.msgs))
	}
	reply := w.ai.msgs[1]
	if !reply.done || reply.err != "" {
		t.Fatalf("done=%v err=%q", reply.done, reply.err)
	}
	if !strings.Contains(reply.text, "450") {
		t.Fatalf("answer = %q", reply.text)
	}
	var thinking, tool *step
	for _, st := range reply.steps {
		if st.thinking {
			thinking = st
		} else {
			tool = st
		}
	}
	if thinking == nil || !strings.Contains(thinking.text, "用户表") {
		t.Fatalf("thinking step missing: %+v", reply.steps)
	}
	if tool == nil || tool.tool != "sql_readonly_execute" || tool.result == nil || !tool.result.OK {
		t.Fatalf("tool step: %+v result=%+v", tool, tool.result)
	}
	if tool.grid == nil || len(tool.grid.rows) != 1 || plainText(tool.grid.rows[0]["n"]) != "450" {
		t.Fatalf("structured rows missing: %+v", tool.grid)
	}
	if tool.sql != sql {
		t.Fatalf("sql = %q", tool.sql)
	}
	if calls.Load() != 2 {
		t.Fatalf("chat calls = %d, want 2 (tool, then answer)", calls.Load())
	}
	tool.open, thinking.open = true, true
	e.has("运行只读 SQL")
	e.has("450")
	e.shot("11-assistant")

	// the SQL block of the answer can be sent to a query tab and run
	w.ai.insertSQL(sql, true)
	q, ok := w.tabs[w.active].(*queryTab)
	if !ok || len(q.results) != 1 {
		t.Fatalf("insert+run: tab=%T", w.tabs[w.active])
	}

	// the conversation is stored and can be reopened
	sessions, _ := e.sh.app.AI.ListAISessions(10)
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d", len(sessions))
	}
	w.ai.reset()
	w.ai.loadSession(sessions[0].ID)
	if len(w.ai.msgs) != 2 || !w.ai.msgs[0].user || !strings.Contains(w.ai.msgs[1].text, "450") {
		t.Fatalf("restored: %+v", w.ai.msgs)
	}
}

func TestAssistantReportsErrors(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer srv.Close()
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.ai.send("hi")
	reply := w.ai.msgs[1]
	if !reply.done || reply.err == "" {
		t.Fatalf("an upstream failure must show as an error: done=%v err=%q", reply.done, reply.err)
	}
	if w.ai.running {
		t.Fatal("must not stay running")
	}
}

func TestAssistantNotConfigured(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	e.has("还没有配置 AI")
	e.shot("12-ai-unconfigured")
	if err := e.tt.Click("配置 AI"); err != nil {
		t.Fatal(err)
	}
	e.has("服务地址")
}
