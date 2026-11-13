package app

import (
	"errors"
	"testing"
)

func TestFriendlyAIErrorRateLimit(t *testing.T) {
	if got := friendlyAIError(errors.New("429 Too Many Requests")); got == nil || got.Error() != "AI 服务当前繁忙（限流），请稍后重试" {
		t.Fatalf("rate limit should become a readable message: %v", got)
	}
	orig := errors.New("invalid api key")
	if got := friendlyAIError(orig); got != orig {
		t.Fatalf("other errors pass through untouched: %v", got)
	}
	if friendlyAIError(nil) != nil {
		t.Fatal("nil stays nil")
	}
}

func TestGenerateSessionTitleCutsByCharactersNotBytes(t *testing.T) {
	long := "请帮我统计最近三十天每个渠道的新增用户数量并按照转化率从高到低排序然后给出建议"
	title := generateSessionTitle(long)
	if []rune(title)[len([]rune(title))-1] != '…' || len([]rune(title)) != 31 {
		t.Fatalf("title = %q", title)
	}
	for _, r := range title {
		if r == '\uFFFD' {
			t.Fatalf("a byte-based cut would leave a broken character: %q", title)
		}
	}
	if got := generateSessionTitle("  a \n\n b  "); got != "a b" {
		t.Fatalf("whitespace is folded: %q", got)
	}
	if generateSessionTitle("  ") != "AI 对话" {
		t.Fatal("empty falls back to a default title")
	}
}
