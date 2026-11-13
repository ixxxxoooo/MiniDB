package sqltext

import "strings"

// 查询参数：语句里的占位符在运行前由用户填值。占位格式有六种，默认不启用。

// ParamFormat 是占位符的写法。
type ParamFormat string

const (
	ParamColon       ParamFormat = "colon"       // :name
	ParamAt          ParamFormat = "at"          // @name
	ParamDollar      ParamFormat = "dollar"      // $name
	ParamBrace       ParamFormat = "brace"       // {name}
	ParamDoubleBrace ParamFormat = "doublebrace" // {{name}}
	ParamDollarBrace ParamFormat = "dollarbrace" // ${name}
)

// ParamFormats 按界面顺序列出全部格式。
var ParamFormats = []ParamFormat{ParamColon, ParamAt, ParamDollar, ParamBrace, ParamDoubleBrace, ParamDollarBrace}

// Example 是格式的写法示例，供界面显示。
func (f ParamFormat) Example() string {
	switch f {
	case ParamAt:
		return "@name"
	case ParamDollar:
		return "$name"
	case ParamBrace:
		return "{name}"
	case ParamDoubleBrace:
		return "{{name}}"
	case ParamDollarBrace:
		return "${name}"
	}
	return ":name"
}

// ParamFormatOf 把存下来的字符串还原成格式，认不得的按冒号处理。
func ParamFormatOf(s string) ParamFormat {
	for _, f := range ParamFormats {
		if string(f) == s {
			return f
		}
	}
	return ParamColon
}

// ParamRef 是语句里的一处占位符。
type ParamRef struct {
	Name       string
	Start, End int // 占位符在语句里的字节范围，含前后缀
}

func isParamStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isParamChar(b byte) bool { return isParamStart(b) || (b >= '0' && b <= '9') }

// FindParams 找出语句里的占位符。注释和反引号里的不算；字符串字面量里的只有
// inQuotes 为真时才算（"引号内占位"开关）。冒号格式不碰 PostgreSQL 的 ::类型转换，
// @ 格式不碰 MySQL 的 @@系统变量，$ 格式不碰 PostgreSQL 的 $1 位置参数。
func FindParams(sql string, f ParamFormat, inQuotes bool) []ParamRef {
	var out []ParamRef
	n := len(sql)
	var quote byte // 当前所在的引号，0 表示不在字符串里
	for i := 0; i < n; i++ {
		c := sql[i]
		switch {
		case quote != 0:
			if c == '\\' && quote != '`' && i+1 < n {
				i++
				continue
			}
			if c == quote {
				quote = 0
				continue
			}
			if !inQuotes || quote == '`' {
				continue
			}
		case c == '-' && i+1 < n && sql[i+1] == '-', c == '#':
			if j := strings.IndexByte(sql[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = n
			}
			continue
		case c == '/' && i+1 < n && sql[i+1] == '*':
			if j := strings.Index(sql[i+2:], "*/"); j >= 0 {
				i += 2 + j + 1
			} else {
				i = n
			}
			continue
		case c == '\'' || c == '"' || c == '`':
			quote = c
			continue
		}
		if ref, ok := paramAt(sql, i, f); ok {
			out = append(out, ref)
			i = ref.End - 1
		}
	}
	return out
}

// paramAt 判断 i 处是不是一个占位符的开头。
func paramAt(sql string, i int, f ParamFormat) (ParamRef, bool) {
	n := len(sql)
	name := func(from int) (string, int) {
		j := from
		if j >= n || !isParamStart(sql[j]) {
			return "", from
		}
		for j < n && isParamChar(sql[j]) {
			j++
		}
		return sql[from:j], j
	}
	prev := byte(0)
	if i > 0 {
		prev = sql[i-1]
	}
	switch f {
	case ParamColon:
		if sql[i] != ':' || prev == ':' || (i+1 < n && sql[i+1] == ':') || isParamChar(prev) {
			return ParamRef{}, false
		}
		if nm, end := name(i + 1); nm != "" {
			return ParamRef{nm, i, end}, true
		}
	case ParamAt:
		if sql[i] != '@' || prev == '@' || (i+1 < n && sql[i+1] == '@') || isParamChar(prev) {
			return ParamRef{}, false
		}
		if nm, end := name(i + 1); nm != "" {
			return ParamRef{nm, i, end}, true
		}
	case ParamDollar:
		if sql[i] != '$' || isParamChar(prev) {
			return ParamRef{}, false
		}
		if nm, end := name(i + 1); nm != "" {
			return ParamRef{nm, i, end}, true
		}
	case ParamBrace:
		if sql[i] != '{' || (i+1 < n && sql[i+1] == '{') || prev == '{' || prev == '$' {
			return ParamRef{}, false
		}
		if nm, end := name(i + 1); nm != "" && end < n && sql[end] == '}' {
			return ParamRef{nm, i, end + 1}, true
		}
	case ParamDoubleBrace:
		if sql[i] != '{' || i+1 >= n || sql[i+1] != '{' {
			return ParamRef{}, false
		}
		if nm, end := name(i + 2); nm != "" && end+1 < n && sql[end] == '}' && sql[end+1] == '}' {
			return ParamRef{nm, i, end + 2}, true
		}
	case ParamDollarBrace:
		if sql[i] != '$' || i+1 >= n || sql[i+1] != '{' {
			return ParamRef{}, false
		}
		if nm, end := name(i + 2); nm != "" && end < n && sql[end] == '}' {
			return ParamRef{nm, i, end + 1}, true
		}
	}
	return ParamRef{}, false
}

// ParamNames 列出一批语句里不重复的占位符名，按首次出现的顺序。
func ParamNames(stmts []string, f ParamFormat, inQuotes bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range stmts {
		for _, r := range FindParams(s, f, inQuotes) {
			if !seen[r.Name] {
				seen[r.Name] = true
				out = append(out, r.Name)
			}
		}
	}
	return out
}

// ApplyParams 把占位符换成用户填的值。值原样写入，由用户自己带引号；
// 没有填值的占位符保持原样。
func ApplyParams(sql string, f ParamFormat, inQuotes bool, values map[string]string) string {
	refs := FindParams(sql, f, inQuotes)
	if len(refs) == 0 {
		return sql
	}
	var b strings.Builder
	last := 0
	for _, r := range refs {
		v, ok := values[r.Name]
		if !ok {
			continue
		}
		b.WriteString(sql[last:r.Start])
		b.WriteString(v)
		last = r.End
	}
	b.WriteString(sql[last:])
	return b.String()
}

// ResultName 读出语句里 “-- name: 标题” 注释给结果标签起的名字。
func ResultName(stmt string) string {
	for _, line := range strings.Split(stmt, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "--") {
			if t != "" {
				return ""
			}
			continue
		}
		t = strings.TrimSpace(strings.TrimPrefix(t, "--"))
		if strings.HasPrefix(strings.ToLower(t), "name:") {
			return strings.TrimSpace(t[len("name:"):])
		}
	}
	return ""
}

// Uglify 把 SQL 压成一行：去掉注释，连续空白合成一个空格，字符串和引号标识符原样保留。
func Uglify(sql string) string {
	var b strings.Builder
	n := len(sql)
	space := false
	emit := func(s string) {
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteString(s)
	}
	for i := 0; i < n; {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			space = true
			i++
		case c == '-' && i+1 < n && sql[i+1] == '-', c == '#':
			if j := strings.IndexByte(sql[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = n
			}
			space = true
		case c == '/' && i+1 < n && sql[i+1] == '*':
			if j := strings.Index(sql[i+2:], "*/"); j >= 0 {
				i += 2 + j + 2
			} else {
				i = n
			}
			space = true
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < n {
				if c != '`' && sql[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if sql[j] == c {
					if j+1 < n && sql[j+1] == c { // 双写的引号
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			j = min(j, n)
			emit(sql[i:j])
			i = j
		default:
			j := i
			for j < n && !strings.ContainsRune(" \t\r\n'\"`#", rune(sql[j])) && !(sql[j] == '-' && j+1 < n && sql[j+1] == '-') && !(sql[j] == '/' && j+1 < n && sql[j+1] == '*') {
				j++
			}
			if j == i { // 保证前进
				j = i + 1
			}
			emit(sql[i:j])
			i = j
		}
	}
	return strings.TrimSpace(b.String())
}
