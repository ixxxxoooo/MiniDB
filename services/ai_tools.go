package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/database"
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
	ToolCallID string
	ToolSQL    string
	ToolOutput string
	DurationMs int64
	Err        error
}

type aiToolPlanDecision struct {
	Action string `json:"action"`
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

type aiToolCallArgs struct {
	TableNames []string
	Keywords   []string
	Limit      int
	SQL        string
}

type aiToolInvocationPlan struct {
	Tool   string
	Reason string
	Args   aiToolCallArgs
}

const maxToolCallSteps = 6

var orderedToolNames = []string{"table_fuzzy_match", "table_describe", "sql_readonly_execute", "table_ddl", "table_stats"}

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

// runPlannedTools 工具调用调度器：模型函数调用优先 + 规则回退（结果可注入后续上下文）
func (s *AIService) runPlannedTools(connID, dbName, userQuestion string, schema *ai.SchemaContext, requestID string, stream bool) string {
	if schema == nil || strings.TrimSpace(userQuestion) == "" {
		return ""
	}

	start := time.Now()
	mentions := parseMentions(userQuestion)
	seedTools := planToolsByRule(userQuestion, mentions)

	if stream {
		s.emitStreamEvent(ChatStreamEvent{
			RequestID: requestID,
			Type:      "status",
			Delta:     "planning_tools",
		})
	}

	var outputs []string
	usedTools := map[string]bool{}
	var errCount int
	var toolChain []string

	// 发射推理事件：分析用户意图
	if stream {
		s.emitStreamEvent(ChatStreamEvent{
			RequestID:       requestID,
			Type:            "thinking",
			ThinkingContent: "正在分析用户问题，识别需要查询的数据库信息...",
		})
	}

	for step := 1; step <= maxToolCallSteps; step++ {
		// 发射循环状态事件：标记当前迭代轮次
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:     requestID,
				Type:          "loop_status",
				LoopAction:    "CONTINUE",
				LoopReason:    "继续收集信息",
				LoopIteration: step,
				LoopMaxIter:   maxToolCallSteps,
			})
		}
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "status",
				Delta:     fmt.Sprintf("round_%d_planning", step),
			})
		}
		plan := s.planNextTool(userQuestion, mentions, seedTools, outputs, usedTools)
		if plan.Tool == "" {
			// 发射循环终止状态：信息收集完成
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:     requestID,
					Type:          "loop_status",
					LoopAction:    "FINALIZE",
					LoopReason:    "信息收集完成，准备生成回答",
					LoopIteration: step,
					LoopMaxIter:   maxToolCallSteps,
				})
			}
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID: requestID,
					Type:      "status",
					Delta:     "tool_loop_done",
				})
			}
			break
		}

		// 发射推理事件：工具选择理由
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:       requestID,
				Type:            "thinking",
				ThinkingContent: fmt.Sprintf("选择工具 %s：%s", plan.Tool, fallbackReason(plan.Reason)),
			})
		}

		callID := fmt.Sprintf("tool_%d_%02d", start.UnixMilli(), step)
		toolInput := formatToolInput(userQuestion, plan.Args)
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "status",
				Delta:     fmt.Sprintf("round_%d_running_%s", step, plan.Tool),
			})
		}

		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_plan",
				ToolName:   plan.Tool,
				ToolCallID: callID,
				ToolState:  "planned",
				ToolOutput: fallbackReason(plan.Reason),
			})
		}

		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_start",
				ToolName:   plan.Tool,
				ToolCallID: callID,
				ToolState:  "running",
				ToolInput:  toolInput,
			})
		}

		logger.Info("[AIService][Tools] 开始执行: step=%d tool=%s input=%s", step, plan.Tool, toolInput)
		result := s.executeTool(plan.Tool, connID, dbName, userQuestion, schema, mentions, plan.Args)
		result.ToolCallID = callID
		if result.Err != nil {
			errCount++
			logger.Warn("[AIService][Tools] 工具执行失败: tool=%s err=%v", plan.Tool, result.Err)
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:  requestID,
					Type:       "tool_error",
					ToolName:   plan.Tool,
					ToolCallID: callID,
					ToolState:  "error",
					ToolInput:  toolInput,
					ToolOutput: result.Err.Error(),
					DurationMs: result.DurationMs,
				})
			}
			usedTools[plan.Tool] = true
			// 发射分析事件：工具执行失败
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:       requestID,
					Type:            "analysis",
					AnalysisContent: generateToolAnalysis(plan.Tool, "", true),
					ToolName:        plan.Tool,
				})
			}
			if stream {
				s.emitStreamEvent(ChatStreamEvent{
					RequestID: requestID,
					Type:      "status",
					Delta:     fmt.Sprintf("round_%d_error", step),
				})
			}
			continue
		}
		logger.Info("[AIService][Tools] 执行完成: step=%d tool=%s duration=%dms output_len=%d", step, plan.Tool, result.DurationMs, len(result.ToolOutput))

		if stream && result.ToolSQL != "" {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_sql",
				ToolName:   plan.Tool,
				ToolCallID: callID,
				ToolState:  "running",
				ToolSQL:    result.ToolSQL,
			})
		}
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_result",
				ToolName:   plan.Tool,
				ToolCallID: callID,
				ToolState:  "success",
				ToolOutput: result.ToolOutput,
				DurationMs: result.DurationMs,
			})
		}

		// 发射分析事件：工具执行成功
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:       requestID,
				Type:            "analysis",
				AnalysisContent: generateToolAnalysis(plan.Tool, result.ToolOutput, false),
				ToolName:        plan.Tool,
			})
		}
		toolChain = append(toolChain, plan.Tool)

		outputs = append(outputs, fmt.Sprintf("### 工具调用 #%d `%s`\n%s", step, plan.Tool, result.ToolOutput))
		usedTools[plan.Tool] = true
		if stream {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "status",
				Delta:     fmt.Sprintf("round_%d_completed", step),
			})
		}
	}

	// 发射执行轨迹汇总
	if stream && len(toolChain) > 0 {
		s.emitStreamEvent(ChatStreamEvent{
			RequestID:       requestID,
			Type:            "execution_trace",
			TraceToolChain:  strings.Join(toolChain, " → "),
			TraceTotalIter:  len(toolChain),
			TraceDurationMs: time.Since(start).Milliseconds(),
		})
	}

	if len(outputs) == 0 {
		return ""
	}

	logger.Info("[AIService][Tools] 工具执行完成: requestID=%s tools=%d errors=%d duration=%dms", requestID, len(outputs), errCount, time.Since(start).Milliseconds())
	return "\n\n-- 以下是工具链返回的可审计上下文（非黑盒）：\n" + strings.Join(outputs, "\n\n")
}

func (s *AIService) planNextTool(userQuestion string, mentions, seedTools, previousOutputs []string, used map[string]bool) aiToolInvocationPlan {
	available := availableTools(used)
	if len(available) == 0 {
		return aiToolInvocationPlan{}
	}

	if plan, ok := s.planNextToolByFunctionCall(userQuestion, mentions, previousOutputs, available); ok {
		return plan
	}

	for _, tool := range seedTools {
		if used[tool] {
			continue
		}
		return aiToolInvocationPlan{Tool: tool, Reason: "规则路由命中，优先执行"}
	}

	if len(previousOutputs) == 0 {
		// 无规则命中且尚无上下文时，不强行调用工具，避免无意义“空转”
		return aiToolInvocationPlan{}
	}

	decision, err := s.planNextToolByAI(userQuestion, previousOutputs, available)
	if err != nil {
		logger.Warn("[AIService][Tools] AI 规划下一工具失败: %v", err)
		return aiToolInvocationPlan{}
	}
	if strings.ToLower(strings.TrimSpace(decision.Action)) == "stop" {
		return aiToolInvocationPlan{}
	}
	tool := normalizeToolName(decision.Tool)
	if !containsTool(available, tool) {
		return aiToolInvocationPlan{}
	}
	return aiToolInvocationPlan{
		Tool:   tool,
		Reason: fallbackReason(decision.Reason),
	}
}

func (s *AIService) planNextToolByFunctionCall(userQuestion string, mentions, previousOutputs, available []string) (aiToolInvocationPlan, bool) {
	tools := buildFunctionToolDefinitions(available)
	if len(tools) == 0 {
		return aiToolInvocationPlan{}, false
	}

	prompt := buildFunctionPlannerPrompt(userQuestion, mentions, previousOutputs, available)
	res, err := s.client.PlanToolCalls(
		context.Background(),
		"你是数据库助手的工具规划器。你要么发起一个函数调用，要么输出 stop。不要回答业务问题。",
		prompt,
		tools,
	)
	if err != nil {
		logger.Warn("[AIService][Tools] 函数调用规划失败，回退规则规划: %v", err)
		return aiToolInvocationPlan{}, false
	}
	if len(res.ToolCalls) == 0 {
		return aiToolInvocationPlan{}, false
	}

	call := res.ToolCalls[0]
	tool := normalizeToolName(call.Name)
	if !containsTool(available, tool) {
		return aiToolInvocationPlan{}, false
	}

	reason := "模型函数调用"
	content := strings.TrimSpace(res.Content)
	if content != "" && strings.ToLower(content) != "stop" {
		reason = content
	}

	return aiToolInvocationPlan{
		Tool:   tool,
		Reason: reason,
		Args:   parseToolCallArgs(call.Arguments),
	}, true
}

func buildFunctionPlannerPrompt(userQuestion string, mentions, previousOutputs, available []string) string {
	mentionsText := "(无)"
	if len(mentions) > 0 {
		mentionsText = strings.Join(mentions, ", ")
	}
	return fmt.Sprintf(
		"用户问题：\n%s\n\n显式提及（@）：%s\n\n当前可用工具：%s\n\n已完成工具结果摘要：\n%s\n\n规则：\n1) 如果还需要额外信息，调用一个最合适的工具（只调用一个）。\n2) 如果信息足够，输出 stop，不要调用工具。\n3) 优先使用用户提及的表名。\n4) 调用 table_describe/table_ddl/table_stats 时尽量填写 table_names。\n5) 调用 sql_readonly_execute 时优先给出完整只读 SQL；若无法写 SQL，再给 table_names。",
		userQuestion,
		mentionsText,
		strings.Join(available, ", "),
		summarizeToolOutputs(previousOutputs, 1200),
	)
}

func buildFunctionToolDefinitions(available []string) []ai.FunctionToolDefinition {
	defs := make([]ai.FunctionToolDefinition, 0, len(available))
	for _, tool := range available {
		switch tool {
		case "table_fuzzy_match":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "按关键词匹配潜在相关表名",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keywords": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
					},
					"additionalProperties": false,
				},
			})
		case "table_describe":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "查看指定表字段定义",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"additionalProperties": false,
				},
			})
		case "sql_readonly_execute":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "执行只读 SQL 查询并返回结果摘要",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sql": map[string]any{"type": "string"},
						"table_names": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
					},
					"additionalProperties": false,
				},
			})
		case "table_ddl":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "查看指定表建表语句",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"additionalProperties": false,
				},
			})
		case "table_stats":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "查看指定表统计信息（行数、大小等）",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"additionalProperties": false,
				},
			})
		}
	}
	return defs
}

func (s *AIService) planNextToolByAI(userQuestion string, previousOutputs, available []string) (aiToolPlanDecision, error) {
	summary := summarizeToolOutputs(previousOutputs, 1200)
	plannerPrompt := fmt.Sprintf(
		"用户问题：\n%s\n\n当前可用工具（只能二选一：调用或停止）：\n%s\n\n已完成工具结果摘要：\n%s\n\n请只返回 JSON：\n{\"action\":\"call|stop\",\"tool\":\"工具名\",\"reason\":\"一句话原因\"}",
		userQuestion,
		strings.Join(available, ", "),
		summary,
	)
	resp, err := s.client.ChatWithMessages(
		context.Background(),
		"你是数据库助手的工具调度器。你每次只决定下一步要不要调用一个工具。只能输出 JSON，不要解释。",
		[]ai.ChatMessage{{Role: "user", Content: plannerPrompt}},
	)
	if err != nil {
		return aiToolPlanDecision{}, err
	}
	decision, ok := parseToolPlanDecision(resp)
	if !ok {
		return aiToolPlanDecision{}, fmt.Errorf("工具规划解析失败: %s", truncateToolPlannerText(resp, 200))
	}
	if decision.Action == "" {
		decision.Action = "stop"
	}
	return decision, nil
}

func (s *AIService) executeTool(toolName, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs) aiToolExecutionResult {
	begin := time.Now()
	switch toolName {
	case "table_fuzzy_match":
		return s.execToolTableFuzzyMatch(userQuestion, schema, args, begin)
	case "table_describe":
		return s.execToolTableDescribe(userQuestion, schema, mentions, args, begin)
	case "sql_readonly_execute":
		return s.execToolSQLReadonlyExecute(connID, dbName, userQuestion, schema, mentions, args, begin)
	case "table_ddl":
		return s.execToolTableDDL(userQuestion, schema, mentions, args, begin)
	case "table_stats":
		return s.execToolTableStats(connID, dbName, userQuestion, schema, mentions, args, begin)
	default:
		return aiToolExecutionResult{
			ToolName:   toolName,
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("未知工具: %s", toolName),
		}
	}
}

func (s *AIService) execToolTableFuzzyMatch(userQuestion string, schema *ai.SchemaContext, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	keywords := normalizeKeywords(args.Keywords)
	if len(keywords) == 0 {
		keywords = extractKeywords(userQuestion)
	}
	if len(keywords) == 0 {
		keywords = []string{strings.ToLower(strings.TrimSpace(userQuestion))}
	}
	limit := args.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	logger.Debug("[AIService][Tools] table_fuzzy_match: keywords=%v limit=%d", keywords, limit)

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
	if len(matches) > limit {
		matches = matches[:limit]
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

func (s *AIService) execToolTableDescribe(userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, 3)
	logger.Debug("[AIService][Tools] table_describe: table_names=%v picked=%d", args.TableNames, len(targets))
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

func (s *AIService) execToolSQLReadonlyExecute(connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	limit := args.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	logger.Debug("[AIService][Tools] sql_readonly_execute: limit=%d raw_sql_len=%d table_names=%v", limit, len(args.SQL), args.TableNames)

	sqlStr := strings.TrimSpace(args.SQL)
	if sqlStr == "" {
		targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, 1)
		if len(targets) == 0 {
			return aiToolExecutionResult{
				ToolName:   "sql_readonly_execute",
				DurationMs: time.Since(begin).Milliseconds(),
				ToolSQL:    "-- query: missing SQL and target table",
				ToolOutput: "### 工具 sql_readonly_execute 结果\n- 未提供 SQL，且无法推断目标表，跳过",
			}
		}
		tableName := database.QuoteTableName(schema.DatabaseType, targets[0].Name)
		sqlStr = fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	}

	check := database.CheckAutoExecutableSelectSQL(sqlStr)
	if !check.Allowed {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        fmt.Errorf("只允许只读 SQL，检测到高风险语句: %s", check.Verb),
		}
	}

	if s.query == nil {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        fmt.Errorf("QueryService 未注入，无法执行只读 SQL"),
		}
	}

	result, err := s.query.ExecuteSQLPaged(connID, dbName, sqlStr, 1, limit)
	if err != nil {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        err,
		}
	}
	if result == nil {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        fmt.Errorf("执行结果为空"),
		}
	}
	if strings.TrimSpace(result.Error) != "" {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        fmt.Errorf(result.Error),
		}
	}

	output := buildReadonlyQueryToolOutput(result)
	return aiToolExecutionResult{
		ToolName:   "sql_readonly_execute",
		ToolSQL:    sqlStr,
		ToolOutput: "### 工具 sql_readonly_execute 结果\n" + output,
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableDDL(userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, 2)
	logger.Debug("[AIService][Tools] table_ddl: table_names=%v picked=%d", args.TableNames, len(targets))
	if len(targets) == 0 {
		return aiToolExecutionResult{
			ToolName:   "table_ddl",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    "-- metadata: show create table",
			ToolOutput: "### 工具 table_ddl 结果\n- 未找到目标表，跳过",
		}
	}
	// 用 Markdown SQL 代码块包裹 DDL，前端渲染时自动语法高亮
	ddl := ai.BuildTablesDDL(targets)
	return aiToolExecutionResult{
		ToolName:   "table_ddl",
		ToolSQL:    "-- metadata: show create table",
		ToolOutput: "### 工具 table_ddl 结果\n\n```sql\n" + ddl + "\n```",
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableStats(connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, 3)
	logger.Debug("[AIService][Tools] table_stats: table_names=%v picked=%d", args.TableNames, len(targets))
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
	if strings.Contains(questionLower, "查询") || strings.Contains(questionLower, "查一下") || strings.Contains(questionLower, "查出") ||
		strings.Contains(questionLower, "统计结果") || strings.Contains(questionLower, "select") || strings.Contains(questionLower, "where") {
		toolSet["sql_readonly_execute"] = true
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

	var result []string
	for _, item := range orderedToolNames {
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

func pickTargetTablesWithArgs(schema *ai.SchemaContext, tableNames []string, userQuestion string, mentions []string, max int) []ai.TableSchema {
	if schema == nil || len(schema.Tables) == 0 {
		return nil
	}
	if len(tableNames) == 0 {
		return pickTargetTables(schema, userQuestion, mentions, max)
	}

	nameMap := make(map[string]ai.TableSchema, len(schema.Tables))
	for _, t := range schema.Tables {
		nameMap[strings.ToLower(t.Name)] = t
	}
	var picked []ai.TableSchema
	seen := map[string]bool{}
	for _, raw := range tableNames {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		if t, ok := nameMap[name]; ok {
			seen[name] = true
			picked = append(picked, t)
		}
		if len(picked) >= max {
			break
		}
	}
	if len(picked) > 0 {
		return picked
	}
	return pickTargetTables(schema, userQuestion, mentions, max)
}

func availableTools(used map[string]bool) []string {
	var available []string
	for _, tool := range orderedToolNames {
		if used[tool] {
			continue
		}
		available = append(available, tool)
	}
	return available
}

func normalizeToolName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func containsTool(available []string, tool string) bool {
	for _, item := range available {
		if item == tool {
			return true
		}
	}
	return false
}

func summarizeToolOutputs(outputs []string, maxLen int) string {
	if len(outputs) == 0 {
		return "(空)"
	}
	raw := strings.Join(outputs, "\n\n")
	if len(raw) <= maxLen {
		return raw
	}
	return raw[:maxLen] + "...(截断)"
}

func parseToolPlanDecision(text string) (aiToolPlanDecision, bool) {
	jsonText := extractJSONObject(text)
	if jsonText == "" {
		return aiToolPlanDecision{}, false
	}
	var decision aiToolPlanDecision
	if err := json.Unmarshal([]byte(jsonText), &decision); err != nil {
		return aiToolPlanDecision{}, false
	}
	decision.Action = strings.TrimSpace(strings.ToLower(decision.Action))
	decision.Tool = normalizeToolName(decision.Tool)
	decision.Reason = strings.TrimSpace(decision.Reason)
	if decision.Action == "" {
		decision.Action = "stop"
	}
	if decision.Action != "call" && decision.Action != "stop" {
		decision.Action = "stop"
	}
	return decision, true
}

func parseToolCallArgs(arguments string) aiToolCallArgs {
	args := aiToolCallArgs{}
	raw := strings.TrimSpace(arguments)
	if raw == "" {
		return args
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return args
	}
	args.TableNames = normalizeStringSlice(extractStringArray(payload, []string{"table_names", "tableNames", "tables"}))
	args.Keywords = normalizeKeywords(extractStringArray(payload, []string{"keywords", "keyword"}))
	args.Limit = extractInt(payload, []string{"limit", "top_k", "topK"})
	args.SQL = strings.TrimSpace(extractString(payload, []string{"sql", "query", "statement"}))
	return args
}

func extractString(payload map[string]any, keys []string) string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractStringArray(payload map[string]any, keys []string) []string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				return nil
			}
			return []string{val}
		case []any:
			out := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func extractInt(payload map[string]any, keys []string) int {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case int64:
			return int(val)
		}
	}
	return 0
}

func normalizeStringSlice(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func normalizeKeywords(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		kw := strings.ToLower(strings.TrimSpace(item))
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		out = append(out, kw)
	}
	return out
}

func buildReadonlyQueryToolOutput(result *database.QueryResult) string {
	if result == nil {
		return "- 空结果"
	}
	if len(result.Rows) == 0 {
		return fmt.Sprintf("- 查询成功，0 行（耗时 %dms）", result.Duration)
	}

	columns := make([]string, 0, len(result.Columns))
	for _, col := range result.Columns {
		if strings.TrimSpace(col.Name) == "" {
			continue
		}
		columns = append(columns, col.Name)
	}
	if len(columns) == 0 && len(result.Rows) > 0 {
		for key := range result.Rows[0] {
			columns = append(columns, key)
		}
		sort.Strings(columns)
	}
	if len(columns) == 0 {
		return fmt.Sprintf("- 查询成功，返回 %d 行（耗时 %dms）", len(result.Rows), result.Duration)
	}

	rowLimit := 20
	if len(result.Rows) < rowLimit {
		rowLimit = len(result.Rows)
	}
	total := result.Total
	if total == 0 && len(result.Rows) > 0 {
		total = int64(len(result.Rows))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- 查询成功，返回 %d 行（展示前 %d 行，耗时 %dms）\n\n", total, rowLimit, result.Duration))
	sb.WriteString("| " + strings.Join(columns, " | ") + " |\n")
	separators := make([]string, len(columns))
	for i := range separators {
		separators[i] = "---"
	}
	sb.WriteString("| " + strings.Join(separators, " | ") + " |\n")
	for i := 0; i < rowLimit; i++ {
		row := result.Rows[i]
		cells := make([]string, 0, len(columns))
		for _, col := range columns {
			val := row[col]
			cell := "NULL"
			if val != nil {
				cell = strings.TrimSpace(fmt.Sprint(val))
				if cell == "" {
					cell = "''"
				}
			}
			if len(cell) > 120 {
				cell = cell[:120] + "..."
			}
			cell = strings.ReplaceAll(cell, "\n", " ")
			cell = strings.ReplaceAll(cell, "|", "\\|")
			cells = append(cells, cell)
		}
		sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return strings.TrimSpace(sb.String())
}

func hasToolArgs(args aiToolCallArgs) bool {
	return len(args.TableNames) > 0 || len(args.Keywords) > 0 || args.Limit > 0 || strings.TrimSpace(args.SQL) != ""
}

func formatToolInput(userQuestion string, args aiToolCallArgs) string {
	if !hasToolArgs(args) {
		return userQuestion
	}
	payload := map[string]any{}
	if len(args.TableNames) > 0 {
		payload["table_names"] = args.TableNames
	}
	if len(args.Keywords) > 0 {
		payload["keywords"] = args.Keywords
	}
	if args.Limit > 0 {
		payload["limit"] = args.Limit
	}
	if strings.TrimSpace(args.SQL) != "" {
		payload["sql"] = strings.TrimSpace(args.SQL)
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		return userQuestion
	}
	return string(bs)
}

func fallbackReason(reason string) string {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "规划工具调用"
	}
	return r
}

func extractJSONObject(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 {
			trimmed = strings.Join(lines[1:len(lines)-1], "\n")
			trimmed = strings.TrimSpace(trimmed)
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return ""
	}
	return trimmed[start : end+1]
}

func truncateToolPlannerText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen]
}

// generateToolAnalysis 基于工具执行结果生成简短分析文案（规则生成，无额外 API 调用）
func generateToolAnalysis(toolName, output string, hasError bool) string {
	if hasError {
		return fmt.Sprintf("工具 %s 执行失败，需要调整策略或尝试其他方式获取数据", toolName)
	}
	outputLen := len(output)
	if outputLen == 0 {
		return fmt.Sprintf("工具 %s 返回空结果，可能需要调整查询条件", toolName)
	}
	if outputLen > 500 {
		return fmt.Sprintf("工具 %s 返回了丰富的数据（%d 字符），信息量充足", toolName, outputLen)
	}
	return fmt.Sprintf("工具 %s 已返回结果（%d 字符），继续评估是否需要更多信息", toolName, outputLen)
}
