package services

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
)

const maxChatAutoFixRetries = 3

// repairSQLSystemPrompt 自动修复轮次使用的系统提示（中英文说明）
var repairSQLSystemPrompt = strings.TrimSpace(`
你是数据库 SQL 修复助手 / You are a database SQL repair assistant.
用户提供的 SQL 执行失败，请分析原因并给出修复后的完整 SQL，用 Markdown 的 ` + "```sql" + ` 代码块包裹。
回复开头必须额外输出一个 ` + "```tableplus-ai-meta" + ` 结构化元数据块，声明是否继续自动执行：
` + "```tableplus-ai-meta" + `
{"autoExecute":{"enabled":true,"mode":"first_sql_readonly","reason":"retry_after_fix"}}
` + "```" + `
仅输出必要说明与可执行 SQL。
`)

// ChatAutoExecuteResult 自动执行编排结果（供 Wails 绑定与前端消费，字段名保持稳定）
type ChatAutoExecuteResult struct {
	MergedContent string `json:"mergedContent"`
	Ran           bool   `json:"ran"`
	SkipReason    string `json:"skipReason,omitempty"`
	SkippedUnsafe bool   `json:"skippedUnsafe,omitempty"`
	ReasonCode    string `json:"reasonCode,omitempty"`
	Verb          string `json:"verb,omitempty"`
}

// emitRepairDelta 将修复阶段内容以流式事件推给前端，与主会话共用 requestId，AIPanel 会继续往当前助手气泡追加
func (s *AIService) emitRepairDelta(requestID, chunk string) {
	if chunk == "" {
		return
	}
	s.emitStreamEvent(ChatStreamEvent{
		RequestID: requestID,
		Type:      "delta",
		Delta:     chunk,
	})
}

// RunChatAutoExecute 在后端完成自动执行意图的安全校验、执行与失败后的 AI 修复重试，返回合并后的助手展示 Markdown
func (s *AIService) RunChatAutoExecute(connID, dbName string, autoExecute ChatAutoExecuteDirective, assistantContent string, conversationMessages []ai.ChatMessage, requestID string) (*ChatAutoExecuteResult, error) {
	s.ReloadConfig()
	if s.query == nil {
		logger.Warn("[AIService] RunChatAutoExecute: QueryService 未注入，跳过自动执行")
		return &ChatAutoExecuteResult{
			MergedContent: assistantContent,
			Ran:           false,
			SkipReason:    "query_service_nil",
		}, nil
	}
	if !autoExecute.Enabled {
		return &ChatAutoExecuteResult{MergedContent: assistantContent, Ran: false}, nil
	}
	sqlStr, ok := database.ExtractFirstSQLFenceFromMarkdown(assistantContent)
	if !ok || strings.TrimSpace(sqlStr) == "" {
		logger.Info("[AIService] RunChatAutoExecute: 未找到 sql 代码块，跳过执行")
		return &ChatAutoExecuteResult{MergedContent: assistantContent, Ran: false, SkipReason: "missing_sql_fence"}, nil
	}

	check := database.CheckAutoExecutableSelectSQL(sqlStr)
	if !check.Allowed {
		logger.Info("[AIService] RunChatAutoExecute: 安全校验未通过 reason=%s verb=%s", check.ReasonCode, check.Verb)
		suffix := formatAutoExecuteSkippedSuffix(check)
		return &ChatAutoExecuteResult{
			MergedContent: assistantContent + suffix,
			Ran:           false,
			SkippedUnsafe: true,
			ReasonCode:    check.ReasonCode,
			Verb:          check.Verb,
		}, nil
	}

	logger.Info("[AIService] RunChatAutoExecute: 开始执行 SQL requestID=%s sql_len=%d", requestID, len(sqlStr))
	s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "executing_sql"})

	merged := assistantContent
	currentSQL := sqlStr
	ctxBase := slices.Clone(conversationMessages)

	for attempt := 0; attempt <= maxChatAutoFixRetries; attempt++ {
		result, err := s.query.ExecuteSQLPaged(connID, dbName, currentSQL, 1, database.DefaultPageSize)
		if err != nil {
			logger.Error("[AIService] RunChatAutoExecute: 执行异常 attempt=%d err=%v", attempt, err)
			chunk := fmt.Sprintf("\n\n**执行异常 / Execute error**: `%s`", err.Error())
			merged += chunk
			s.emitRepairDelta(requestID, chunk)
			break
		}
		if result != nil && result.Error == "" {
			okChunk := formatQuerySuccessMarkdownSuffix(result, attempt > 0)
			merged += okChunk
			s.emitRepairDelta(requestID, okChunk)
			logger.Info("[AIService] RunChatAutoExecute: 执行成功 attempt=%d rows=%d", attempt, len(result.Rows))
			break
		}
		errMsg := ""
		if result != nil {
			errMsg = result.Error
		}
		if attempt >= maxChatAutoFixRetries {
			failChunk := fmt.Sprintf("\n\n---\n\n**❌ 自动修复失败 / Auto-fix failed**（已达最大重试 / max retries）\n\n`%s`", errMsg)
			merged += failChunk
			s.emitRepairDelta(requestID, failChunk)
			logger.Warn("[AIService] RunChatAutoExecute: 已达最大修复次数")
			break
		}

		s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "auto_fixing"})
		errBanner := fmt.Sprintf("\n\n---\n\n**⚠️ SQL 执行错误 / SQL error**\n\n`%s`\n\n**🔧 自动修复 / Auto-fix**（第 %d / %d 次）\n\n",
			errMsg, attempt+1, maxChatAutoFixRetries)
		merged += errBanner
		s.emitRepairDelta(requestID, errBanner)

		fixMsgs := buildSQLRepairMessages(ctxBase, merged, currentSQL, errMsg)
		// 修复阶段走流式：delta 经 Wails 事件推到前端，与主回答同一 requestId 续写气泡
		s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "calling_ai"})
		fixResp, ferr := s.client.ChatWithMessagesStream(
			context.Background(),
			repairSQLSystemPrompt,
			fixMsgs,
			func(delta string) {
				s.emitRepairDelta(requestID, delta)
			},
		)
		if ferr != nil {
			logger.Error("[AIService] RunChatAutoExecute: 修复流式请求失败: %v", ferr)
			errChunk := fmt.Sprintf("\n\n**修复请求失败 / Fix request failed**: %s", ferr.Error())
			merged += errChunk
			s.emitRepairDelta(requestID, errChunk)
			break
		}
		meta, fixClean, _ := database.ExtractAutoExecuteMetaBlock(strings.TrimSpace(fixResp))
		_ = meta // 修复轮当前仍以提取首个 SQL 为准继续执行，meta 仅用于兼容后续扩展
		// 流式过程中前端已通过 delta 累积内容；merged 需与完整响应对齐供最终 mergedContent 一致
		merged += "\n\n" + fixClean

		if nextSQL, ok2 := database.ExtractFirstSQLFenceFromMarkdown(fixClean); ok2 && strings.TrimSpace(nextSQL) != "" {
			currentSQL = strings.TrimSpace(nextSQL)
			continue
		}
		logger.Info("[AIService] RunChatAutoExecute: 修复响应中无有效 SQL，结束重试")
		break
	}

	return &ChatAutoExecuteResult{
		MergedContent: merged,
		Ran:           true,
	}, nil
}

func buildSQLRepairMessages(base []ai.ChatMessage, assistantSoFar, failedSQL, errMsg string) []ai.ChatMessage {
	out := make([]ai.ChatMessage, 0, len(base)+2)
	out = append(out, base...)
	out = append(out, ai.ChatMessage{Role: "assistant", Content: assistantSoFar})
	feedback := fmt.Sprintf("[SQL_ERROR] 执行以下 SQL 时报错 / SQL failed:\n```sql\n%s\n```\n错误信息 / Error: %s\n\n请分析并生成修复后的 SQL。", failedSQL, errMsg)
	out = append(out, ai.ChatMessage{Role: "user", Content: feedback})
	return out
}

func formatAutoExecuteSkippedSuffix(c database.AutoExecutableCheckResult) string {
	var reasonCN, reasonEN string
	switch c.ReasonCode {
	case "empty_sql":
		reasonCN, reasonEN = "SQL 为空", "SQL is empty"
	case "risky_sql":
		reasonCN = fmt.Sprintf("语句类型 %s 不允许自动执行（仅允许 SELECT/SHOW/DESC/DESCRIBE/EXPLAIN/WITH）", c.Verb)
		reasonEN = fmt.Sprintf("Statement type %s is not allowed (only read-only statements)", c.Verb)
	default:
		reasonCN = fmt.Sprintf("无法识别的语句类型 %s", c.Verb)
		reasonEN = fmt.Sprintf("Unrecognized statement type %s", c.Verb)
	}
	return fmt.Sprintf("\n\n---\n\n**⚠️ 已跳过自动执行 / Auto-run skipped**\n\n**原因 / Reason**\n- %s\n- %s\n", reasonCN, reasonEN)
}

func formatQuerySuccessMarkdownSuffix(result *database.QueryResult, afterFix bool) string {
	if result == nil {
		return "\n\n**执行成功 / OK**"
	}
	prefix := "\n\n"
	if afterFix {
		prefix = "\n\n---\n\n**✅ SQL 已自动修复并执行成功 / SQL auto-fixed and executed**\n\n"
	}
	if len(result.Rows) > 0 {
		var cols []string
		for _, c := range result.Columns {
			cols = append(cols, c.Name)
		}
		if len(cols) == 0 && len(result.Rows) > 0 {
			for k := range result.Rows[0] {
				cols = append(cols, k)
			}
			slices.Sort(cols)
		}
		var header, sep strings.Builder
		header.WriteString("| ")
		sep.WriteString("| ")
		for i, c := range cols {
			if i > 0 {
				header.WriteString(" | ")
				sep.WriteString(" | ")
			}
			header.WriteString(c)
			sep.WriteString("---")
		}
		header.WriteString(" |")
		sep.WriteString(" |")
		maxRows := 50
		var rowLines []string
		for i, row := range result.Rows {
			if i >= maxRows {
				break
			}
			var cells []string
			for _, c := range cols {
				v := row[c]
				s := "NULL"
				if v != nil {
					s = fmt.Sprintf("%v", v)
					if len(s) > 80 {
						s = s[:80]
					}
				}
				cells = append(cells, s)
			}
			rowLines = append(rowLines, "| "+strings.Join(cells, " | ")+" |")
		}
		body := strings.Join(rowLines, "\n")
		suffix := ""
		if len(result.Rows) > maxRows {
			suffix = "\n| ... |"
		}
		return fmt.Sprintf("%s**查询结果 / Query result** (%d rows, %dms):\n\n%s\n%s\n%s",
			prefix, result.Total, result.Duration, header.String(), sep.String(), body+suffix)
	}
	return fmt.Sprintf("%s**执行成功 / Execution successful**, %d rows (%dms)", prefix, result.Total, result.Duration)
}
