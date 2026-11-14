package sqltext

import "strings"

// 全仓库的字符串字面量转义都收在这个文件里，不要再各写一份 strings.ReplaceAll(s, "'", "''")。

// QuoteString 生成标准 SQL 的字符串字面量：两侧加单引号，内部的单引号双写。
// 这是 PostgreSQL、SQLite 的规则，也是不关心具体方言的场景（导出、复制为 SQL）的默认写法。
// MySQL 系默认把反斜杠当转义符，要用 QuoteStringBackslash。
func QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// QuoteStringBackslash 生成 MySQL 系的字符串字面量：反斜杠写成 \\，单引号写成 \'。
// 不转义反斜杠的话，以反斜杠结尾的值会把结尾引号"吃掉"。
func QuoteStringBackslash(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
