package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func TestEmitBufferedResponseOutputSuppressesAnswerWhenToolCallExists(t *testing.T) {
	var events []string

	emitBufferedResponseOutput("I need to inspect the table first.", true, ToolStreamCallbacks{
		OnFinalAnswer: func() {
			events = append(events, "answer_start")
		},
		OnDelta: func(delta string) {
			events = append(events, "delta:"+delta)
		},
	})

	if len(events) != 0 {
		t.Fatalf("tool round output should not be emitted as answer, got %#v", events)
	}
}

func TestEmitBufferedResponseOutputEmitsAssistantMessageWhenToolCallExists(t *testing.T) {
	var events []string

	emitBufferedResponseOutput("I need to inspect the table first.", true, ToolStreamCallbacks{
		OnFinalAnswer: func() {
			events = append(events, "answer_start")
		},
		OnDelta: func(delta string) {
			events = append(events, "delta:"+delta)
		},
		OnAssistantMessage: func(content string) {
			events = append(events, "assistant:"+content)
		},
	})

	if len(events) != 1 || events[0] != "assistant:I need to inspect the table first." {
		t.Fatalf("tool round content should be emitted as assistant message only, got %#v", events)
	}
}

func TestEmitBufferedResponseOutputEmitsAnswerWhenNoToolCall(t *testing.T) {
	var events []string

	emitBufferedResponseOutput("final answer", false, ToolStreamCallbacks{
		OnFinalAnswer: func() {
			events = append(events, "answer_start")
		},
		OnDelta: func(delta string) {
			events = append(events, "delta:"+delta)
		},
	})

	if len(events) != 2 {
		t.Fatalf("expected answer_start and delta, got %#v", events)
	}
	if events[0] != "answer_start" || events[1] != "delta:final answer" {
		t.Fatalf("unexpected event order: %#v", events)
	}
}

func TestEmitStreamingResponseDeltaStartsAnswerOnce(t *testing.T) {
	var events []string
	started := false

	callbacks := ToolStreamCallbacks{
		OnFinalAnswer: func() {
			events = append(events, "answer_start")
		},
		OnDelta: func(delta string) {
			events = append(events, "delta:"+delta)
		},
	}

	emitStreamingResponseDelta("hello ", &started, callbacks)
	emitStreamingResponseDelta("world", &started, callbacks)

	if len(events) != 3 {
		t.Fatalf("expected answer_start and two deltas, got %#v", events)
	}
	if events[0] != "answer_start" || events[1] != "delta:hello " || events[2] != "delta:world" {
		t.Fatalf("unexpected event order: %#v", events)
	}
}

func TestApplyReasoningSummaryRequestOnlyForReasoningModels(t *testing.T) {
	params := responses.ResponseNewParams{}
	if !applyReasoningSummaryRequest(&params, "gpt-5.1") {
		t.Fatal("expected reasoning summary to be enabled for gpt-5.1")
	}
	if params.Reasoning.Summary != shared.ReasoningSummaryAuto {
		t.Fatalf("unexpected reasoning summary: %q", params.Reasoning.Summary)
	}
	if params.Reasoning.Effort != shared.ReasoningEffortLow {
		t.Fatalf("unexpected reasoning effort: %q", params.Reasoning.Effort)
	}

	plainParams := responses.ResponseNewParams{}
	if applyReasoningSummaryRequest(&plainParams, "gpt-4o") {
		t.Fatal("non-reasoning GPT models should not receive reasoning params")
	}
	if plainParams.Reasoning.Summary != "" {
		t.Fatalf("unexpected reasoning summary on plain model: %q", plainParams.Reasoning.Summary)
	}

	o1Params := responses.ResponseNewParams{}
	if applyReasoningSummaryRequest(&o1Params, "o1-preview") {
		t.Fatal("older o1 models should not receive summary params")
	}
}

func TestResponsesReActFallbackCooldownActiveAndExpires(t *testing.T) {
	client := NewClient(&Config{Model: "gpt-5.5"})
	start := time.Now()

	client.disableResponsesReActTemporarily(errors.New("broken stream"))

	active, until, why := client.responsesReActFallbackActive(start)
	if !active {
		t.Fatal("expected Responses ReAct fallback cooldown to be active")
	}
	if why != "broken stream" {
		t.Fatalf("unexpected cooldown reason: %q", why)
	}
	if until.Before(start.Add(responsesReActFallbackCooldown-time.Second)) || until.After(start.Add(responsesReActFallbackCooldown+time.Second)) {
		t.Fatalf("unexpected cooldown deadline: %s", until)
	}

	active, _, why = client.responsesReActFallbackActive(until.Add(time.Nanosecond))
	if active {
		t.Fatal("expected Responses ReAct fallback cooldown to expire")
	}
	if why != "" {
		t.Fatalf("expected expired cooldown reason to be cleared, got %q", why)
	}
}

func TestUpdateConfigClearsResponsesReActFallbackCooldown(t *testing.T) {
	client := NewClient(&Config{Model: "gpt-5.5"})
	client.disableResponsesReActTemporarily(errors.New("broken stream"))

	client.UpdateConfig(&Config{Model: "gpt-4o"})

	active, _, why := client.responsesReActFallbackActive(time.Now())
	if active {
		t.Fatal("expected config update to clear Responses ReAct fallback cooldown")
	}
	if why != "" {
		t.Fatalf("expected cooldown reason to be cleared, got %q", why)
	}
	if client.ModelName() != "gpt-4o" {
		t.Fatalf("expected updated model name, got %q", client.ModelName())
	}
}

func TestUpdateConfigPreservesResponsesReActFallbackCooldownWhenProviderUnchanged(t *testing.T) {
	client := NewClient(&Config{
		BaseURL:      "https://ai.mxou.cn/v1",
		APIKey:       "key",
		Model:        "gpt-5.5",
		SystemPrompt: "old prompt",
		Headers:      map[string]string{"X-Provider": "mxou"},
	})
	client.disableResponsesReActTemporarily(errors.New("broken stream"))

	client.UpdateConfig(&Config{
		BaseURL:      " https://ai.mxou.cn/v1 ",
		APIKey:       "key",
		Model:        "gpt-5.5",
		SystemPrompt: "new prompt",
		Headers:      map[string]string{"X-Provider": "mxou"},
	})

	active, _, why := client.responsesReActFallbackActive(time.Now())
	if !active {
		t.Fatal("expected unchanged provider config reload to preserve Responses ReAct fallback cooldown")
	}
	if why != "broken stream" {
		t.Fatalf("unexpected cooldown reason: %q", why)
	}
}

func TestChatCompatRunStreamsContentDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello \"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"world\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.5\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	compat := &ChatCompatClient{
		client: client,
		config: &Config{Model: "gpt-5.5"},
	}

	var assistantDeltas []string
	var finalDeltas []string
	result, err := compat.Run(
		context.Background(),
		"system",
		[]ChatMessage{{Role: "user", Content: "say hello"}},
		nil,
		1,
		nil,
		ToolStreamCallbacks{
			OnAssistantMessage: func(content string) {
				assistantDeltas = append(assistantDeltas, content)
			},
			OnDelta: func(delta string) {
				finalDeltas = append(finalDeltas, delta)
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("unexpected result: %q", result)
	}
	if len(assistantDeltas) != 2 || assistantDeltas[0] != "hello " || assistantDeltas[1] != "world" {
		t.Fatalf("expected streamed assistant deltas, got %#v", assistantDeltas)
	}
	if len(finalDeltas) != 0 {
		t.Fatalf("content should not be emitted again after streamed deltas, got %#v", finalDeltas)
	}
}

func TestResponsesRunDoesNotEmitToolCallBeforeFailedStreamCompletes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"sequence_number\":1,\"item\":{\"id\":\"fc_1\",\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"table_stats\",\"arguments\":\"{\\\"table_names\\\":[\\\"users\\\"]}\",\"status\":\"completed\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\n\n"))
	}))
	defer server.Close()

	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	reAct := &ResponsesReActClient{
		client: client,
		config: &Config{Model: "gpt-5.5"},
	}

	toolStarted := 0
	toolArgsDone := 0
	executed := 0
	_, err := reAct.Run(
		context.Background(),
		"system",
		[]ChatMessage{{Role: "user", Content: "inspect users"}},
		[]FunctionToolDefinition{{
			Name:        "table_stats",
			Description: "stats",
			Parameters:  map[string]any{"type": "object"},
		}},
		1,
		func(call FunctionToolCall) ToolExecutionResult {
			executed++
			return ToolExecutionResult{Output: "{}"}
		},
		ToolStreamCallbacks{
			OnToolCall: func(call FunctionToolCall) {
				toolStarted++
			},
			OnToolArgumentsDone: func(call FunctionToolCall) {
				toolArgsDone++
			},
		},
	)

	if err == nil {
		t.Fatal("expected malformed stream to fail")
	}
	if toolStarted != 0 || toolArgsDone != 0 || executed != 0 {
		t.Fatalf("failed Responses stream should not emit or execute tool calls, got starts=%d args=%d executed=%d", toolStarted, toolArgsDone, executed)
	}
}
