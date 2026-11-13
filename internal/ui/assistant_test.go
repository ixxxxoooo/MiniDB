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

	mui "github.com/egoist/mygo/ui"

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
	tool.open, thinking.open, reply.logMode = true, true, 1
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

// TestAssistantEmptyStateSuggestionsAndRegenerate 空对话显示提示卡，点一张就发出去；
// 完成后默认折叠活动日志，可以手动展开；"重新生成"用同一个问题再问一次。
func TestAssistantEmptyStateSuggestionsAndRegenerate(t *testing.T) {
	e := newEnv(t)
	srv, calls := fakeLLM(t, "SELECT COUNT(*) AS n FROM users")
	if err := e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	e.tt.Frame()
	e.has("问问你的数据库")
	e.has("了解这个库")
	e.has("看看 orders") // 有表时多一张针对第一张表的卡片
	e.shot("93-ai-empty")
	if err := e.tt.Click("最大的表"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "统计行数最多的 5 张表" {
		t.Fatalf("点提示卡应发出对应问题: %+v", w.ai.msgs)
	}
	reply := w.ai.msgs[1]
	e.has("已完成 2 个步骤")
	if reply.logMode != 0 {
		t.Fatal("完成后活动日志默认折叠")
	}
	e.hasNot("已完成 0 个步骤") // 折叠时日志只剩一行摘要；结果表格另摆在回复里
	e.shot("94-ai-collapsed")
	before := calls.Load()
	w.ai.regenerate()
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "统计行数最多的 5 张表" || calls.Load() <= before {
		t.Fatalf("重新生成应再问一次且不堆叠: msgs=%d calls=%d→%d", len(w.ai.msgs), before, calls.Load())
	}
	if got := visibleAnswer("好的\n```minidb-next-steps\n{}\n```"); got != "好的" {
		t.Fatalf("复制的正文不应带结构化块: %q", got)
	}
	if got := toolTarget(map[string]any{"table_name": "orders", "column": "status"}); got != "orders.status" {
		t.Fatalf("toolTarget=%q", got)
	}
}

// emptyAfterToolLLM 模拟线上遇到的网关行为：工具跑完后的那一轮只返回思考、没有正文；
// 追加"请给出最终回答"的总结请求后才有回答。
func emptyAfterToolLLM(t *testing.T, sql string, neverAnswer bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/responses") {
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"message":"not found"}}`)
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
		switch {
		case strings.Contains(string(body), "禁止继续调用任何工具") && !neverAnswer:
			send(map[string]any{"role": "assistant", "content": "共有 450 个用户。"}, nil)
			send(map[string]any{}, "stop")
		case strings.Contains(string(body), `"role":"tool"`):
			send(map[string]any{"role": "assistant", "reasoning_content": "结果拿到了"}, nil)
			send(map[string]any{}, "stop")
		default:
			args, _ := json.Marshal(map[string]any{"sql": sql})
			send(map[string]any{"tool_calls": []map[string]any{{"index": 0, "id": "call_1", "type": "function",
				"function": map[string]any{"name": "sql_readonly_execute", "arguments": string(args)}}}}, nil)
			send(map[string]any{}, "tool_calls")
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAssistantRecoversFromEmptyAnswerAfterTools(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", false)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.ai.send("有多少用户？")
	reply := w.ai.msgs[1]
	if reply.err != "" || !strings.Contains(reply.text, "450") {
		t.Fatalf("空回答应触发总结请求并拿到答案: err=%q text=%q", reply.err, reply.text)
	}
}

func TestAssistantShowsErrorWhenModelNeverAnswers(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", true)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("有多少用户？")
	reply := w.ai.msgs[1]
	if reply.err == "" {
		t.Fatalf("始终没有回答时必须报错而不是留空: %+v", reply)
	}
	e.tt.Frame()
	e.tt.Frame()
	e.has("450") // 查询结果的表格直接摆在回复里，不用点开日志
	e.shot("95-ai-empty-answer")
}

func TestAssistantInputShortcuts(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", false)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	e.tt.Frame()
	// 输入框自动聚焦；⇧↵ 换行而不发送
	e.tt.Type("第一行")
	e.tt.Key(mui.Shift, mui.KeyEnter)
	e.tt.Type("第二行")
	e.tt.Frame()
	if len(w.ai.msgs) != 0 || w.ai.input != "第一行\n第二行" {
		t.Fatalf("⇧↵ 应换行: msgs=%d input=%q", len(w.ai.msgs), w.ai.input)
	}
	// ↵ 发送并清空
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "第一行\n第二行" || w.ai.input != "" {
		t.Fatalf("↵ 应发送: msgs=%d input=%q", len(w.ai.msgs), w.ai.input)
	}
	// ↑ 在空输入时找回上一条提问；⌘↵ 同样发送
	e.tt.Key(0, mui.KeyUp)
	e.tt.Frame()
	if w.ai.input != "第一行\n第二行" {
		t.Fatalf("↑ 应找回上一条提问: %q", w.ai.input)
	}
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if len(w.ai.msgs) != 4 {
		t.Fatalf("⌘↵ 应发送: msgs=%d", len(w.ai.msgs))
	}
}

func TestAssistantSessionsPopover(t *testing.T) {
	e := newEnv(t)
	srv, _ := fakeLLM(t, "SELECT COUNT(*) AS n FROM users")
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("有多少用户？")
	w.ai.openSessions()
	e.tt.Frame()
	e.tt.Frame()
	e.tt.Frame()
	e.has("历史对话")
	e.has("有多少用户？")
	e.shot("96-ai-sessions-popover")
}
