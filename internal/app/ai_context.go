package app

import (
	"strings"
	"unicode/utf8"

	"minidb/internal/ai"
)

// 库结构总览的规模分档与对话历史的预算。
const (
	overviewColumnsMaxTables = 40  // 表数不超过它时，总览里带上每张表的列
	overviewColumnsPerTable  = 30  // 每表最多列出多少列
	overviewTablesMax        = 300 // 表很多时，总览最多列出多少张表
	historyMaxMessages       = 12  // 带给模型的最近消息条数
	historyMaxChars          = 36000
	historyOldAnswerChars    = 3000 // 较早的助手回答最多保留多少字符
)

// buildSchemaOverview 构建放进系统提示词的库结构总览：
// 小库每表一行带列名与类型，表多时只列表名与注释，其余靠 table_list / schema_search / table_describe 按需查。
// 输出只取决于 schema 本身，所以同一个库每一轮的提示词前缀完全相同，能命中提示词缓存；
// 之前"首轮发完整 DDL、之后发摘要"的做法会让前缀每轮变化，而且模型后续轮次看不到列信息。
func buildSchemaOverview(schema *ai.SchemaContext) string {
	if schema == nil || len(schema.Tables) == 0 {
		return ""
	}
	if len(schema.Tables) <= overviewColumnsMaxTables {
		return ai.BuildCompactSchema(schema.Tables, overviewColumnsPerTable)
	}
	return ai.BuildTableList(schema.Tables, overviewTablesMax)
}

// trimHistory 裁剪带给模型的会话历史：只留最近几条，总字符数有预算，
// 较早的长回答截短；最后一条（用户的新问题）一定保留。
func trimHistory(messages []ai.ChatMessage) []ai.ChatMessage {
	if len(messages) > historyMaxMessages {
		messages = messages[len(messages)-historyMaxMessages:]
	}
	kept := make([]ai.ChatMessage, 0, len(messages))
	budget := historyMaxChars
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if i < len(messages)-1 && m.Role != "user" {
			m.Content = truncateRunes(m.Content, historyOldAnswerChars)
		}
		size := utf8.RuneCountInString(m.Content)
		if size > budget {
			if i == len(messages)-1 {
				m.Content = truncateRunes(m.Content, budget)
				kept = append(kept, m)
			}
			break // 更早的消息不再带
		}
		budget -= size
		kept = append(kept, m)
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// truncateRunes 按字符数截断，超出部分用省略号标出。
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n])) + "…（已截断）"
}
