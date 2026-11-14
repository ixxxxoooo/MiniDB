package database

import (
	"fmt"
	"strings"
)

func buildWhereClause(dbType DBType, filters []Filter, tableColumns []string) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var conditions []string
	var joins []string // joins[i] 连接 conditions[i-1] 与 conditions[i]
	var args []interface{}
	for _, f := range filters {
		join := "AND"
		if f.Or {
			join = "OR"
		}
		joins = append(joins, join)
		if f.Column == "" {
			return "", nil, fmt.Errorf("不支持的筛选列: %s", f.Column)
		}
		if f.Column == "__any" {
			cond, anyArgs, err := buildAnyColumnCondition(dbType, f, tableColumns)
			if err != nil {
				return "", nil, err
			}
			conditions = append(conditions, cond)
			args = append(args, anyArgs...)
			continue
		}
		op := strings.ToUpper(strings.TrimSpace(f.Operator))
		col := QuoteIdent(dbType, f.Column)
		switch op {
		case "IS NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NULL", col))
		case "IS NOT NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", col))
		case "=", "!=", "<>", ">", "<", ">=", "<=", "LIKE", "NOT LIKE":
			placeholder := "?"
			if dbType == Postgres {
				placeholder = fmt.Sprintf("$%d", len(args)+1)
			}
			conditions = append(conditions, fmt.Sprintf("%s %s %s", col, op, placeholder))
			args = append(args, f.Value)
		case "IN":
			values := splitFilterListValue(f.Value)
			if len(values) == 0 {
				return "", nil, fmt.Errorf("IN 筛选值不能为空")
			}
			placeholders := make([]string, 0, len(values))
			for _, value := range values {
				if dbType == Postgres {
					placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
				} else {
					placeholders = append(placeholders, "?")
				}
				args = append(args, value)
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")))
		default:
			return "", nil, fmt.Errorf("不支持的筛选操作符: %s", f.Operator)
		}
	}
	var b strings.Builder
	b.WriteString("WHERE ")
	for i, cond := range conditions {
		if i > 0 {
			b.WriteString(" " + joins[i] + " ")
		}
		b.WriteString(cond)
	}
	return b.String(), args, nil
}

func buildOrderByClause(dbType DBType, sorts []Sort) (string, error) {
	if len(sorts) == 0 {
		return "", nil
	}
	var parts []string
	for _, s := range sorts {
		if s.Column == "" || s.Column == "__any" {
			return "", fmt.Errorf("不支持的排序列: %s", s.Column)
		}
		dir := strings.ToUpper(strings.TrimSpace(s.Direction))
		if dir != "ASC" && dir != "DESC" {
			return "", fmt.Errorf("不支持的排序方向: %s", s.Direction)
		}
		parts = append(parts, fmt.Sprintf("%s %s", QuoteIdent(dbType, s.Column), dir))
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}

func quoteTable(dbType DBType, dbName, table string) string {
	if IsMySQLCompatible(dbType) {
		if dbName != "" {
			return fmt.Sprintf("%s.%s", QuoteIdent(dbType, dbName), QuoteIdent(dbType, table))
		}
		return QuoteIdent(dbType, table)
	}
	// 其余方言一律走 QuoteIdent：不再有"不加引号原样返回"的静默分支
	return QuoteIdent(dbType, table)
}

// QuoteTableName 对表名进行引用，不含库名前缀（供外部调用）
func QuoteTableName(dbType DBType, table string) string {
	return QuoteIdent(dbType, table)
}

func splitFilterListValue(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// hasAnyColumnFilter 判断筛选条件是否包含任意列搜索
func hasAnyColumnFilter(filters []Filter) bool {
	for _, f := range filters {
		if f.Column == "__any" {
			return true
		}
	}
	return false
}

// castColumnAsText 按方言将列转为文本以便跨类型模糊匹配
func castColumnAsText(dbType DBType, column string) string {
	quoted := QuoteIdent(dbType, column)
	switch dbType {
	case Postgres, SQLite:
		return fmt.Sprintf("CAST(%s AS TEXT)", quoted)
	default:
		return fmt.Sprintf("CAST(%s AS CHAR)", quoted)
	}
}

// buildAnyColumnCondition 将 __any 展开为跨列 OR 的 LIKE 条件
func buildAnyColumnCondition(dbType DBType, filter Filter, tableColumns []string) (string, []interface{}, error) {
	value := strings.TrimSpace(filter.Value)
	if value == "" {
		return "", nil, fmt.Errorf("任意列搜索关键字不能为空")
	}
	if len(tableColumns) == 0 {
		return "", nil, fmt.Errorf("表无可用列，无法执行任意列搜索")
	}
	likeValue := "%" + value + "%"
	var parts []string
	var args []interface{}
	for _, col := range tableColumns {
		placeholder := "?"
		if dbType == Postgres {
			placeholder = fmt.Sprintf("$%d", len(args)+1)
		}
		parts = append(parts, fmt.Sprintf("%s LIKE %s", castColumnAsText(dbType, col), placeholder))
		args = append(args, likeValue)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}
