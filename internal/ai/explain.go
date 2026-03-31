package ai

import (
	"context"
)

// SQLExplanation SQL 解释结果
type SQLExplanation struct {
	Summary       string   `json:"summary"`
	Steps         []string `json:"steps"`
	Optimizations []string `json:"optimizations"`
}

const explainSystemPrompt = `你是一个资深的数据库优化专家。你的任务是解释 SQL 语句的执行逻辑，并给出优化建议。

请按以下格式返回：
## 概述
[一句话概括这条 SQL 的作用]

## 执行步骤
1. [步骤1]
2. [步骤2]
...

## 优化建议
- [建议1]
- [建议2]
...

如果 SQL 已经是最优的，在优化建议中说明即可。`

// ExplainSQL 解释 SQL
func (c *Client) ExplainSQL(ctx context.Context, sqlStr string) (string, error) {
	return c.Chat(ctx, explainSystemPrompt, sqlStr)
}
