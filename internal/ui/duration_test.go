package ui

import (
	"testing"
	"time"
)

// 耗时的显示：不足 1 毫秒的也有数，不再是「0 ms」。
func TestFmtDur(t *testing.T) {
	for d, want := range map[time.Duration]string{
		3 * time.Microsecond:    "<0.01 ms",
		320 * time.Microsecond:  "0.32 ms",
		4200 * time.Microsecond: "4.2 ms",
		85 * time.Millisecond:   "85 ms",
		1500 * time.Millisecond: "1.5s",
	} {
		if got := fmtDur(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}
