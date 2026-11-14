package database

import (
	"fmt"
	"strings"
)

// hasLimitClause 检测 SQL 最外层是否已经带有 LIMIT 子句。
// 先剥掉注释并抹平字符串/引用标识符，再只看括号深度为 0 的 LIMIT 关键字：
// 子查询里的 LIMIT（如 WHERE id IN (SELECT ... LIMIT 1)）、字符串里的 'limit'、
// 反引号列名 `limit` 都不应该让外层查询失去自动分页。
func hasLimitClause(sql string) bool {
	skeleton, _ := sqlSkeleton(sql, readOnlyLexVariants[0])
	depth := 0
	n := len(skeleton)
	for i := 0; i < n; i++ {
		switch skeleton[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case 'l', 'L':
			if depth != 0 || i+5 > n || !strings.EqualFold(skeleton[i:i+5], "limit") {
				continue
			}
			if i > 0 && isIdentByte(skeleton[i-1]) {
				continue
			}
			if i+5 < n && isIdentByte(skeleton[i+5]) {
				continue
			}
			return true
		}
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

// buildAutoCountSQL 构造自动分页用的 COUNT 语句。
// 对结构简单的单表查询（无子查询/JOIN/UNION/GROUP BY/DISTINCT/ORDER BY 等）直接生成
// "SELECT COUNT(*) FROM <from 片段>"，避免把整条查询包成子查询让数据库执行两遍；
// 其余情况回退到子查询包装，保持原有正确性。
func buildAutoCountSQL(cleanSQL string) string {
	if from, ok := extractSimpleCountFrom(cleanSQL); ok {
		return "SELECT COUNT(*) FROM " + from
	}
	return fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __auto_count__", cleanSQL)
}

// extractSimpleCountFrom 仅在查询结构足够简单时提取可直接用于 COUNT 的 FROM 片段。
// 保守规则：无括号（子查询/函数调用）、无引号字面量、无 JOIN/UNION/GROUP BY/HAVING/DISTINCT，
// 且只出现一次 FROM；命中任一风险特征即返回 false 走子查询包装。
func extractSimpleCountFrom(cleanSQL string) (string, bool) {
	cleaned := StripLeadingSQLComments(strings.TrimSpace(cleanSQL))
	if cleaned == "" || len(cleaned) < len("SELECT 1 FROM t") {
		return "", false
	}
	upper := strings.ToUpper(cleaned)
	if !strings.HasPrefix(upper, "SELECT ") {
		return "", false
	}
	for _, risk := range []string{"(", ")", "'", `"`, " JOIN ", " UNION ", " GROUP BY ", " HAVING ", " DISTINCT "} {
		if strings.Contains(upper, risk) {
			return "", false
		}
	}
	fromIdx := strings.Index(upper, " FROM ")
	if fromIdx < 0 {
		return "", false
	}
	// 只允许一个 FROM（多表连接已通过 JOIN 关键词排除，这里防御逗号连接等写法）
	if strings.Contains(upper[fromIdx+len(" FROM "):], " FROM ") {
		return "", false
	}
	// 从 FROM 之后截断 ORDER BY / LIMIT / OFFSET（词边界匹配，列名恰好为 order/limit 时不受影响）
	rest := cleaned[fromIdx+len(" FROM "):]
	restUpper := upper[fromIdx+len(" FROM "):]
	for _, kw := range []string{" ORDER BY ", " LIMIT ", " OFFSET "} {
		if k := strings.Index(restUpper, kw); k >= 0 {
			rest = rest[:k]
			restUpper = restUpper[:k]
		}
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false
	}
	return rest, true
}
