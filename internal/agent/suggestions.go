package agent

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SuggestionTag 模型在回答末尾输出下一步建议所用的代码块语言标记。
const SuggestionTag = "minidb-next-steps"

var suggestionBlock = regexp.MustCompile("(?s)\\n*```" + SuggestionTag + "\\s*(\\{.*?\\})\\s*```\\s*$")

// SplitSuggestions 把回答末尾的"下一步建议"块拆出来：返回去掉该块的正文和解析出的按钮。
// 块是展示之外的结构化数据，不应进入回答正文、历史记录或复制出去的文字。
// 没有块、或块不合法时，正文原样返回（只去首尾空白）。
func SplitSuggestions(content string) (string, []Suggestion) {
	content = strings.TrimSpace(content)
	m := suggestionBlock.FindStringSubmatchIndex(content)
	if m == nil {
		return stripUnterminatedTag(content), nil
	}
	body := strings.TrimSpace(content[:m[0]])
	var payload struct {
		Choices []Suggestion `json:"choices"`
	}
	if err := json.Unmarshal([]byte(content[m[2]:m[3]]), &payload); err != nil {
		return body, nil
	}
	var out []Suggestion
	for _, c := range payload.Choices {
		label, prompt := strings.TrimSpace(c.Label), strings.TrimSpace(c.Prompt)
		if label == "" || prompt == "" || utf8.RuneCountInString(label) > 28 || utf8.RuneCountInString(prompt) > 200 {
			continue
		}
		out = append(out, Suggestion{Label: label, Prompt: prompt})
		if len(out) == 4 {
			break
		}
	}
	return body, out
}

// stripUnterminatedTag 去掉被截断（没有收尾 ```）的建议块，避免半截 JSON 出现在正文里。
func stripUnterminatedTag(content string) string {
	if i := strings.Index(content, "```"+SuggestionTag); i >= 0 {
		return strings.TrimSpace(content[:i])
	}
	return content
}
