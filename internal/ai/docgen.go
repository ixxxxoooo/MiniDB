package ai

import (
	"context"
	"fmt"
	"strings"
)

const docgenSystemPrompt = `你是一个技术文档写作专家。根据提供的表结构信息，生成专业的 Markdown 格式表文档。

文档应包含：
1. 表概述（表的用途和功能说明）
2. 字段说明（每个字段的含义、类型、约束等）
3. 使用示例（常见的查询示例）
4. 注意事项（使用该表时需要注意的点）

使用清晰的中文 Markdown 格式。`

// GenerateTableDoc 自动生成表文档
func (c *Client) GenerateTableDoc(ctx context.Context, schema *SchemaContext, tableName string) (string, error) {
	var tableSchema *TableSchema
	for _, t := range schema.Tables {
		if t.Name == tableName {
			tableSchema = &t
			break
		}
	}

	if tableSchema == nil {
		return "", fmt.Errorf("表 %s 不存在", tableName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("表名: %s\n数据库类型: %s\n\n字段列表:\n", tableName, schema.DatabaseType))
	for _, c := range tableSchema.Columns {
		nullable := "NOT NULL"
		if c.Nullable {
			nullable = "可为空"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s, %s) %s\n", c.Name, c.Type, nullable, c.Comment))
	}

	return c.Chat(ctx, docgenSystemPrompt, sb.String())
}
