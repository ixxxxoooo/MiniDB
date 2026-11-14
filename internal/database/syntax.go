package database

import (
	"fmt"
	"strings"
)

// Syntax 收拢各方言在「怎么写查询」上的差异：占位符、分页、转文本、取零行。
// 方言通过可选接口 SyntaxProvider 提供自己的写法；没有提供的用 MySQL 的通用写法（? 占位、LIMIT/OFFSET）。
// 业务代码一律走下面的 Placeholder、AppendPage 等函数，不再按类型名写 switch。
type Syntax interface {
	// Placeholder 是第 n 个（从 1 开始）参数的占位符。
	Placeholder(n int) string
	// CastText 把已引用的列转成可以做 LIKE 的文本。
	CastText(quotedCol string) string
	// AppendPage 在完整的 SELECT 后面追加分页；offset 小于 0 表示没有偏移（keyset 分页）。
	AppendPage(sel string, limit, offset int) string
	// ZeroRows 生成只取列信息、不取数据的查询；from 是已经引用好的表或子查询。
	ZeroRows(from string) string
	// HasRowLimit 报告语句最外层是不是已经限制了行数（LIMIT、TOP、FETCH）。
	HasRowLimit(sel string) bool
	// CountOf 生成数一条任意 SELECT 一共有多少行的语句。
	CountOf(sel string) string
}

// SyntaxProvider 由写法与通用写法不同的方言实现。
type SyntaxProvider interface{ Syntax() Syntax }

// stdSyntax 是通用写法：LIMIT/OFFSET 分页；numbered 为真时占位符是 $1、$2…，否则是 ?。
type stdSyntax struct {
	numbered bool
	textType string // CAST(x AS <textType>)；空表示 CHAR
}

func (s stdSyntax) Placeholder(n int) string {
	if s.numbered {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s stdSyntax) CastText(col string) string {
	t := s.textType
	if t == "" {
		t = "CHAR"
	}
	return fmt.Sprintf("CAST(%s AS %s)", col, t)
}

func (s stdSyntax) AppendPage(sel string, limit, offset int) string {
	if offset < 0 {
		return fmt.Sprintf("%s LIMIT %d", sel, limit)
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sel, limit, offset)
}

func (s stdSyntax) ZeroRows(from string) string { return "SELECT * FROM " + from + " LIMIT 0" }
func (s stdSyntax) HasRowLimit(sel string) bool { return hasLimitClause(sel) }
func (s stdSyntax) CountOf(sel string) string   { return buildAutoCountSQL(sel) }

// PostgreSQL：$n 占位符，转文本用 TEXT；SQLite：? 占位符，转文本用 TEXT。
func (postgresDialect) Syntax() Syntax { return stdSyntax{numbered: true, textType: "TEXT"} }
func (sqliteDialect) Syntax() Syntax   { return stdSyntax{textType: "TEXT"} }

// SyntaxOf 返回类型的查询写法；未知类型用通用写法。
func SyntaxOf(t DBType) Syntax {
	if d, ok := dialects[t]; ok {
		if p, ok := d.(SyntaxProvider); ok {
			return p.Syntax()
		}
	}
	return stdSyntax{}
}

// Placeholder 是第 n 个（从 1 开始）参数的占位符。
func Placeholder(t DBType, n int) string { return SyntaxOf(t).Placeholder(n) }

// AppendPage 在 SELECT 后追加分页；offset 小于 0 表示不带偏移。
func AppendPage(t DBType, sel string, limit, offset int) string {
	return SyntaxOf(t).AppendPage(sel, limit, offset)
}

// SelectZeroRows 生成只取列信息的查询，from 是已经引用好的表。
func SelectZeroRows(t DBType, from string) string { return SyntaxOf(t).ZeroRows(from) }

// topLevelIndex 返回括号深度为 0 处第一个（last 为真时是最后一个）以整词出现的关键字的位置，没有返回 -1。
// 扫描时跳过字符串、引用的标识符（"" “ []）和注释；kw 用大写，中间的空格匹配任意空白。
func topLevelIndex(sql, kw string, last bool) int {
	words := strings.Fields(kw)
	found, depth := -1, 0
	n := len(sql)
	for i := 0; i < n; i++ {
		switch c := sql[i]; {
		case c == '\'' || c == '"' || c == '`':
			for i++; i < n; i++ {
				if sql[i] == c {
					if i+1 < n && sql[i+1] == c { // 双写的引号是转义
						i++
						continue
					}
					break
				}
			}
		case c == '[':
			for i++; i < n && sql[i] != ']'; i++ {
			}
		case c == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			end := strings.Index(sql[i+2:], "*/")
			if end < 0 {
				return found
			}
			i += end + 3
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && (c|0x20) >= 'a' && (c|0x20) <= 'z' && (i == 0 || !isIdentByte(sql[i-1])):
			if end, ok := matchWords(sql, i, words); ok {
				found = i
				if !last {
					return found
				}
				i = end - 1
			}
		}
	}
	return found
}

// matchWords 判断 sql 从 i 起是不是依次出现 words（忽略大小写，词之间是空白，词尾是边界），返回结束位置。
func matchWords(sql string, i int, words []string) (int, bool) {
	p := i
	for k, w := range words {
		if k > 0 {
			q := p
			for q < len(sql) && (sql[q] == ' ' || sql[q] == '\t' || sql[q] == '\n' || sql[q] == '\r') {
				q++
			}
			if q == p {
				return 0, false
			}
			p = q
		}
		if p+len(w) > len(sql) || !strings.EqualFold(sql[p:p+len(w)], w) {
			return 0, false
		}
		p += len(w)
		if p < len(sql) && isIdentByte(sql[p]) {
			return 0, false
		}
	}
	return p, true
}

// PageQuery 给任意一条用户写的 SELECT 取一页：已经限制了行数的包成子查询再分页，否则直接追加分页。
func PageQuery(t DBType, sel string, limit, offset int) string {
	syn := SyntaxOf(t)
	if syn.HasRowLimit(sel) {
		sel = "SELECT * FROM (" + sel + ") AS __page__"
	}
	return syn.AppendPage(sel, limit, offset)
}
