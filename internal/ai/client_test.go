package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// apiError 向本地假网关发一次真实请求，返回 SDK 产生的 API 错误（含 HTTP 状态码）。
func apiError(t *testing.T, status int, body string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	client := openai.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	_, err := client.Responses.New(context.Background(), responses.ResponseNewParams{Model: shared.ResponsesModel("m")})
	if err == nil {
		t.Fatal("expected an API error")
	}
	return err
}

func TestFallbackDecisionUsesStatusCode(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		body         string
		wantFallback bool
		wantLimited  bool
	}{
		{"not found means endpoint missing", 404, `{"error":{"message":"not found"}}`, true, false},
		{"method not allowed", 405, `{"error":{"message":"no"}}`, true, false},
		{"rate limit is never a reason to switch endpoints", 429, `{"error":{"message":"slow down"}}`, false, true},
		{"bad key must not trip the breaker", 401, `{"error":{"message":"bad key responses"}}`, false, false},
		{"unrelated 400 must not trip the breaker", 400, `{"error":{"message":"max_tokens too large","type":"invalid_request_error"}}`, false, false},
		{"400 saying responses is unsupported", 400, `{"error":{"message":"responses api is not supported"}}`, true, false},
		{"deepseek cannot continue a tool call without stored state", 400, `{"error":{"message":"No tool callfound for tool output with call_id call_00_x","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`, true, false},
		{"gateway that cannot find the previous response", 400, `{"error":{"message":"previous_response_id is not found"}}`, true, false},
		{"server error", 500, `{"error":{"message":"boom"}}`, false, false},
	}
	for _, c := range cases {
		err := apiError(t, c.status, c.body)
		if got := shouldFallbackToChat(err); got != c.wantFallback {
			t.Errorf("%s: fallback=%v want %v (%v)", c.name, got, c.wantFallback, err)
		}
		if got := IsRateLimit(err); got != c.wantLimited {
			t.Errorf("%s: rateLimit=%v want %v", c.name, got, c.wantLimited)
		}
	}
	if shouldFallbackToChat(fmt.Errorf("AI Responses 流式响应失败: unexpected EOF")) {
		t.Error("a broken stream that merely mentions Responses is not a missing endpoint")
	}
	if !IsRateLimit(fmt.Errorf("Too Many Requests")) || IsRateLimit(fmt.Errorf("row id 14290 not found")) {
		t.Error("text matching should catch rate-limit phrases but not stray digits")
	}
}

func TestResponsesCooldownLifecycle(t *testing.T) {
	c := NewClient(&Config{BaseURL: "http://a", APIKey: "k", Model: "m"})
	if blocked, _ := c.responsesBlocked(time.Now()); blocked {
		t.Fatal("fresh client must try Responses")
	}
	c.blockResponses(fmt.Errorf("404"))
	if blocked, why := c.responsesBlocked(time.Now()); !blocked || why != "404" {
		t.Fatalf("blocked=%v why=%q", blocked, why)
	}
	c.UpdateConfig(&Config{BaseURL: "http://a", APIKey: "k", Model: "m", SystemPrompt: "只改提示词"})
	if blocked, _ := c.responsesBlocked(time.Now()); !blocked {
		t.Fatal("changing only the prompt keeps the same provider, so the breaker stays")
	}
	c.UpdateConfig(&Config{BaseURL: "http://b", APIKey: "k", Model: "m"})
	if blocked, _ := c.responsesBlocked(time.Now()); blocked {
		t.Fatal("a different endpoint must be tried again")
	}
	c.blockResponses(nil)
	if blocked, _ := c.responsesBlocked(time.Now().Add(responsesCooldown + time.Second)); blocked {
		t.Fatal("the breaker expires")
	}
}

func TestGenerationConfigSkipsTemperatureForReasoningModels(t *testing.T) {
	for model, wantTemp := range map[string]bool{"gpt-4o": true, "deepseek-chat": true, "gpt-5.1": false, "o3-mini": false, "deepseek-reasoner": false} {
		params := responses.ResponseNewParams{}
		applyGenerationConfig(&params, &Config{Model: model, Temperature: 0.3, MaxTokens: 512})
		if got := params.Temperature.Valid(); got != wantTemp {
			t.Errorf("%s: temperature sent=%v want %v", model, got, wantTemp)
		}
		if !params.MaxOutputTokens.Valid() {
			t.Errorf("%s: maxTokens must always be applied", model)
		}
	}
}

func TestApplyReasoningSummaryRequestOnlyForReasoningModels(t *testing.T) {
	params := responses.ResponseNewParams{}
	if !applyReasoningSummaryRequest(&params, "gpt-5.1") || params.Reasoning.Summary != shared.ReasoningSummaryAuto || params.Reasoning.Effort != shared.ReasoningEffortLow {
		t.Fatalf("gpt-5.1 should ask for a low-effort summary: %+v", params.Reasoning)
	}
	for _, model := range []string{"gpt-4o", "o1-preview", "deepseek-chat", ""} {
		p := responses.ResponseNewParams{}
		if applyReasoningSummaryRequest(&p, model) || p.Reasoning.Summary != "" {
			t.Errorf("%q must not receive reasoning params", model)
		}
	}
	pro := responses.ResponseNewParams{}
	applyReasoningSummaryRequest(&pro, "gpt-5-pro")
	if pro.Reasoning.Effort != shared.ReasoningEffortHigh {
		t.Fatalf("gpt-5-pro effort = %q", pro.Reasoning.Effort)
	}
}

func TestStrictSchemaForcesClosedObject(t *testing.T) {
	got := strictSchema(map[string]any{"properties": map[string]any{"b": map[string]any{"type": "string"}, "a": map[string]any{"type": "string"}}})
	if got["type"] != "object" || got["additionalProperties"] != false {
		t.Fatalf("schema = %+v", got)
	}
	if req, _ := got["required"].([]string); len(req) != 2 || req[0] != "a" {
		t.Fatalf("missing required defaults to every property, sorted: %+v", got["required"])
	}
	if empty := strictSchema(nil); empty["additionalProperties"] != false {
		t.Fatalf("nil schema must still be valid: %+v", empty)
	}
}
