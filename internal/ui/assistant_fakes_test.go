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
)

// fakeLLM speaks just enough of the Chat Completions API: the first request
// of a conversation asks for a tool, the next answers in text.
func fakeLLM(t *testing.T, sql string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	return fakeLLMWith(t, sql, "")
}

// fakeLLMWith 同 fakeLLM；narration 非空时，模型在调用工具之前先说一句话。
func fakeLLMWith(t *testing.T, sql, narration string) (*httptest.Server, *atomic.Int32) {
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
			if narration != "" {
				send(map[string]any{"content": narration}, nil)
			}
			args, _ := json.Marshal(map[string]any{"sql": sql, "limit": 10})
			send(map[string]any{"tool_calls": []map[string]any{{"index": 0, "id": "call_1", "type": "function",
				"function": map[string]any{"name": "sql_readonly_execute", "arguments": string(args)}}}}, nil)
			send(map[string]any{}, "tool_calls")
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &chatCalls
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
