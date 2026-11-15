package ui

import (
	"testing"
	"time"
)

// 耗时的显示：不足 1 毫秒的用微秒（µs）显示，不再是「0 ms」。
func TestFmtDur(t *testing.T) {
	for d, want := range map[time.Duration]string{
		500 * time.Nanosecond:   "<1 µs",
		3 * time.Microsecond:    "3 µs",
		320 * time.Microsecond:  "320 µs",
		999 * time.Microsecond:  "999 µs",
		4200 * time.Microsecond: "4.2 ms",
		85 * time.Millisecond:   "85 ms",
		1500 * time.Millisecond: "1.5s",
		20558 * time.Millisecond: "20.6s",
		59999 * time.Millisecond: "1m 00s",
		83 * time.Second:         "1m 23s",
		3725 * time.Second:       "62m 05s",
	} {
		if got := fmtDur(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

// AI 回答下面的用量：耗时走 fmtDur（20,558 ms 写成 20.6s），token 数带千分位。
func TestAIUsageText(t *testing.T) {
	if got, want := aiUsageText(40852, 20558), "40,852 tokens · 20.6s"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	if got, want := aiUsageText(0, 83000), "1m 23s"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	if got := aiUsageText(100, 0); got != "" {
		t.Errorf("没有耗时不显示，got %q", got)
	}
}
