package ai

import (
	"context"
	"fmt"
)

const diagnoseSystemPrompt = `你是一个数据库错误诊断专家。分析 SQL 执行错误，提供以下内容：

1. 错误类型分类
2. 错误原因分析
3. 修复建议
4. 修复后的 SQL（如果适用）

请用清晰的中文 Markdown 格式返回。如果能给出修复后的 SQL，请用代码块包裹。`

// DiagnoseError SQL 错误诊断
func (c *Client) DiagnoseError(ctx context.Context, sqlStr, errorMsg string) (string, error) {
	userMsg := fmt.Sprintf("SQL 语句:\n```sql\n%s\n```\n\n错误信息:\n%s", sqlStr, errorMsg)
	return c.Chat(ctx, diagnoseSystemPrompt, userMsg)
}
