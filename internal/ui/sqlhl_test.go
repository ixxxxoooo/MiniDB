package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSQLSpans(t *testing.T) {
	p := lightTokens
	base := ui.Hex("#111111")
	sp := sqlSpans(p, "SELECT id, 'a''b' FROM t -- 注释\nWHERE n >= 10.5", base)
	var all strings.Builder
	kinds := map[string]ui.Color{}
	for _, s := range sp {
		all.WriteString(s.Text)
		kinds[strings.TrimSpace(s.Text)] = s.Color
	}
	if all.String() != "SELECT id, 'a''b' FROM t -- 注释\nWHERE n >= 10.5" {
		t.Fatalf("切分后拼回来应与原文一致: %q", all.String())
	}
	if kinds["SELECT"] != p.iconView || kinds["'a''b'"] != p.iconQuery || kinds["10.5"] != p.valNum || kinds["-- 注释"] != p.fgMuted {
		t.Fatalf("颜色不对: %v", kinds)
	}
}
