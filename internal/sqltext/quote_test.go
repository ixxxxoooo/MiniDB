package sqltext

import "testing"

func TestQuoteString(t *testing.T) {
	cases := map[string]string{
		"":      "''",
		"abc":   "'abc'",
		"it's":  "'it''s'",
		`a\b`:   `'a\b'`,
		"''":    "''''''",
		"中文'引号": "'中文''引号'",
	}
	for in, want := range cases {
		if got := QuoteString(in); got != want {
			t.Errorf("QuoteString(%q) = %s，想要 %s", in, got, want)
		}
	}
}

func TestQuoteStringBackslash(t *testing.T) {
	cases := map[string]string{
		"":     "''",
		"it's": `'it\'s'`,
		`a\b`:  `'a\\b'`,
		`end\`: `'end\\'`,
		`\'`:   `'\\\''`,
	}
	for in, want := range cases {
		if got := QuoteStringBackslash(in); got != want {
			t.Errorf("QuoteStringBackslash(%q) = %s，想要 %s", in, got, want)
		}
	}
}
