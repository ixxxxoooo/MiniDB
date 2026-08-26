package services

import (
	"fmt"
	"strings"
	"testing"
)

func TestFriendlyAIErrorRateLimit(t *testing.T) {
	err429 := fmt.Errorf("POST https://api.b.ai/v1/chat/completions: 429 Too Many Requests ")
	wrapped := friendlyAIError(err429)
	if !strings.Contains(wrapped.Error(), "限流") {
		t.Fatalf("429 should become friendly rate-limit message, got: %v", wrapped)
	}
	other := friendlyAIError(fmt.Errorf("connection refused"))
	if !strings.Contains(other.Error(), "connection refused") {
		t.Fatalf("non-rate-limit errors should pass through, got: %v", other)
	}
	if friendlyAIError(nil) != nil {
		t.Fatal("nil should stay nil")
	}
}

func TestIsRateLimitErrorText(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("429 Too Many Requests"), true},
		{fmt.Errorf("rate limit exceeded"), true},
		{fmt.Errorf("quota exceeded"), true},
		{fmt.Errorf("403 Forbidden"), false},
		{nil, false},
	}
	for i, c := range cases {
		if got := isRateLimitErrorText(c.err); got != c.want {
			t.Fatalf("case %d: isRateLimitErrorText(%v) = %v, want %v", i, c.err, got, c.want)
		}
	}
}