package ai

import "testing"

func runSplit(chunks ...string) (th, tx string) {
	var s thinkSplitter
	for _, c := range chunks {
		a, b := s.feed(c)
		th, tx = th+a, tx+b
	}
	a, b := s.flush()
	return th + a, tx + b
}

func TestThinkSplitter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chunks []string
		th, tx string
	}{
		{"整块", []string{"<think>先看表</think>\n\n答案是 3"}, "先看表", "答案是 3"},
		{"标签被切开", []string{"<thi", "nk>先", "看表</th", "ink>答案"}, "先看表", "答案"},
		{"没有标签", []string{"直接", "回答 a < b"}, "", "直接回答 a < b"},
		{"结尾像半个标签", []string{"结果 <"}, "", "结果 <"},
		{"正文里提到标签", []string{"用 <think> 标签包住"}, "", "用 <think> 标签包住"},
		{"开头有空白", []string{"\n<think>想</think>好"}, "想", "\n好"},
		{"没闭合", []string{"<think>还在想"}, "还在想", ""},
	} {
		th, tx := runSplit(tc.chunks...)
		if th != tc.th || tx != tc.tx {
			t.Errorf("%s: thinking=%q text=%q, want %q / %q", tc.name, th, tx, tc.th, tc.tx)
		}
	}
}

func TestChatReasoningDeltaFields(t *testing.T) {
	for raw, want := range map[string]string{
		`{"reasoning_content":"a"}`:   "a",
		`{"reasoning":"b"}`:           "b",
		`{"reasoning":{"delta":"c"}}`: "c",
		`{"reasoning_details":[{"type":"reasoning.text","text":"d"},{"summary":"e"}]}`: "de",
		`{"thinking":"f"}`: "f",
		`{"content":"x"}`:  "",
	} {
		if got := chatReasoningDelta(raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}
