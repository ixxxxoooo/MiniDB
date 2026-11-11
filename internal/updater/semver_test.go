package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.1", "1.0.0", 1},
		{"1.2", "1.2.0", 0},
		{"1.10.0", "1.9.9", 1},
		{"0.0.1", "0.0.2", -1},
		{"1.2.0-beta.1", "1.2.0", -1},
		{"1.2.0", "1.2.0-rc.1", 1},
		{"1.2.0-beta.2", "1.2.0-beta.10", -1},
		{"1.2.0-alpha", "1.2.0-beta", -1},
		{"1.2.0-beta", "1.2.0-beta.1", -1},
		{"1.2.1-beta.1", "1.2.0", 1},
		{"1.2.0+build5", "1.2.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
