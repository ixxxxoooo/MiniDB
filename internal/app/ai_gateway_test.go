package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"minidb/internal/agent"
)

// sse 向 Responses 流写事件。
type sse struct {
	w   http.ResponseWriter
	seq int
}

func (s *sse) send(v map[string]any) {
	s.seq++
	v["sequence_number"] = s.seq
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "data: %s\n\n", b)
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *sse) text(delta string) {
	s.send(map[string]any{"type": "response.output_text.delta", "delta": delta, "item_id": "m", "output_index": 0, "content_index": 0})
}

func (s *sse) call(id, name, args string) {
	s.send(map[string]any{"type": "response.output_item.done", "output_index": 1, "item": map[string]any{
		"id": "fc_" + id, "type": "function_call", "call_id": id, "name": name, "arguments": args, "status": "completed"}})
}

func (s *sse) done(id string) {
	s.send(map[string]any{"type": "response.completed", "response": map[string]any{"id": id, "object": "response", "status": "completed", "output": []any{},
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 20, "total_tokens": 120,
			"input_tokens_details": map[string]any{"cached_tokens": 0}, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}}})
}

// gateway 模拟 Responses 网关：handler 拿到本次是第几次请求、请求体和流写入器。
func gateway(t *testing.T, handler func(n int, body string, s *sse)) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		n := len(bodies)
		mu.Unlock()
		// 与 DeepSeek 等不保存响应的网关一致：工具输出在同一次请求里找不到对应的函数调用就报 400
		if strings.Contains(string(raw), `"function_call_output"`) && !strings.Contains(string(raw), `"type":"function_call"`) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"No tool callfound for tool output with call_id x.","type":"invalid_request_error"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		handler(n, string(raw), &sse{w: w})
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

// useGateway 把助手指向假网关。
func (h *aiHarness) useGateway(t *testing.T, url string) {
	t.Helper()
	if err := h.a.Settings.SaveAIConfig(AIConfig{BaseURL: url, APIKey: "k", Model: "gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	h.a.AI.ReloadConfig()
}

// record 订阅事件，返回读取已收到事件的函数。
func (h *aiHarness) record() func() []agent.Event {
	var mu sync.Mutex
	var evs []agent.Event
	h.a.Bus.On(TopicAgent, func(p any) {
		mu.Lock()
		evs = append(evs, p.(agent.Event))
		mu.Unlock()
	})
	return func() []agent.Event { mu.Lock(); defer mu.Unlock(); return append([]agent.Event(nil), evs...) }
}

func fold(evs []agent.Event) *agent.Transcript {
	var tr agent.Transcript
	for _, ev := range evs {
		tr.Apply(ev)
	}
	return &tr
}
