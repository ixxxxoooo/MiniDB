package ui

import (
	"strconv"
	"strings"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 筛选浮层里「WHERE 预览」的文字：按当前编辑中的各行拼出一条 WHERE 子句（纯逻辑，可单独测试）。

// whereText 把 f.filters 里填了值的各行拼成 "WHERE a > 30 AND b = 'x'"，没有可用的条件时返回空串。
// ident / lit 分别给标识符和字符串字面量加引号（各数据库写法不同，由调用方传入）；数字原样输出。
func (f *filterState) whereText(ident, lit func(string) string) string {
	var sb strings.Builder
	for _, r := range f.filters {
		var part string
		switch {
		case r.col == locales.DatagridRawSQL():
			part = strings.TrimSpace(r.val)
			if part != "" {
				part = "(" + part + ")"
			}
		case r.op == "IS NULL" || r.op == "IS NOT NULL":
			part = filterCol(r.col, ident) + " " + r.op
		case strings.TrimSpace(r.val) != "":
			part = filterCol(r.col, ident) + " " + r.op + " " + filterValue(r.op, r.val, lit)
		}
		if part == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString(map[bool]string{true: " OR ", false: " AND "}[f.filterOr])
		}
		sb.WriteString(part)
	}
	if sb.Len() == 0 {
		return ""
	}
	return "WHERE " + sb.String()
}

// filterCol 是条件左边的列：「任意列」写成 *，其余加标识符引号（纯字母数字下划线的不加，预览更清爽）。
func filterCol(col string, ident func(string) string) string {
	if col == locales.TableFilterAny() {
		return "*"
	}
	for _, r := range col {
		if r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return ident(col)
		}
	}
	return col
}

// filterValue 是条件右边的值：数字原样；IN 的括号列表原样；其余按字符串加引号。
func filterValue(op, v string, lit func(string) string) string {
	v = strings.TrimSpace(v)
	if op == "IN" {
		if strings.HasPrefix(v, "(") {
			return v
		}
		return "(" + v + ")"
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil && op != "LIKE" && op != "NOT LIKE" {
		return v
	}
	return lit(v)
}
