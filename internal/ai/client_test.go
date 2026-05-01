package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
