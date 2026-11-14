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
	"time"
)

// sse 把若干事件拼成 Anthropic 风格的 SSE 响应体。
func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		var head struct{ Type string }
		_ = json.Unmarshal([]byte(e), &head)
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", head.Type, e)
	}
	return b.String()
}

func textTurn(text string) string {
	return sse(
		`{"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":5}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"ping"}`,
		fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, text),
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
		`{"type":"message_stop"}`,
	)
}

func toolTurn(lead, id, name string, argParts ...string) string {
	evs := []string{`{"type":"message_start","message":{"usage":{"input_tokens":20}}}`}
	if lead != "" {
		evs = append(evs,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, lead),
			`{"type":"content_block_stop","index":0}`)
	}
	evs = append(evs, fmt.Sprintf(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":%q,"name":%q,"input":{}}}`, id, name))
	for _, p := range argParts {
		evs = append(evs, fmt.Sprintf(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":%q}}`, p))
	}
	evs = append(evs,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
		`{"type":"message_stop"}`)
	return sse(evs...)
}

// fakeAnthropic 按顺序返回预设的响应，并记录每次请求的头和请求体。
type fakeAnthropic struct {
	mu      sync.Mutex
	replies []func(w http.ResponseWriter)
	headers []http.Header
	bodies  []map[string]any
	paths   []string
	srv     *httptest.Server
}

func newFakeAnthropic(t *testing.T, replies ...func(w http.ResponseWriter)) *fakeAnthropic {
	t.Helper()
	f := &fakeAnthropic{replies: replies}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		i := len(f.bodies)
		f.bodies, f.headers, f.paths = append(f.bodies, body), append(f.headers, r.Header.Clone()), append(f.paths, r.URL.Path)
		f.mu.Unlock()
		if i >= len(f.replies) {
			i = len(f.replies) - 1
		}
		f.replies[i](w)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func stream(body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}
}

func status(code int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}
}

func (f *fakeAnthropic) client(model string) *Client {
	return NewClient(&Config{Provider: ProviderAnthropic, BaseURL: f.srv.URL + "/v1", APIKey: "sk-ant", Model: model, Headers: map[string]string{"anthropic-beta": "x"}})
}

func fastRetry(t *testing.T) {
	old := anthropicRetryBase
	anthropicRetryBase = time.Millisecond
	t.Cleanup(func() { anthropicRetryBase = old })
}

func TestAnthropicSimpleChat(t *testing.T) {
	f := newFakeAnthropic(t, stream(textTurn("你好")))
	got, err := f.client("claude-x").Chat(context.Background(), "系统提示", "hi")
	if err != nil || got != "你好" {
		t.Fatalf("Chat = %q, %v", got, err)
	}
	if f.paths[0] != "/v1/messages" {
		t.Errorf("路径 %s", f.paths[0])
	}
	h := f.headers[0]
	if h.Get("x-api-key") != "sk-ant" || h.Get("anthropic-version") != anthropicVersion || h.Get("anthropic-beta") != "x" {
		t.Errorf("请求头不对: %v", h)
	}
	b := f.bodies[0]
	if b["model"] != "claude-x" || b["stream"] != true || b["max_tokens"] != float64(anthropicMaxTokens) || !strings.Contains(b["system"].(string), "系统提示") {
		t.Errorf("请求体不对: %v", b)
	}
	if _, has := b["tools"]; has {
		t.Error("没有工具时不该带 tools")
	}
}

func TestAnthropicToolLoop(t *testing.T) {
	f := newFakeAnthropic(t,
		stream(toolTurn("我来查一下。", "toolu_1", "run_sql", `{"sql":"SEL`, `ECT 1"}`)),
		stream(textTurn("结果是 1")),
	)
	cl := f.client("claude-x")
	cl.config.MaxTokens, cl.config.Temperature = 1000, 1.7
	var deltas []string
	var calls []FunctionToolCall
	var results []string
	var usage []Usage
	out, err := cl.ChatWithToolsStream(context.Background(), "sys",
		[]ChatMessage{{Role: "user", Content: "查 1"}},
		[]FunctionToolDefinition{{Name: "run_sql", Description: "执行 SQL", Parameters: map[string]any{
			"type": "object", "properties": map[string]any{"sql": map[string]any{"type": "string"}},
		}}},
		5,
		func(c FunctionToolCall) ToolExecutionResult {
			if c.Arguments != `{"sql":"SELECT 1"}` {
				t.Errorf("参数没有拼完整: %q", c.Arguments)
			}
			return ToolExecutionResult{Output: "1 row"}
		},
		ToolStreamCallbacks{
			OnDelta:      func(s string) { deltas = append(deltas, s) },
			OnToolCall:   func(c FunctionToolCall) { calls = append(calls, c) },
			OnToolResult: func(id, name, out string, _ int64) { results = append(results, id+"|"+name+"|"+out) },
			OnUsage:      func(u Usage) { usage = append(usage, u) },
		})
	if err != nil || out != "结果是 1" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if strings.Join(deltas, "") != "我来查一下。结果是 1" || len(calls) != 1 || calls[0].ID != "toolu_1" || len(results) != 1 || results[0] != "toolu_1|run_sql|1 row" {
		t.Errorf("回调不对: deltas=%v calls=%v results=%v", deltas, calls, results)
	}
	if len(usage) != 2 || usage[0].InputTokens != 20 || usage[0].OutputTokens != 9 || usage[1].InputTokens != 15 || usage[1].TotalTokens != 22 {
		t.Errorf("用量不对: %+v", usage)
	}

	first := f.bodies[0]
	if first["max_tokens"] != float64(1000) || first["temperature"] != float64(1) {
		t.Errorf("生成参数不对（温度应被限制在 1）: %v", first)
	}
	tools := first["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["input_schema"] == nil {
		t.Errorf("tools 不对: %v", first["tools"])
	}
	choice := first["tool_choice"].(map[string]any)
	if choice["type"] != "auto" || choice["disable_parallel_tool_use"] != true {
		t.Errorf("tool_choice: %v", choice)
	}
	// 第二轮的历史：用户 → 助手(text + tool_use) → 用户(tool_result)
	msgs := f.bodies[1]["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("第二轮应有 3 条消息: %v", msgs)
	}
	asst := msgs[1].(map[string]any)["content"].([]any)
	use := asst[1].(map[string]any)
	if asst[0].(map[string]any)["type"] != "text" || use["type"] != "tool_use" || use["id"] != "toolu_1" || use["input"].(map[string]any)["sql"] != "SELECT 1" {
		t.Errorf("助手消息不对: %v", asst)
	}
	res := msgs[2].(map[string]any)
	block := res["content"].([]any)[0].(map[string]any)
	if res["role"] != "user" || block["type"] != "tool_result" || block["tool_use_id"] != "toolu_1" || block["content"] != "1 row" {
		t.Errorf("tool_result 不对: %v", res)
	}
}

func TestAnthropicRoundLimitFinalizesWithoutTools(t *testing.T) {
	f := newFakeAnthropic(t,
		stream(toolTurn("", "toolu_1", "run_sql", `{}`)),
		stream(textTurn("总结")),
	)
	out, err := f.client("m").ChatWithToolsStream(context.Background(), "sys",
		[]ChatMessage{{Role: "user", Content: "q"}},
		[]FunctionToolDefinition{{Name: "run_sql"}}, 1,
		func(FunctionToolCall) ToolExecutionResult { return ToolExecutionResult{Err: fmt.Errorf("boom")} },
		ToolStreamCallbacks{})
	if err != nil || out != "总结" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	last := f.bodies[1]
	if last["tool_choice"].(map[string]any)["type"] != "none" || last["tools"] == nil {
		t.Errorf("总结轮应保留 tools 并用 tool_choice=none: %v", last)
	}
	msgs := last["messages"].([]any)
	blocks := msgs[len(msgs)-1].(map[string]any)["content"].([]any)
	if blocks[0].(map[string]any)["type"] != "tool_result" || blocks[0].(map[string]any)["is_error"] != true || blocks[1].(map[string]any)["type"] != "text" {
		t.Errorf("总结轮最后一条消息不对: %v", blocks)
	}
	if !strings.Contains(last["system"].(string), "工具调用轮次上限") {
		t.Error("总结轮应追加总结指令")
	}
}

func TestAnthropicErrors(t *testing.T) {
	fastRetry(t)
	ctx := context.Background()

	// 鉴权失败：不重试，错误里带上服务端消息
	f := newFakeAnthropic(t, status(401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	_, err := f.client("m").Chat(ctx, "s", "q")
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") || len(f.bodies) != 1 || IsRateLimit(err) {
		t.Fatalf("401: err=%v 请求次数=%d", err, len(f.bodies))
	}

	// 429 后恢复：重试成功
	f = newFakeAnthropic(t, status(429, `{"error":{"message":"slow"}}`), status(529, `{"error":{"message":"overloaded"}}`), stream(textTurn("ok")))
	if got, err := f.client("m").Chat(ctx, "s", "q"); err != nil || got != "ok" || len(f.bodies) != 3 {
		t.Fatalf("重试: got=%q err=%v 请求次数=%d", got, err, len(f.bodies))
	}

	// 一直过载：重试耗尽后被识别为限流
	f = newFakeAnthropic(t, status(529, `{"error":{"message":"overloaded"}}`))
	_, err = f.client("m").Chat(ctx, "s", "q")
	if err == nil || !IsRateLimit(err) || len(f.bodies) != anthropicRetries+1 {
		t.Fatalf("过载: err=%v 请求次数=%d", err, len(f.bodies))
	}

	// 流中途的 overloaded 错误事件
	f = newFakeAnthropic(t, stream(sse(`{"type":"message_start","message":{"usage":{}}}`, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)))
	if _, err = f.client("m").Chat(ctx, "s", "q"); err == nil || !IsRateLimit(err) {
		t.Fatalf("流内错误: %v", err)
	}
}

func TestAnthropicTruncatedStreamRunsNoTools(t *testing.T) {
	// 工具调用已经发完，但连接在 message_stop 前断了：不能执行工具
	body := strings.TrimSuffix(toolTurn("", "toolu_1", "run_sql", `{}`), "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	f := newFakeAnthropic(t, stream(body))
	ran := false
	_, err := f.client("m").ChatWithToolsStream(context.Background(), "s", []ChatMessage{{Role: "user", Content: "q"}},
		[]FunctionToolDefinition{{Name: "run_sql"}}, 3,
		func(FunctionToolCall) ToolExecutionResult { ran = true; return ToolExecutionResult{} }, ToolStreamCallbacks{})
	if err == nil || ran {
		t.Fatalf("截断的流应报错且不执行工具: err=%v ran=%v", err, ran)
	}
}

func TestAnthropicMaxTokensDropsToolCalls(t *testing.T) {
	body := strings.Replace(toolTurn("半截", "toolu_1", "run_sql", `{"sql":`), `"stop_reason":"tool_use"`, `"stop_reason":"max_tokens"`, 1)
	f := newFakeAnthropic(t, stream(body))
	ran := false
	out, err := f.client("m").ChatWithToolsStream(context.Background(), "s", []ChatMessage{{Role: "user", Content: "q"}},
		[]FunctionToolDefinition{{Name: "run_sql"}}, 3,
		func(FunctionToolCall) ToolExecutionResult { ran = true; return ToolExecutionResult{} }, ToolStreamCallbacks{})
	if err != nil || ran || out != "半截" {
		t.Fatalf("被长度截断的调用不该执行: out=%q err=%v ran=%v", out, err, ran)
	}
}

func TestBuildAnthropicMessages(t *testing.T) {
	system, msgs := buildAnthropicMessages("基础", []ChatMessage{
		{Role: "assistant", Content: "开头的助手消息要丢掉"},
		{Role: "system", Content: "额外规则"},
		{Role: "user", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "user", Content: "   "},
		{Role: "assistant", Content: "c"},
		{Role: "developer", Content: "开发者消息"},
		{Role: "user", Content: "d"},
	})
	if system != "基础\n\n额外规则\n\n开发者消息" {
		t.Errorf("system = %q", system)
	}
	want := []anthropicMsg{{"user", "a\n\nb"}, {"assistant", "c"}, {"user", "d"}}
	if len(msgs) != len(want) {
		t.Fatalf("msgs = %+v", msgs)
	}
	for i := range want {
		if msgs[i] != want[i] {
			t.Errorf("msgs[%d] = %+v, 期望 %+v", i, msgs[i], want[i])
		}
	}
	if _, err := newAnthropicLoop(&Config{}, "s", []ChatMessage{{Role: "assistant", Content: "x"}}, nil, ToolStreamCallbacks{}); err == nil {
		t.Error("没有用户消息时应报错")
	}
}

func TestToolInput(t *testing.T) {
	for in, want := range map[string]string{"": "{}", "  ": "{}", "null": "{}", "[1]": "{}", "{bad": "{}", `{"a":1}`: `{"a":1}`} {
		if got := string(toolInput(in)); got != want {
			t.Errorf("toolInput(%q) = %s, 期望 %s", in, got, want)
		}
	}
}

func TestProviderSelection(t *testing.T) {
	c := NewClient(&Config{})
	if _, ok := c.providerFor(&Config{Provider: " Anthropic "}).(anthropicProvider); !ok {
		t.Error("anthropic 应选中原生通道（忽略大小写和空白）")
	}
	for _, p := range []string{"", ProviderOpenAI, "未知"} {
		if _, ok := c.providerFor(&Config{Provider: p}).(openaiCompatProvider); !ok {
			t.Errorf("%q 应走 OpenAI 兼容通道", p)
		}
	}
	if sameProvider(&Config{Provider: "anthropic"}, &Config{}) {
		t.Error("协议不同不算同一个端点")
	}
}
