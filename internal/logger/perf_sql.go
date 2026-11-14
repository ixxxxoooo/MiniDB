package logger

import (
	"regexp"
	"strings"
)

// perfSQLMax 是日志里保留的 SQL 字符数。
const perfSQLMax = 160

// 单引号字符串字面量（含 \' 和 ” 转义）。字面量里可能有密码、令牌、个人数据，不进日志。
var reSQLLiteral = regexp.MustCompile(`'(?:[^'\\]|\\.|'')*'`)

// PerfSQL 把 SQL 整理成能放进性能日志的样子：折叠空白、字符串字面量换成 '?'、截断。
// 参数 sql 是原始语句；返回值只用于日志，不能再拿去执行。
func PerfSQL(sql string) string {
	s := strings.Join(strings.Fields(sql), " ")
	s = reSQLLiteral.ReplaceAllString(s, "'?'")
	if r := []rune(s); len(r) > perfSQLMax {
		s = string(r[:perfSQLMax]) + "…"
	}
	return s
}

// SQLText 把一条 SQL 作为打点属性传入：只有这行日志真的要写时才脱敏，平时零开销。
//
//	defer logger.Perf("db.query", "sql", logger.SQLText(query))()
type SQLText string

// String 实现 fmt.Stringer，输出脱敏后的语句。
func (s SQLText) String() string { return PerfSQL(string(s)) }
