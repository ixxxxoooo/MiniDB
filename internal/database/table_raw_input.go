package database

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	reLeadingBlockComment = regexp.MustCompile(`(?s)^\s*/\*[\s\S]*?\*/\s*`)
	reLeadingLineComment  = regexp.MustCompile(`(?m)^\s*--[^\n]*(?:\n|\z)\s*`)
)

// StripLeadingSQLComments 去除 SQL 前导块注释与行注释（与前端原逻辑对齐）
func StripLeadingSQLComments(sql string) string {
	text := strings.TrimSpace(sql)
	for text != "" {
		prev := text
		text = reLeadingBlockComment.ReplaceAllString(text, "")
		text = reLeadingLineComment.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == prev {
			break
		}
	}
	return text
}

// SQLLeadingVerb 返回清理注释后首个 SQL 关键字（小写），供安全策略等使用
func SQLLeadingVerb(sql string) string {
	cleaned := StripLeadingSQLComments(sql)
	if cleaned == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range cleaned {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + ('a' - 'A'))
		} else if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			break
		} else {
			break
		}
	}
	return b.String()
}

// BuildTableDataQuerySQL 根据原始输入构造单表查询 SQL（完整语句则原样返回；否则视为 WHERE 片段）
// serverVersion 来自 GetServerVersion，供后续按版本限制语法；当前用于日志与方言识别一致性
func BuildTableDataQuerySQL(dbType, dbName, table, rawInput string, page, pageSize int, serverVersion string) (string, error) {
	_ = serverVersion // 预留：如旧版 MySQL 对 ONLY_FULL_GROUP_BY 等与 UI 拼接相关的差异
	in := strings.TrimSpace(rawInput)
	if in == "" {
		return "", nil
	}
	if IsLikelyFullStatementForDialect(dbType, in) {
		if !isSingleSQLStatement(in) {
			return "", fmt.Errorf("表格 Raw SQL 仅允许单条只读查询")
		}
		check := CheckAutoExecutableSelectSQL(in)
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
	return fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT %d OFFSET %d", qt, in, pageSize, offset), nil
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
