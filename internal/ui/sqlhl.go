package ui

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// sqlKeywords 是日志里要染色的关键字（大小写不敏感）。
var sqlKeywords = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Fields(`select from where and or not in is null like between exists case when then else end
		insert into values update set delete create alter drop truncate table index view database schema column primary key
		foreign references unique default constraint add rename to as join left right inner outer full cross on using group
		by order having limit offset distinct union all asc desc begin commit rollback grant revoke with returning replace
		if explain analyze show use describe pragma`) {
		m[k] = true
	}
	return m
}()

// sqlSpans 把一段 SQL 切成带颜色的片段，给日志用：关键字、字符串、数字、注释各一种颜色。
// base 是普通文字的颜色；这是一个够用的词法扫描，不求覆盖各家方言的全部语法。
func sqlSpans(p tokens, sql string, base ui.Color) []ui.Span {
	var spans []ui.Span
	add := func(text string, col ui.Color, weight int, italic bool) {
		if text == "" {
			return
		}
		if n := len(spans); n > 0 && spans[n-1].Color == col && spans[n-1].Weight == weight && spans[n-1].Italic == italic {
			spans[n-1].Text += text
			return
		}
		spans = append(spans, ui.Span{Text: text, Color: col, Weight: weight, Italic: italic})
	}
	rs := []rune(sql)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '-' && i+1 < len(rs) && rs[i+1] == '-': // 行注释
			j := i
			for j < len(rs) && rs[j] != '\n' {
				j++
			}
			add(string(rs[i:j]), p.fgMuted, 0, true)
			i = j
		case r == '/' && i+1 < len(rs) && rs[i+1] == '*': // 块注释
			j := i + 2
			for j+1 < len(rs) && !(rs[j] == '*' && rs[j+1] == '/') {
				j++
			}
			j = min(j+2, len(rs))
			add(string(rs[i:j]), p.fgMuted, 0, true)
			i = j
		case r == '\'' || r == '"' || r == '`': // 字符串与带引号的标识符
			j := i + 1
			for j < len(rs) {
				if rs[j] == r {
					if j+1 < len(rs) && rs[j+1] == r { // 连写两个引号是转义
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			col := p.iconQuery
			if r != '\'' {
				col = base
			}
			add(string(rs[i:min(j, len(rs))]), col, 0, false)
			i = min(j, len(rs))
		case unicode.IsDigit(r):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.') {
				j++
			}
			add(string(rs[i:j]), p.valNum, 0, false)
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			word := string(rs[i:j])
			if sqlKeywords[strings.ToLower(word)] {
				add(word, p.iconView, 600, false)
			} else {
				add(word, base, 0, false)
			}
			i = j
		default:
			add(string(r), base, 0, false)
			i++
		}
	}
	return spans
}

// sqlLineMaxRunes 是列表行里参与高亮的最大字符数：行内只显示一行，更长的部分看不到，不必每帧都去扫描。
const sqlLineMaxRunes = 240

// sqlLineSpans 把 SQL 压成一行再做语法高亮，给列表里的单行显示用。
func sqlLineSpans(p tokens, sql string, base ui.Color) []ui.Span {
	line := oneLine(sql)
	if rs := []rune(line); len(rs) > sqlLineMaxRunes {
		line = string(rs[:sqlLineMaxRunes]) + "…"
	}
	return sqlSpans(p, line, base)
}
