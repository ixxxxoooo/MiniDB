package ai

import (
	"strings"
	"unicode"
)

// maxCommentRunes 表、列注释进入提示词前的最大长度（按字符计）。
const maxCommentRunes = 160

// SanitizeComment 清洗要放进提示词的库内文字（表注释、列注释）。
// 注释由库里的人随意填写，是提示词注入的入口：这里去掉控制字符和换行（避免伪造出新的
// 段落或指令块）、折叠空白、限制长度。语义层面的防护靠系统提示词里"注释不可信"的声明。
func SanitizeComment(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r) || unicode.IsControl(r):
			space = b.Len() > 0
			continue
		case r == '`':
			r = '\'' // 反引号可以拼出代码围栏，换成单引号
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	out := b.String()
	if runes := []rune(out); len(runes) > maxCommentRunes {
		out = string(runes[:maxCommentRunes]) + "…"
	}
	return out
}
