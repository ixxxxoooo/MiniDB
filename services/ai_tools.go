package services

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/logger"
	"time"
)

// AIToolDefinition 定义 AI 可调用工具（可扩展）
type AIToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"readOnly"`
}

type aiToolExecutionResult struct {
	ToolName   string
	ToolSQL    string
	ToolOutput string
	DurationMs int64
	Err        error
}

// ListTools 返回当前 AI 可用工具清单，供前端 @tool 联想使用
func (s *AIService) ListTools() []AIToolDefinition {
	return []AIToolDefinition{
		{
			Name:        "table_fuzzy_match",
			Description: "按关键词模糊匹配当前数据库中的表名",
			ReadOnly:    true,
		},
		{
			Name:        "table_describe",
			Description: "查看指定表的字段和注释信息",
			ReadOnly:    true,
		},
		{
			Name:        "table_ddl",
			Description: "查看指定表的 CREATE TABLE DDL",
			ReadOnly:    true,
		},
		{
			Name:        "table_stats",
			Description: "查看指定表的行数与基础统计",
			ReadOnly:    true,
		},
		{
			Name:        "sql_readonly_execute",
			Description: "执行只读 SQL（SELECT/SHOW/EXPLAIN/DESC）",
			ReadOnly:    true,
		},
	}
}

// runPlannedTools 规则优先的工具路由与执行器（为后续模型兜底预留）
func (s *AIService) runPlannedTools(connID, dbName, userQuestion string, schema *ai.SchemaContext, requestID string, stream bool) string {
	if schema == nil || strings.TrimSpace(userQuestion) == "" {
		return ""
	}

	start := time.Now()
	mentions := parseMentions(userQuestion)
	selectedTools := planToolsByRule(userQuestion, mentions)
	if len(selectedTools) == 0 {
		return ""
	}

	if stream {
		s.emitStreamEvent(ChatStreamEvent{
			RequestID: requestID,
			Type:      "status",
			Delta:     "planning_tools",
		})
	}

	var outputs []string
	var errCount int
	for _, tool := range selectedTools {
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "tool_start",
				ToolName:  tool,
				ToolInput: userQuestion,
			})
		}

		result := s.executeTool(tool, connID, dbName, userQuestion, schema, mentions)
		if result.Err != nil {
			errCount++
			logger.Warn("[AIService][Tools] 工具执行失败: tool=%s err=%v", tool, result.Err)
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:  requestID,
					Type:       "tool_error",
					ToolName:   tool,
					ToolInput:  userQuestion,
					ToolOutput: result.Err.Error(),
					DurationMs: result.DurationMs,
				})
			}
			continue
		}

		if stream && result.ToolSQL != "" {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "tool_sql",
				ToolName:  tool,
				ToolSQL:   result.ToolSQL,
			})
		}
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_result",
				ToolName:   tool,
				ToolOutput: result.ToolOutput,
				DurationMs: result.DurationMs,
			})
		}

		outputs = append(outputs, result.ToolOutput)
	}

	if len(outputs) == 0 {
		return ""
	}

	logger.Info("[AIService][Tools] 工具执行完成: requestID=%s tools=%d errors=%d duration=%dms", requestID, len(selectedTools), errCount, time.Since(start).Milliseconds())
	return "\n\n-- 以下是工具链返回的可审计上下文（非黑盒）：\n" + strings.Join(outputs, "\n\n")
}

func (s *AIService) executeTool(toolName, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string) aiToolExecutionResult {
	begin := time.Now()
	switch toolName {
	case "table_fuzzy_match":
		return s.execToolTableFuzzyMatch(userQuestion, schema, begin)
	case "table_describe":
		return s.execToolTableDescribe(userQuestion, schema, mentions, begin)
	case "table_ddl":
		return s.execToolTableDDL(userQuestion, schema, mentions, begin)
	case "table_stats":
		return s.execToolTableStats(connID, dbName, userQuestion, schema, mentions, begin)
	default:
		return aiToolExecutionResult{
			ToolName:   toolName,
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("未知工具: %s", toolName),
		}
	}
}

func (s *AIService) execToolTableFuzzyMatch(userQuestion string, schema *ai.SchemaContext, begin time.Time) aiToolExecutionResult {
	keywords := extractKeywords(userQuestion)
	if len(keywords) == 0 {
		keywords = []string{strings.ToLower(strings.TrimSpace(userQuestion))}
	}

	type match struct {
		Name    string
		Comment string
		Score   int
	}
	var matches []match
	for _, table := range schema.Tables {
		name := strings.ToLower(table.Name)
		comment := strings.ToLower(table.Comment)
		score := 0
		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(name, kw) {
				score += 3
			}
			if strings.Contains(stripTablePrefix(name), kw) {
				score += 2
			}
			if comment != "" && strings.Contains(comment, kw) {
				score += 2
			}
		}
		if score > 0 {
			matches = append(matches, match{Name: table.Name, Comment: table.Comment, Score: score})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > 20 {
		matches = matches[:20]
	}

	var lines []string
	for _, m := range matches {
		if m.Comment != "" {
			lines = append(lines, fmt.Sprintf("- %s（注释: %s，匹配分=%d）", m.Name, m.Comment, m.Score))
		} else {
			lines = append(lines, fmt.Sprintf("- %s（匹配分=%d）", m.Name, m.Score))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "- 未匹配到明显相关表")
	}

	return aiToolExecutionResult{
		ToolName:   "table_fuzzy_match",
		ToolSQL:    "-- metadata: list tables and comments",
		ToolOutput: "### 工具 table_fuzzy_match 结果\n" + strings.Join(lines, "\n"),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableDescribe(userQuestion string, schema *ai.SchemaContext, mentions []string, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTables(schema, userQuestion, mentions, 3)
	if len(targets) == 0 {
		return aiToolExecutionResult{
			ToolName:   "table_describe",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    "-- metadata: describe table columns",
			ToolOutput: "### 工具 table_describe 结果\n- 未找到目标表，跳过",
		}
	}

	var out strings.Builder
	out.WriteString("### 工具 table_describe 结果\n")
	for _, table := range targets {
		out.WriteString(fmt.Sprintf("- 表 `%s` 字段概览：\n", table.Name))
		limit := len(table.Columns)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			col := table.Columns[i]
			out.WriteString(fmt.Sprintf("  - %s %s", col.Name, col.Type))
			if col.IsPrimary {
				out.WriteString(" [PK]")
			}
			if col.Comment != "" {
				out.WriteString(" // " + col.Comment)
			}
			out.WriteString("\n")
		}
		if len(table.Columns) > limit {
			out.WriteString(fmt.Sprintf("  - ... 其余 %d 个字段省略\n", len(table.Columns)-limit))
		}
	}

	return aiToolExecutionResult{
		ToolName:   "table_describe",
		ToolSQL:    "-- metadata: describe selected tables",
		ToolOutput: out.String(),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableDDL(userQuestion string, schema *ai.SchemaContext, mentions []string, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTables(schema, userQuestion, mentions, 2)
	if len(targets) == 0 {
		return aiToolExecutionResult{
			ToolName:   "table_ddl",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    "-- metadata: show create table",
			ToolOutput: "### 工具 table_ddl 结果\n- 未找到目标表，跳过",
		}
	}
	return aiToolExecutionResult{
		ToolName:   "table_ddl",
		ToolSQL:    "-- metadata: show create table",
		ToolOutput: "### 工具 table_ddl 结果\n" + ai.BuildTablesDDL(targets),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableStats(connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTables(schema, userQuestion, mentions, 3)
	if len(targets) == 0 {
		return aiToolExecutionResult{
			ToolName:   "table_stats",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    "-- metadata: table stats",
			ToolOutput: "### 工具 table_stats 结果\n- 未找到目标表，跳过",
		}
	}

	dbService := NewDatabaseService(s.manager)
	var lines []string
	for _, t := range targets {
		stats, err := dbService.GetTableStats(connID, dbName, t.Name)
		if err != nil || stats == nil {
			lines = append(lines, fmt.Sprintf("- %s: 获取统计失败(%v)", t.Name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: rows=%d totalSize=%d engine=%s", t.Name, stats.RowCount, stats.TotalSize, stats.Engine))
	}

	return aiToolExecutionResult{
		ToolName:   "table_stats",
		ToolSQL:    "-- metadata: get table statistics",
		ToolOutput: "### 工具 table_stats 结果\n" + strings.Join(lines, "\n"),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func parseMentions(question string) []string {
	re := regexp.MustCompile(`@([a-zA-Z0-9_:\-]+)`)
	matched := re.FindAllStringSubmatch(question, -1)
	seen := make(map[string]bool)
	var out []string
	for _, m := range matched {
		if len(m) < 2 {
			continue
		}
		key := strings.TrimSpace(m[1])
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func planToolsByRule(userQuestion string, mentions []string) []string {
	questionLower := strings.ToLower(userQuestion)
	toolSet := map[string]bool{}

	// @tool:xxx 显式指定工具优先
	for _, m := range mentions {
		if strings.HasPrefix(strings.ToLower(m), "tool:") {
			toolName := strings.TrimPrefix(strings.ToLower(m), "tool:")
			if toolName != "" {
				toolSet[toolName] = true
			}
		}
	}

	// 规则路由（关键词）
	if strings.Contains(questionLower, "表") || strings.Contains(questionLower, "table") {
		toolSet["table_fuzzy_match"] = true
	}
	if strings.Contains(questionLower, "字段") || strings.Contains(questionLower, "结构") || strings.Contains(questionLower, "列") || strings.Contains(questionLower, "describe") {
		toolSet["table_describe"] = true
	}
	if strings.Contains(questionLower, "ddl") || strings.Contains(questionLower, "建表") || strings.Contains(questionLower, "create table") {
		toolSet["table_ddl"] = true
	}
	if strings.Contains(questionLower, "统计") || strings.Contains(questionLower, "行数") || strings.Contains(questionLower, "大小") {
		toolSet["table_stats"] = true
	}

	// @table:xxx 或 @xxx 形式默认增加结构工具
	for _, m := range mentions {
		if strings.HasPrefix(strings.ToLower(m), "table:") || (!strings.Contains(m, ":") && !strings.HasPrefix(strings.ToLower(m), "tool")) {
			toolSet["table_describe"] = true
		}
	}

	ordered := []string{"table_fuzzy_match", "table_describe", "table_ddl", "table_stats"}
	var result []string
	for _, item := range ordered {
		if toolSet[item] {
			result = append(result, item)
		}
	}
	return result
}

func pickTargetTables(schema *ai.SchemaContext, userQuestion string, mentions []string, max int) []ai.TableSchema {
	if schema == nil || len(schema.Tables) == 0 {
		return nil
	}
	nameMap := make(map[string]ai.TableSchema, len(schema.Tables))
	for _, t := range schema.Tables {
		nameMap[strings.ToLower(t.Name)] = t
	}

	var picked []ai.TableSchema
	seen := map[string]bool{}
	add := func(name string) {
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "" || seen[n] {
			return
		}
		if t, ok := nameMap[n]; ok {
			seen[n] = true
			picked = append(picked, t)
		}
	}

	for _, m := range mentions {
		if strings.HasPrefix(strings.ToLower(m), "table:") {
			add(strings.TrimPrefix(m, "table:"))
			continue
		}
		if !strings.Contains(m, ":") {
			add(m)
		}
	}

	if len(picked) == 0 {
		relevant := filterRelevantTables(schema.Tables, userQuestion)
		for _, t := range relevant {
			add(t.Name)
			if len(picked) >= max {
				break
			}
		}
	}

	if len(picked) > max {
		picked = picked[:max]
	}
	return picked
}
