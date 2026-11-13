package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// responsesServer 模拟 DeepSeek 这类不保存响应的 /responses 网关：
// 第一次请求要求调用 table_stats；第二次请求（带 function_call_output）回答文本。
// 与真实网关一致：只要工具输出在同一次请求的输入里找不到对应的函数调用，就返回 400。
func responsesServer(t *testing.T, requests *[]string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		*requests = append(*requests, string(body))
		mu.Unlock()
		hasOutput := strings.Contains(string(body), `"function_call_output"`)
		if hasOutput && !strings.Contains(string(body), `"type":"function_call"`) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"message":"No tool callfound for tool output with call_id call_1.","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(v map[string]any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		completed := func(id string) map[string]any {
			return map[string]any{"type": "response.completed", "sequence_number": 9, "response": map[string]any{
				"id": id, "object": "response", "status": "completed", "output": []any{},
				"usage": map[string]any{"input_tokens": 100, "output_tokens": 20, "total_tokens": 120,
					"input_tokens_details": map[string]any{"cached_tokens": 0}, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}}}
		}
		if hasOutput {
			send(map[string]any{"type": "response.output_text.delta", "sequence_number": 1, "delta": "共 ", "item_id": "m", "output_index": 0, "content_index": 0})
			send(map[string]any{"type": "response.output_text.delta", "sequence_number": 2, "delta": "450 行", "item_id": "m", "output_index": 0, "content_index": 0})
			send(completed("resp_2"))
			return
		}
		send(map[string]any{"type": "response.output_text.delta", "sequence_number": 1, "delta": "我先看看统计。", "item_id": "m", "output_index": 0, "content_index": 0})
		send(map[string]any{"type": "response.output_item.done", "sequence_number": 2, "output_index": 1, "item": map[string]any{
			"id": "fc_1", "type": "function_call", "call_id": "call_1", "name": "table_stats", "arguments": `{"table_names":["users"]}`, "status": "completed"}})
		send(completed("resp_1"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResponsesRunOrdersTextToolTextAndReportsUsage(t *testing.T) {
	var requests []string
	srv := responsesServer(t, &requests)
	r := &ResponsesReActClient{client: openai.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL)), config: &Config{Model: "gpt-4o"}}

	var events []string
	var usage Usage
	out, err := r.Run(context.Background(), "sys", []ChatMessage{{Role: "user", Content: "多少行"}},
		[]FunctionToolDefinition{{Name: "table_stats", Description: "stats", Parameters: map[string]any{"type": "object"}}}, 5,
		func(call FunctionToolCall) ToolExecutionResult { return ToolExecutionResult{Output: "users: 450"} },
		ToolStreamCallbacks{
			OnRound:      func(n int) { events = append(events, fmt.Sprint("round", n)) },
			OnDelta:      func(s string) { events = append(events, "text:"+s) },
			OnToolCall:   func(c FunctionToolCall) { events = append(events, "call:"+c.Name) },
			OnToolResult: func(id, name, output string, ms int64) { events = append(events, "result:"+output) },
			OnUsage:      func(u Usage) { usage.TotalTokens += u.TotalTokens },
		})
	if err != nil {
		t.Fatal(err)
	}
	if out != "共 450 行" {
		t.Fatalf("final answer = %q (must be the text after the last tool, not the narration before it)", out)
	}
	want := "round1 text:我先看看统计。 call:table_stats result:users: 450 round2 text:共  text:450 行"
	if got := strings.Join(events, " "); got != want {
		t.Fatalf("event order\n got: %s\nwant: %s", got, want)
	}
	if usage.TotalTokens != 240 {
		t.Fatalf("usage should add up across both rounds: %d", usage.TotalTokens)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two requests: %v", requests)
	}
	// 无状态续接：第二次请求自带完整上下文，不能依赖服务端保存的上一轮响应
	second := requests[1]
	if strings.Contains(second, "previous_response_id") {
		t.Fatalf("gateways like DeepSeek do not store responses, so previous_response_id must not be sent: %s", second)
	}
	for _, want := range []string{"多少行", "我先看看统计。", `"type":"function_call"`, `"call_id":"call_1"`, `"name":"table_stats"`, `"function_call_output"`, "users: 450"} {
		if !strings.Contains(second, want) {
			t.Fatalf("second request must replay %q so the tool output has its call: %s", want, second)
		}
	}
	if strings.Contains(second, `"id":"fc_1"`) {
		t.Fatalf("replayed function_call must not carry the server item id: %s", second)
	}
}

func TestResponsesRunDoesNotExecuteToolsOfAFailedStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"sequence_number\":1,\"item\":{\"id\":\"fc_1\",\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"table_stats\",\"arguments\":\"{}\",\"status\":\"completed\"}}\n\n")
		fmt.Fprint(w, "data: {\n\n")
	}))
	defer srv.Close()
	r := &ResponsesReActClient{client: openai.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL)), config: &Config{Model: "m"}}
	called := 0
	_, err := r.Run(context.Background(), "s", []ChatMessage{{Role: "user", Content: "x"}},
		[]FunctionToolDefinition{{Name: "table_stats"}}, 1,
		func(FunctionToolCall) ToolExecutionResult { called++; return ToolExecutionResult{} },
		ToolStreamCallbacks{OnToolCall: func(FunctionToolCall) { called++ }})
	if err == nil || called != 0 {
		t.Fatalf("a malformed stream must fail without running tools: err=%v called=%d", err, called)
	}
}

// chatGateway 模拟只支持 Chat Completions 的网关：/responses 返回 404。
func chatGateway(t *testing.T, hits *map[string]int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		(*hits)[r.URL.Path]++
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/responses") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"message":"not found"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range []string{`{"content":"hello "}`, `{"content":"world"}`, `{}`} {
			finish := "null"
			if c == `{}` {
				finish = `"stop"`
			}
			fmt.Fprintf(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":%s,\"finish_reason\":%s}]}\n\n", c, finish)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClientFallsBackOnceThenSkipsResponses(t *testing.T) {
	hits := map[string]int{}
	srv := chatGateway(t, &hits)
	c := NewClient(&Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	for i := 0; i < 2; i++ {
		var deltas []string
		out, err := c.ChatWithToolsStream(context.Background(), "s", []ChatMessage{{Role: "user", Content: "hi"}}, nil, 1, nil,
			ToolStreamCallbacks{OnDelta: func(s string) { deltas = append(deltas, s) }})
		if err != nil || out != "hello world" || len(deltas) != 2 {
			t.Fatalf("run %d: out=%q err=%v deltas=%v", i, out, err, deltas)
		}
	}
	if hits["/responses"] != 1 || hits["/chat/completions"] != 2 {
		t.Fatalf("the 404 trips the breaker once; later runs go straight to chat: %v", hits)
	}
}

func TestToolErrorsAreMarkedForTheModel(t *testing.T) {
	text, _ := executeToolCall(func(FunctionToolCall) ToolExecutionResult {
		return ToolExecutionResult{Err: fmt.Errorf("表不存在")}
	}, FunctionToolCall{Name: "x"})
	if text != "ERROR: 表不存在" {
		t.Fatalf("text = %q", text)
	}
	text, _ = executeToolCall(func(FunctionToolCall) ToolExecutionResult {
		return ToolExecutionResult{Output: "ERROR: 已带前缀\n建议：换个表", Err: fmt.Errorf("x")}
	}, FunctionToolCall{})
	if text != "ERROR: 已带前缀\n建议：换个表" {
		t.Fatalf("an output that already says ERROR is kept: %q", text)
	}
	if text, _ = executeToolCall(nil, FunctionToolCall{}); !strings.HasPrefix(text, "ERROR") {
		t.Fatalf("no executor: %q", text)
	}
}

// TestChatCompatRunsToolLoopWhenGatewayReportsStop 模拟 DeepSeek 思考模式的 Chat 网关：
// 带工具调用却返回 finish_reason=stop，并随流给出 reasoning_content；下一轮必须把推理原样回传。
func TestChatCompatRunsToolLoopWhenGatewayReportsStop(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		n := len(bodies)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(delta, finish string) {
			fmt.Fprintf(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":%s,\"finish_reason\":%s}]}\n\n", delta, finish)
		}
		if n == 1 {
			chunk(`{"reasoning_content":"先看统计"}`, "null")
			chunk(`{"tool_calls":[{"index":0,"id":"call_9","type":"function","function":{"name":"table_stats","arguments":"{\"table_names\":[\"users\"]}"}}]}`, "null")
			chunk(`{}`, `"stop"`)
		} else {
			chunk(`{"content":"共 450 行"}`, "null")
			chunk(`{}`, `"stop"`)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := &ChatCompatClient{client: openai.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL)), config: &Config{Model: "m"}}
	out, err := c.Run(context.Background(), "sys", []ChatMessage{{Role: "user", Content: "多少行"}},
		[]FunctionToolDefinition{{Name: "table_stats", Parameters: map[string]any{"type": "object"}}}, 5,
		func(call FunctionToolCall) ToolExecutionResult { return ToolExecutionResult{Output: "users: 450"} },
		ToolStreamCallbacks{})
	if err != nil || out != "共 450 行" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected two requests, got %d", len(bodies))
	}
	for _, want := range []string{`"reasoning_content":"先看统计"`, `"tool_call_id":"call_9"`, `users: 450`} {
		if !strings.Contains(bodies[1], want) {
			t.Fatalf("second request must contain %s: %s", want, bodies[1])
		}
	}
}

// TestResponsesFinalizeReplaysHistoryAtRoundLimit 到达轮次上限后的总结请求同样是无状态的：
// 必须带上全部历史（含函数调用与输出），且不再提供工具。
func TestResponsesFinalizeReplaysHistoryAtRoundLimit(t *testing.T) {
	var requests []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, string(body))
		n := len(requests)
		mu.Unlock()
		hasOutput := strings.Contains(string(body), `"function_call_output"`)
		if hasOutput && !strings.Contains(string(body), `"type":"function_call"`) {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"message":"No tool callfound"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(v map[string]any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		if n == 1 {
			send(map[string]any{"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": map[string]any{
				"id": "fc_1", "type": "function_call", "call_id": "call_1", "name": "table_stats", "arguments": `{}`, "status": "completed"}})
		} else {
			send(map[string]any{"type": "response.output_text.delta", "sequence_number": 1, "delta": "总结", "item_id": "m", "output_index": 0, "content_index": 0})
		}
	}))
	defer srv.Close()

	r := &ResponsesReActClient{client: openai.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL)), config: &Config{Model: "m"}}
	out, err := r.Run(context.Background(), "sys", []ChatMessage{{Role: "user", Content: "问"}},
		[]FunctionToolDefinition{{Name: "table_stats", Parameters: map[string]any{"type": "object"}}}, 1,
		func(FunctionToolCall) ToolExecutionResult { return ToolExecutionResult{Output: "ok"} }, ToolStreamCallbacks{})
	if err != nil || out != "总结" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected tool round + summary round: %d", len(requests))
	}
	last := requests[1]
	for _, want := range []string{`"call_id":"call_1"`, `"function_call_output"`, "不要继续调用工具", "禁止继续调用任何工具"} {
		if !strings.Contains(last, want) {
			t.Fatalf("summary request must contain %q: %s", want, last)
		}
	}
	if strings.Contains(last, "previous_response_id") || strings.Contains(last, `"tools"`) {
		t.Fatalf("summary request is stateless and tool-free: %s", last)
	}
}
