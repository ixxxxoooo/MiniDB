package schemaindex

import (
	"strings"
	"unicode"
)

// stopTokens 查询里没有区分度的词：常见虚词，以及"表""字段"这类在每个注释里都可能出现的词。
var stopTokens = map[string]bool{
	"的": true, "了": true, "吗": true, "呢": true, "和": true, "与": true, "在": true, "是": true, "有": true, "多少": true,
	"the": true, "a": true, "an": true, "of": true, "and": true, "or": true, "to": true, "in": true, "is": true, "table": true,
	"column": true, "field": true, "tables": true, "columns": true, "表": true, "字段": true, "列": true,
}

// hanSeparators 中文里几乎只起连接作用的单字：切词时当作分隔符。
var hanSeparators = map[rune]bool{'的': true, '了': true, '吗': true, '呢': true, '和': true, '与': true}

// tokenize 把标识符、注释或查询切成检索词：
//   - 英文按非字母数字、下划线、驼峰与字母/数字交界拆开（user_id、orderItems、v2Order）；
//   - 中文按相邻两字切（用户表 → 用户、户表），单字单独成词；
//   - 英文词做简单的单复数归一（orders → order，categories → category）。
func tokenize(s string) []string {
	var tokens []string
	for _, word := range splitWords(s) {
		if stopTokens[word] {
			continue
		}
		if isHan(word) {
			tokens = append(tokens, hanGrams(word)...)
			continue
		}
		tokens = append(tokens, singular(word))
	}
	return tokens
}

// splitWords 把文本切成"纯中文词"或"纯英文数字词"，其余字符当分隔符。
func splitWords(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	var prev rune
	for _, r := range s {
		switch {
		case hanSeparators[r]: // "订单的金额"：虚字不参与相邻两字切分，否则会产生"单的""的金"这样的噪声词
			flush()
			prev = 0
			continue
		case unicode.Is(unicode.Han, r):
			if len(cur) > 0 && !unicode.Is(unicode.Han, prev) {
				flush()
			}
			cur = append(cur, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			boundary := len(cur) > 0 && (unicode.Is(unicode.Han, prev) ||
				(unicode.IsLower(prev) && unicode.IsUpper(r)) || // orderItems
				(unicode.IsLetter(prev) != unicode.IsLetter(r))) // v2 / 2fa
			if boundary {
				flush()
			}
			cur = append(cur, r)
		default:
			flush()
			prev = 0
			continue
		}
		prev = r
	}
	flush()
	return words
}

// hanGrams 中文词的检索词：长度 1 取单字，否则取相邻两字。
func hanGrams(word string) []string {
	runes := []rune(word)
	if len(runes) == 1 {
		return []string{word}
	}
	grams := make([]string, 0, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		grams = append(grams, string(runes[i:i+2]))
	}
	return grams
}

func isHan(word string) bool {
	for _, r := range word {
		return unicode.Is(unicode.Han, r)
	}
	return false
}

// singular 英文词的粗略单数形式；太短的词不动，避免把 "bus"、"id" 之类改坏。
func singular(w string) string {
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 4 && strings.HasSuffix(w, "sses"):
		return w[:len(w)-2]
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is"):
		return w[:len(w)-1]
	}
	return w
}
