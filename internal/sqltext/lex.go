package sqltext

import (
	"regexp"
	"strings"
)

// TokenKind 是词法单元的种类。
type TokenKind int

const (
	// TokCode 是普通代码（含分号、空白、关键字、数字、运算符）。
	TokCode TokenKind = iota
	// TokComment 是注释（行注释或块注释）。
	TokComment
	// TokQuoted 是带引号的字符串或标识符（单引号、双引号、反引号），含引号本身。
	TokQuoted
	// TokDollar 是 PostgreSQL 的美元引用串（$$...$$ 或 $tag$...$tag$），含定界符。
	TokDollar
)

// Token 是源文本里一段连续的同种内容，[Start, End) 为字节范围。
type Token struct {
	Kind       TokenKind
	Start, End int
	// Exec 仅对 MySQL 可执行注释 /*! ... */ 为真：服务器会把它当代码执行。
	Exec bool
}

// LexOptions 决定各方言歧义写法的解释方式。
// 编辑器分句取最常见的写法；安全护栏则相反，遇到歧义一律按"代码"处理，
// 并对每种解释各扫一遍（见 database.CheckReadOnlySQL）。
type LexOptions struct {
	// BackslashEscapes：引号内的反斜杠转义下一个字符（MySQL 默认如此，PostgreSQL 不是）。
	BackslashEscapes bool
	// NestedComments：块注释可嵌套（PostgreSQL 如此，MySQL 不是）。
	NestedComments bool
	// HashComments：# 开头到行尾是注释（MySQL）；关闭时 # 按代码处理。
	HashComments bool
	// DollarQuotes：识别 $tag$...$tag$ 美元引用串；关闭时 $ 按代码处理，
	// 这样护栏会继续扫描其中的内容。
	DollarQuotes bool
	// StrictDash：-- 后必须跟空白才算行注释（MySQL 的要求）；关闭时 -- 一律是注释。
	StrictDash bool
	// UnclosedCommentIsCode：未闭合块注释之后的内容按代码处理（方言不同，执行与否不确定）；
	// 关闭时整段吞掉。
	UnclosedCommentIsCode bool
}

// EditorLexOptions 是编辑器分句、格式化使用的解释方式。
var EditorLexOptions = LexOptions{
	BackslashEscapes: true,
	HashComments:     true,
	DollarQuotes:     true,
}

var dollarTag = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*\$|^\$\$`)

// Lex 把 SQL 切成代码、注释、引号串、美元引用串。相邻的代码合并成一个 Token，
// 所有 Token 首尾相接，覆盖整个输入。
func Lex(sql string, o LexOptions) []Token {
	var out []Token
	n := len(sql)
	codeStart := -1
	flushCode := func(end int) {
		if codeStart >= 0 {
			out = append(out, Token{Kind: TokCode, Start: codeStart, End: end})
			codeStart = -1
		}
	}
	emit := func(kind TokenKind, start, end int, exec bool) {
		flushCode(start)
		out = append(out, Token{Kind: kind, Start: start, End: end, Exec: exec})
	}
	startCode := func(i int) {
		if codeStart < 0 {
			codeStart = i
		}
	}

	i := 0
	for i < n {
		c := sql[i]
		switch {
		case isLineCommentStart(sql, i, o):
			end := n
			if j := strings.IndexByte(sql[i:], '\n'); j >= 0 {
				end = i + j
			}
			emit(TokComment, i, end, false)
			i = end
		case c == '/' && i+1 < n && sql[i+1] == '*':
			end, closed := blockCommentEnd(sql, i, o.NestedComments)
			if !closed && o.UnclosedCommentIsCode {
				startCode(i)
				i = n
				break
			}
			emit(TokComment, i, end, i+2 < n && sql[i+2] == '!')
			i = end
		case c == '\'' || c == '"' || c == '`':
			end := quotedEnd(sql, i, o.BackslashEscapes)
			emit(TokQuoted, i, end, false)
			i = end
		case c == '$' && o.DollarQuotes:
			if tag := dollarTag.FindString(sql[i:min(i+64, n)]); tag != "" {
				end := n
				if j := strings.Index(sql[i+len(tag):], tag); j >= 0 {
					end = i + len(tag) + j + len(tag)
				}
				emit(TokDollar, i, end, false)
				i = end
				break
			}
			startCode(i)
			i++
		default:
			startCode(i)
			i++
		}
	}
	flushCode(n)
	return out
}

func isLineCommentStart(sql string, i int, o LexOptions) bool {
	n := len(sql)
	switch sql[i] {
	case '#':
		return o.HashComments
	case '-':
		if i+1 >= n || sql[i+1] != '-' {
			return false
		}
		if !o.StrictDash {
			return true
		}
		return i+2 >= n || isSpaceByte(sql[i+2])
	}
	return false
}

// blockCommentEnd 返回 i 处块注释的结束位置（不含则为 len(sql)）以及是否闭合。
func blockCommentEnd(sql string, i int, nested bool) (end int, closed bool) {
	n := len(sql)
	if !nested {
		j := strings.Index(sql[i+2:], "*/")
		if j < 0 {
			return n, false
		}
		return i + 2 + j + 2, true
	}
	depth := 1
	k := i + 2
	for k < n {
		switch {
		case sql[k] == '/' && k+1 < n && sql[k+1] == '*':
			depth++
			k += 2
		case sql[k] == '*' && k+1 < n && sql[k+1] == '/':
			depth--
			k += 2
			if depth == 0 {
				return k, true
			}
		default:
			k++
		}
	}
	return n, false
}

// quotedEnd 返回 i 处引号串的结束位置（含收尾引号；未闭合则为 len(sql)）。
func quotedEnd(sql string, i int, backslash bool) int {
	n := len(sql)
	quote := sql[i]
	j := i + 1
	for j < n {
		ch := sql[j]
		if backslash && quote != '`' && ch == '\\' && j+1 < n {
			j += 2
			continue
		}
		if ch == quote {
			if j+1 < n && sql[j+1] == quote { // 双写引号表示转义
				j += 2
				continue
			}
			return j + 1
		}
		j++
	}
	return n
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// Skeleton 去掉注释，把引号串和美元引用串的内容换成占位符，返回只含"代码"的骨架。
// executableComment 表示出现了 MySQL 可执行注释 /*! ... */。
func Skeleton(sql string, o LexOptions) (skeleton string, executableComment bool) {
	var b strings.Builder
	for _, t := range Lex(sql, o) {
		switch t.Kind {
		case TokCode:
			b.WriteString(sql[t.Start:t.End])
		case TokComment:
			if t.Exec {
				executableComment = true
			}
			b.WriteByte(' ')
		default:
			b.WriteString(" '' ")
		}
	}
	return b.String(), executableComment
}
