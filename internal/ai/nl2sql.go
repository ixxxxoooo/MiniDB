package ai

import (
	"context"
	"fmt"
	"strings"
)

// NL2SQLResult 自然语言转 SQL 结果
type NL2SQLResult struct {
	SQL         string  `json:"sql"`
	Explanation string  `json:"explanation"`
	Confidence  float64 `json:"confidence"`
}

// SchemaContext 表结构上下文
type SchemaContext struct {
	DatabaseType string
	DatabaseName string
	Tables       []TableSchema
}

// TableSchema 表结构
type TableSchema struct {
	Name    string
	Columns []ColumnSchema
}

// ColumnSchema 列结构
type ColumnSchema struct {
	Name     string
	Type     string
	Nullable bool
	Comment  string
}

const nl2sqlSystemPrompt = `你是一个专业的数据库 SQL 专家。你的任务是将用户的自然语言描述转换为准确的 SQL 查询语句。

规则：
1. 根据提供的表结构信息生成准确的 SQL
2. 使用标准 SQL 语法，兼容目标数据库类型
3. 确保生成的 SQL 安全，避免 SQL 注入
4. 返回格式：先写 SQL 语句，然后用 --- 分隔，再写简短解释

示例返回格式：
SELECT * FROM users WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY) ORDER BY created_at DESC;
---
该查询从 users 表中获取最近 7 天创建的用户记录，按创建时间倒序排列。`

// NaturalLanguageToSQL 自然语言转 SQL
func (c *Client) NaturalLanguageToSQL(ctx context.Context, schema *SchemaContext, prompt string) (*NL2SQLResult, error) {
	// 构建上下文
	schemaStr := buildSchemaContext(schema)
	userMsg := fmt.Sprintf("数据库类型: %s\n数据库名: %s\n\n表结构信息:\n%s\n\n用户需求: %s",
		schema.DatabaseType, schema.DatabaseName, schemaStr, prompt)

	resp, err := c.Chat(ctx, nl2sqlSystemPrompt, userMsg)
	if err != nil {
		return nil, err
	}

	// 解析返回结果
	parts := strings.SplitN(resp, "---", 2)
	result := &NL2SQLResult{
		SQL:        strings.TrimSpace(parts[0]),
		Confidence: 0.85,
	}
	if len(parts) > 1 {
		result.Explanation = strings.TrimSpace(parts[1])
	}

	return result, nil
}

func buildSchemaContext(schema *SchemaContext) string {
	var sb strings.Builder
	for _, t := range schema.Tables {
		sb.WriteString(fmt.Sprintf("表 %s:\n", t.Name))
		for _, c := range t.Columns {
			nullable := "NOT NULL"
			if c.Nullable {
				nullable = "NULL"
			}
			comment := ""
			if c.Comment != "" {
				comment = fmt.Sprintf(" -- %s", c.Comment)
			}
			sb.WriteString(fmt.Sprintf("  - %s %s %s%s\n", c.Name, c.Type, nullable, comment))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
