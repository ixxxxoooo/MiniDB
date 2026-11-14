package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minidb/internal/app"
)

// toolThenAnswerLLM 第一轮调用 tool(args)，看到工具结果后回答 answer。
func toolThenAnswerLLM(t *testing.T, tool, args, answer string) *httptest.Server {
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
		if strings.Contains(string(body), `"role":"tool"`) {
			send(map[string]any{"role": "assistant", "content": answer}, nil)
			send(map[string]any{}, "stop")
		} else {
			send(map[string]any{"tool_calls": []map[string]any{{"index": 0, "id": "call_1", "type": "function",
				"function": map[string]any{"name": tool, "arguments": args}}}}, nil)
			send(map[string]any{}, "tool_calls")
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (e *env) useLLM(srv *httptest.Server) {
	e.t.Helper()
	if err := e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"}); err != nil {
		e.t.Fatal(err)
	}
	e.sh.app.AI.ReloadConfig()
	e.sh.prefs.AIDisclosed = true // 其余测试不关心首次使用的数据外发说明
}

// 模型在调用工具前说的话是过程的一部分：按发生的顺序排在工具之前，不会和最终回答粘在一起。
func TestAssistantKeepsNarrationOutOfTheAnswer(t *testing.T) {
	e := newEnv(t)
	srv, _ := fakeLLMWith(t, "SELECT COUNT(*) AS n FROM users", "我先数一下用户表。")
	e.useLLM(srv)
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("有多少用户？")
	reply := w.ai.msgs[1]

	var kinds []string
	for _, p := range reply.tr.Parts {
		kinds = append(kinds, string(p.Type))
	}
	if strings.Join(kinds, ",") != "thinking,text,tool,text" {
		t.Fatalf("parts in the order they happened: %v", kinds)
	}
	if got := reply.answer(); strings.Contains(got, "我先数一下") || !strings.Contains(got, "450") {
		t.Fatalf("the answer is only what follows the last tool: %q", got)
	}
	if reply.tail() != 3 || len(reply.work()) != 3 {
		t.Fatalf("tail=%d work=%d", reply.tail(), len(reply.work()))
	}
	// 过程按发生的顺序排在回复里：说明文字 → 工具调用 → 回答，不收进折叠的日志
	e.has("我先数一下用户表")
	e.has("共有 450 个用户")
	at := func(sub string) int {
		for i, tx := range e.tt.Texts() {
			if strings.Contains(tx, sub) {
				return i
			}
		}
		return -1
	}
	nar, tool, ans := at("我先数一下用户表"), at("运行只读 SQL"), at("共有 450 个用户")
	if nar < 0 || tool < 0 || ans < 0 || !(nar < tool && tool < ans) {
		t.Fatalf("顺序应是 说明(%d) → 工具(%d) → 回答(%d)", nar, tool, ans)
	}
}

// 失败的工具不用点开就能看到原因和下一步建议。
func TestAssistantShowsWhyAToolFailed(t *testing.T) {
	e := newEnv(t)
	e.useLLM(toolThenAnswerLLM(t, "table_describe", `{"table_names":["usres"]}`, "没有这张表，你是想看 users 吗？"))
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("看看 usres 表")
	reply := w.ai.msgs[1]
	tool := reply.tr.Parts[0].Tool
	if tool == nil || !tool.Failed() || tool.Result.ErrorCode != "table_not_found" || !strings.Contains(tool.Result.Text, "users") {
		t.Fatalf("the failure names the table the user probably meant: %+v", tool)
	}
	e.has("失败") // 回答完成后工具都收成一行，失败的在标题行上有红色标记
	e.hasNot("建议：用 table_list")
	reply.part(0).open = 1 // 点开就是原因和建议
	e.has("表不存在")
	e.has("建议：用 table_list")
	e.shot("97-ai-tool-failed")
}

// 运行中的样子：思考、正在运行的工具；结果到达后原地更新。
func TestAssistantLiveStateUpdatesInPlace(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	live := &chatMsg{id: 1}
	w.ai.msgs = []*chatMsg{{id: 0, user: true, text: "表结构？", tr: app.AgentTranscript{Done: true}}, live}
	apply := func(typ app.AgentEventType, payload any) { live.tr.Apply(app.AgentEvent{Type: typ, Payload: payload}) }

	apply(app.AgentEventStepThinking, "先看看有哪些列")
	apply(app.AgentEventToolRequested, app.AgentToolCall{CallID: "c1", ToolName: "table_describe", Arguments: map[string]any{"table_names": []any{"users"}}})
	e.has("查看表结构")
	e.has("users")
	if live.part(1).grid != nil {
		t.Fatal("no grid before the result arrives")
	}
	apply(app.AgentEventToolResult, app.AgentToolResult{CallID: "c1", ToolName: "table_describe", OK: true, Kind: "rows", Summary: "1 张表 · 2 列",
		Columns: []string{"表", "列"}, Rows: []map[string]any{{"表": "users", "列": "id"}, {"表": "users", "列": "name"}}, Notes: "users 索引: idx_users_name (name)"})
	if live.part(1).grid == nil || len(live.part(1).grid.rows) != 2 {
		t.Fatal("the result grid is built when the result arrives")
	}
	live.part(1).open = 1
	e.has("1 张表 · 2 列")
	e.has("idx_users_name")
	apply(app.AgentEventAnswerDelta, "有两列")
	if live.tail() != 2 || live.answer() != "有两列" {
		t.Fatalf("streamed text after the tool is the answer: tail=%d", live.tail())
	}
	apply(app.AgentEventRunDone, app.AgentRunDone{Content: "users 有 id 和 name 两列。", Usage: app.AgentUsage{TotalTokens: 1234, DurationMs: 900}})
	e.has("users 有 id 和 name 两列。")
	e.has("1,234 tokens")
	e.shot("98-ai-live-done")
}

// 重新生成：保存的会话里不会重复同一个问题，也不会留着被丢掉的旧回答。
func TestAssistantRegenerateKeepsStoredSessionInSync(t *testing.T) {
	e := newEnv(t)
	srv, _ := fakeLLM(t, "SELECT COUNT(*) AS n FROM users")
	e.useLLM(srv)
	w := openWorkspace(e)
	w.ai.send("有多少用户？")
	w.ai.regenerate()
	if len(w.ai.msgs) != 2 {
		t.Fatalf("msgs=%d", len(w.ai.msgs))
	}
	stored, err := e.sh.app.AI.GetAISessionMessages(w.ai.session, 0)
	if err != nil || len(stored) != 2 || stored[0].Role != "user" || stored[1].Role != "assistant" {
		t.Fatalf("one question and one answer, not two of each: %v %+v", err, stored)
	}
	first := w.ai.session
	w.ai.reset()
	w.ai.send("再问一个")
	if w.ai.session == first || !strings.HasPrefix(w.ai.session, "s") {
		t.Fatalf("a new conversation gets a fresh id: %q vs %q", w.ai.session, first)
	}
}

func TestToolPresentationHelpers(t *testing.T) {
	if got := toolTarget(map[string]any{"table_names": []any{"a", "b"}}); got != "a, b" {
		t.Errorf("tables: %q", got)
	}
	if got := toolTarget(map[string]any{"keywords": []any{"金额", "amount"}}); got != "金额, amount" {
		t.Errorf("keywords: %q", got)
	}
	if got := toolTarget(map[string]any{"keyword": "order", "limit": nil}); got != "order" {
		t.Errorf("keyword: %q", got)
	}
	lines := argLines(map[string]any{"sql": "SELECT 1", "table_names": []any{"a", "b"}, "limit": 10.0, "columns": nil})
	if strings.Join(lines, "|") != "limit: 10|table_names: a, b" {
		t.Errorf("sql is shown on its own, nulls are skipped, keys sorted: %v", lines)
	}
	for _, name := range []string{"table_list", "schema_search", "table_describe", "table_stats", "table_sample", "column_values", "table_profile", "sql_readonly_execute", "sql_explain_plan"} {
		if toolLabel(name) == name || toolIcon(name) == icBot {
			t.Errorf("%s lacks a label or an icon", name)
		}
	}
}
