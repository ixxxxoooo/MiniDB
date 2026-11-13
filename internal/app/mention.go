package app

import (
	"strings"
	"unicode"

	"minidb/internal/ai"
)

// Mention 是文字里的一处 @表名 引用。
type Mention struct {
	Name       string // 不含 @
	Start, End int    // 在文字里的 rune 偏移，区间含 @
}

// mentionRune 报告 r 能不能出现在 @ 后面的名字里。点号只在两个名字字符之间才算（schema.table），见 FindMentions。
func mentionRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// asciiWord 报告 r 是不是 ASCII 的字母、数字或下划线：它们紧挨着 @ 时多半是邮箱，而紧挨着 @ 的汉字不是（"查@users"）。
func asciiWord(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// FindMentions 找出文字里所有 @表名：@ 要在开头或紧跟在非名字字符之后（所以邮箱里的 @ 不算），
// 名字由字母、数字、下划线、$ 组成，中间可以有点号。界面用它给引用上色、画 chip，后端用它决定要重点交给模型哪些表。
func FindMentions(text string) []Mention {
	rs := []rune(text)
	var out []Mention
	for i := 0; i < len(rs); i++ {
		if rs[i] != '@' || i > 0 && (asciiWord(rs[i-1]) || rs[i-1] == '.' || rs[i-1] == '@') {
			continue
		}
		j := i + 1
		for j < len(rs) && (mentionRune(rs[j]) || rs[j] == '.' && j+1 < len(rs) && mentionRune(rs[j+1]) && j > i+1) {
			j++
		}
		if j > i+1 {
			out = append(out, Mention{Name: string(rs[i+1 : j]), Start: i, End: j})
			i = j - 1
		}
	}
	return out
}

// MentionNames 是文字里被引用的表名，去重并保持出现的先后。
func MentionNames(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range FindMentions(text) {
		if key := strings.ToLower(m.Name); !seen[key] {
			seen[key] = true
			out = append(out, m.Name)
		}
	}
	return out
}

// mentionedTableLimit 一次最多把几张被点名的表的完整结构交给模型。
const mentionedTableLimit = 6

// resolveMentions 把对话里用户点名的表对到库结构上：越晚点名的越靠前，去重，最多 mentionedTableLimit 张。
// 写成 库名.表名 的取最后一段；没有对上的名字单独返回，让模型知道用户点了一个库里没有的名字。
func resolveMentions(schema *ai.SchemaContext, messages []ai.ChatMessage) (tables []ai.TableSchema, missing []string) {
	if schema == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		names := MentionNames(messages[i].Content)
		for j := len(names) - 1; j >= 0; j-- { // 同一条消息里后写的更靠前，和整体"越晚越靠前"一致
			name := names[j]
			if k := strings.LastIndexByte(name, '.'); k >= 0 {
				name = name[k+1:]
			}
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			found := false
			for _, t := range schema.Tables {
				if strings.EqualFold(t.Name, name) {
					found = true
					if len(tables) < mentionedTableLimit {
						tables = append(tables, t)
					}
					break
				}
			}
			if !found {
				missing = append(missing, names[j])
			}
		}
	}
	return tables, missing
}
