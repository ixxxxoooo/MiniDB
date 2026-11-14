package ui

import "testing"

// @author ixxxxoooo
//
// 侧栏行数的缩写。

func TestShortCount(t *testing.T) {
	for n, want := range map[int64]string{64: "64", 9999: "9999", 12400: "12.4k", 1_300_000: "1.3M"} {
		if got := shortCount(n); got != want {
			t.Errorf("shortCount(%d) = %q, want %q", n, got, want)
		}
	}
}
