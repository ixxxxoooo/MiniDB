package ai

import "testing"

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
