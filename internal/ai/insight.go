package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

const insightSystemPrompt = `你是一个数据分析专家。分析给定的查询结果数据，提供以下内容：

1. 数据摘要：简要概括数据的整体特征
2. 异常检测：发现数据中的异常值或异常模式
3. 趋势分析：识别数据中的趋势和规律

请用清晰的中文 Markdown 格式返回分析结果。`

// AnalyzeData 数据洞察分析
func (c *Client) AnalyzeData(ctx context.Context, columns []string, rows []map[string]interface{}, question string) (string, error) {
	// 将数据序列化（截取前100行避免超长）
	maxRows := len(rows)
	if maxRows > 100 {
		maxRows = 100
	}

	dataJSON, _ := json.MarshalIndent(map[string]interface{}{
		"columns":    columns,
		"rows":       rows[:maxRows],
		"total_rows": len(rows),
	}, "", "  ")

	userMsg := fmt.Sprintf("查询结果数据（共 %d 行，展示前 %d 行）:\n\n%s",
		len(rows), maxRows, string(dataJSON))

	if question != "" {
		userMsg += fmt.Sprintf("\n\n用户问题: %s", question)
	}

	return c.Chat(ctx, insightSystemPrompt, userMsg)
}
