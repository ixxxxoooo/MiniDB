package app

import (
	"context"
	"errors"
	"minidb/internal/database"
	"strings"

	"minidb/internal/ai"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
)

// InlineSQLRequest 编辑器里"让 AI 写 / 改 SQL"的一次请求。
type InlineSQLRequest struct {
	Instruction string   // 用户用自然语言写的要求，可以是中文
	SQL         string   // 要改写的语句；为空表示生成新语句
	Error       string   // 这条语句上次运行的报错（修复场景），可为空
	Tables      []string // 用户明确点名的表（@表名），一定会带上完整结构
}

// inlineDetailTables 一次最多把几张表的完整结构交给模型。
const inlineDetailTables = 6

// inlineSystemPrompt 编辑器内联生成的行为准则：只要 SQL，不要对话。
const inlineSystemPrompt = `你是 MiniDB 的 SQL 编辑器里的内联助手，把用户的自然语言（通常是中文）变成可以直接放进编辑器的 SQL。

## 输出
- 只输出一个 ` + "```sql" + ` 代码块，代码块之外不要写任何文字。
- 需要说明口径或假设时，用 SQL 注释（-- 开头，中文）写在语句前面，保持简短。
- 改写已有语句时输出完整的新语句，不要只给片段；没被要求改的部分保持原样。

## 准则
- 只使用下方库结构里真实存在的表和列，不要编造；点名了表时优先使用它们。
- 需求含糊时按最常见的理解来写，并把假设写进注释，而不是提问。
- SQL 要兼容下方当前数据库的类型与版本。
- 你只负责生成，不会执行；修改数据的语句也只是生成，由用户审阅后自己运行。`

// buildInlinePrompt 组装内联生成的系统提示词。detail 是相关表的完整结构。
func buildInlinePrompt(info promptInfo, detail string) string {
	var sb strings.Builder
	sb.WriteString(inlineSystemPrompt)
	if info.dbType != "" {
		sb.WriteString("\n\n## 当前数据库\n- 库名: " + info.dbName + "\n- 类型: " + string(info.dbType))
		if info.dbVersion != "" {
			sb.WriteString("（版本 " + info.dbVersion + "）")
		}
		if note := database.PromptHintOf(info.dbType); note != "" {
			sb.WriteString("\n- " + note)
		}
	}
	if info.overview != "" {
		sb.WriteString("\n\n## 库结构总览\n" + info.overview)
	}
	if detail != "" {
		sb.WriteString("\n\n## 相关表的完整结构\n" + detail)
	}
	if info.overview == "" && detail == "" {
		sb.WriteString("\n\n（没有读到当前库的结构，只能按用户的描述写；用到的表名和列名请在注释里提醒用户核对。）")
	}
	return sb.String()
}

// buildInlineMessage 用户消息：要求在前，要改的语句和报错跟在后面。
func buildInlineMessage(req InlineSQLRequest) string {
	var sb strings.Builder
	instruction := strings.TrimSpace(req.Instruction)
	switch {
	case req.SQL != "":
		if instruction == "" {
			instruction = "优化这条语句"
		}
		sb.WriteString("要求：" + instruction + "\n\n当前语句：\n```sql\n" + req.SQL + "\n```")
	default:
		sb.WriteString("要求：" + instruction)
	}
	if req.Error != "" {
		sb.WriteString("\n\n这条语句运行时的报错：\n" + truncateRunes(req.Error, 800))
	}
	return sb.String()
}

// relevantTables 挑出要带完整结构的表：用户点名的 + 语句里出现的 + 按要求文字检索到的，最多 limit 张。
func relevantTables(schema *ai.SchemaContext, search *schemaindex.Searcher, req InlineSQLRequest, limit int) []ai.TableSchema {
	if schema == nil {
		return nil
	}
	var out []ai.TableSchema
	seen := map[string]bool{}
	add := func(t ai.TableSchema) {
		key := strings.ToLower(t.Name)
		if !seen[key] && len(out) < limit {
			seen[key] = true
			out = append(out, t)
		}
	}
	byName := func(name string) (ai.TableSchema, bool) {
		for _, t := range schema.Tables {
			if strings.EqualFold(t.Name, name) {
				return t, true
			}
		}
		return ai.TableSchema{}, false
	}
	for _, name := range req.Tables {
		if t, ok := byName(name); ok {
			add(t)
		}
	}
	lower := strings.ToLower(req.SQL)
	if lower != "" {
		for _, t := range schema.Tables {
			if strings.Contains(lower, strings.ToLower(t.Name)) {
				add(t)
			}
		}
	}
	if search != nil {
		for _, hit := range search.Tables([]string{req.Instruction}, limit) {
			if t, ok := byName(hit.Name); ok {
				add(t)
			}
		}
	}
	return out
}

// GenerateSQL 编辑器内联生成 / 改写 SQL：不带工具、流式输出，只返回一个 SQL。
//
// requestID 用于取消（CancelChatStream）；onDelta 收到模型原始输出的增量（可能带代码围栏，用 ExtractSQL 取出 SQL），可为空。
func (s *AIService) GenerateSQL(connID, dbName string, req InlineSQLRequest, requestID string, onDelta func(string)) (string, error) {
	defer logger.Perf("app.AI.GenerateSQL", "conn", connID, "db", dbName)()
	if strings.TrimSpace(req.Instruction) == "" && strings.TrimSpace(req.SQL) == "" {
		return "", errors.New("请先描述想要什么")
	}
	s.ReloadConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer s.registerCancel(requestID, cancel)()

	info := promptInfo{dbName: dbName}
	detail := ""
	if schema, search, err := s.schema.Index(ctx, connID, dbName); err != nil {
		logger.Warn("[AIService] 内联生成：加载库结构失败: %v", err)
	} else {
		info.dbType, info.dbVersion, info.overview = database.DBType(schema.DatabaseType), schema.DatabaseVersion, buildSchemaOverview(schema)
		detail = ai.BuildTablesDDL(relevantTables(schema, search, req, inlineDetailTables))
	}
	logger.Info("[AIService] 内联生成: connID=%s db=%s requestID=%s rewrite=%v", connID, dbName, requestID, req.SQL != "")

	answer, err := s.client.ChatWithToolsStream(ctx, buildInlinePrompt(info, detail),
		[]ai.ChatMessage{{Role: "user", Content: buildInlineMessage(req)}}, nil, 1, nil, ai.ToolStreamCallbacks{OnDelta: onDelta})
	if ctx.Err() != nil {
		return "", errors.New("已停止")
	}
	if err != nil {
		return "", friendlyAIError(err)
	}
	sql := ExtractSQL(answer)
	if sql == "" {
		return "", errors.New("模型没有给出 SQL，请换一种说法再试")
	}
	return sql, nil
}

// ExtractSQL 从模型输出里取出 SQL：有代码围栏就取围栏里的内容，没有就取全文。
// 输出还在流式增长时（围栏没闭合）也能用，界面每来一段增量都可以调它。
func ExtractSQL(text string) string {
	i := strings.Index(text, "```")
	if i < 0 {
		return strings.TrimSpace(text)
	}
	body := text[i+3:]
	// 围栏后面第一行是语言标记（sql），去掉；还没收到换行说明标记本身还在流入
	nl := strings.IndexByte(body, '\n')
	if nl < 0 {
		return ""
	}
	body = body[nl+1:]
	if j := strings.Index(body, "```"); j >= 0 {
		return strings.TrimSpace(body[:j])
	}
	return strings.TrimSpace(strings.TrimRight(body, "`")) // 结尾的围栏可能只到了一两个反引号
}
