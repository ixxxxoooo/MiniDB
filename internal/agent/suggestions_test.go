package agent

import (
	"strings"
	"testing"
)

func TestSplitSuggestions(t *testing.T) {
	body := "共有 **450** 个用户。"
	block := "\n\n```minidb-next-steps\n{\"choices\":[{\"label\":\"按月统计\",\"prompt\":\"按月统计新增用户\"},{\"label\":\"\",\"prompt\":\"空标签丢弃\"}]}\n```"
	got, sug := SplitSuggestions(body + block)
	if got != body || len(sug) != 1 || sug[0].Label != "按月统计" || sug[0].Prompt != "按月统计新增用户" {
		t.Fatalf("body=%q sug=%+v", got, sug)
	}
	if got, sug := SplitSuggestions(body); got != body || sug != nil {
		t.Fatalf("no block: %q %v", got, sug)
	}
	if got, sug := SplitSuggestions(body + "\n```minidb-next-steps\n{\"choices\":[{\"lab"); got != body || sug != nil {
		t.Fatalf("a half-streamed block must not leak into the body: %q", got)
	}
	if got, sug := SplitSuggestions(body + "\n```minidb-next-steps\n{not json}\n```"); got != body || sug != nil {
		t.Fatalf("invalid json: %q %v", got, sug)
	}
	many := strings.Repeat(`{"label":"a","prompt":"b"},`, 6)
	if _, sug := SplitSuggestions("x\n```minidb-next-steps\n{\"choices\":[" + strings.TrimSuffix(many, ",") + "]}\n```"); len(sug) != 4 {
		t.Fatalf("at most 4 buttons: %d", len(sug))
	}
}
