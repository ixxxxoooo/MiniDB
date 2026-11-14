package database

import (
	"fmt"
	"minidb/internal/sqltext"
	"strings"
)

// StripLeadingSQLComments 去除 SQL 前导块注释与行注释（实现在 sqltext）。
func StripLeadingSQLComments(sql string) string { return sqltext.StripLeadingComments(sql) }

// SQLLeadingVerb 返回清理注释后首个 SQL 关键字（小写）（实现在 sqltext）。
func SQLLeadingVerb(sql string) string { return sqltext.LeadingVerb(sql) }

// BuildTableDataQuerySQL 根据原始输入构造单表查询 SQL（完整语句则原样返回；否则视为 WHERE 片段）
// serverVersion 来自 GetServerVersion，供后续按版本限制语法；当前用于日志与方言识别一致性
func BuildTableDataQuerySQL(dbType DBType, dbName, table, rawInput string, page, pageSize int, serverVersion string) (string, error) {
	_ = serverVersion // 预留：如旧版 MySQL 对 ONLY_FULL_GROUP_BY 等与 UI 拼接相关的差异
	// 归一化「智能引号」/全角引号为 ASCII 直引号，避免 macOS 智能引号或粘贴导致 SQL 语法错误。
	in := normalizeSqlQuotes(strings.TrimSpace(rawInput))
	if in == "" {
		return "", nil
	}
	if IsLikelyFullStatementForDialect(dbType, in) {
		if !isSingleSQLStatement(in) {
			return "", fmt.Errorf("表格 Raw SQL 仅允许单条只读查询")
		}
		check := CheckReadOnlySQL(in)
		if !check.Allowed {
			return "", fmt.Errorf("表格 Raw SQL 仅允许只读查询，当前语句类型: %s", strings.ToUpper(SQLLeadingVerb(in)))
		}
		return in, nil
	}
	if strings.Contains(in, ";") || strings.Contains(in, "--") || strings.Contains(in, "/*") || strings.Contains(in, "*/") {
		return "", fmt.Errorf("WHERE 片段不能包含分号或 SQL 注释")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	offset := (page - 1) * pageSize
	qt := quoteTable(dbType, dbName, table)
	return AppendPage(dbType, fmt.Sprintf("SELECT * FROM %s WHERE %s", qt, in), pageSize, offset), nil
}

func isSingleSQLStatement(sql string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	trimmed := strings.TrimSpace(sql)
	for i, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inSingle {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble && strings.TrimSpace(trimmed[i+1:]) != "" {
				return false
			}
		}
	}
	return true
}

// normalizeSqlQuotes 把「智能引号」(smart quotes) 与全角引号归一化为 ASCII 直引号。
// macOS 默认开启智能引号，输入 ' 或 " 会被替换成弯引号（‘ ’ “ ”），导致 SQL 语法错误；
// 也处理从文档粘贴进来的全角引号。仅在 Raw SQL 片段入口使用，不影响用户业务数据内容。
var sqlQuoteReplacer = strings.NewReplacer(
	"‘", "'", "’", "'", "‚", "'", "‛", "'",
	"“", `"`, "”", `"`, "„", `"`, "‟", `"`,
	"＇", "'",
	"＂", `"`,
)

func normalizeSqlQuotes(in string) string {
	return sqlQuoteReplacer.Replace(in)
}
