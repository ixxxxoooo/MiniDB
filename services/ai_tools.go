package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
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

type aiToolCallArgs struct {
	TableName  string
	TableNames []string
	Columns    []string
	Keywords   []string
	Limit      int
	SQL        string
}

// maxToolCallRounds ReAct 循环最大轮次，防止 AI 无限调用工具
const maxToolCallRounds = 6

// tableStatsMaxTables limits one table_stats call. Keep this large enough that
// medium-sized schemas do not burn multiple ReAct rounds just collecting counts.
const tableStatsMaxTables = 20
const tableSampleMaxRows = 100
const tableProfileMaxTables = 3
const tableProfileMaxColumns = 8
const tableProfileDistinctRowThreshold = 100000
const tableStatsMaxConcurrency = 4
const tableProfileMaxConcurrency = 3

var orderedToolNames = []string{"table_fuzzy_match", "table_describe", "sql_readonly_execute", "table_ddl", "table_stats", "table_sample", "table_profile", "sql_explain_plan"}

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
			Description: "查看指定表的行数与基础统计（单次最多 20 张表，后台最多 4 并发）",
			ReadOnly:    true,
		},
		{
			Name:        "table_sample",
			Description: "安全抽样查看指定表前 N 行（默认 20，最多 100）",
			ReadOnly:    true,
		},
		{
			Name:        "table_profile",
			Description: "查看表字段画像（单次最多 3 张表、每表最多 8 列，后台最多 3 并发）",
			ReadOnly:    true,
		},
		{
			Name:        "sql_explain_plan",
			Description: "对单条只读 SQL 生成 EXPLAIN 执行计划，不实际执行查询",
			ReadOnly:    true,
		},
		{
			Name:        "sql_readonly_execute",
			Description: "执行只读 SQL（SELECT/SHOW/EXPLAIN/DESC）",
			ReadOnly:    true,
		},
	}
}

// BuildAllToolDefinitions 构建全部工具的 OpenAI Function 定义，供 ReAct 循环使用
func BuildAllToolDefinitions() []ai.FunctionToolDefinition {
	return buildFunctionToolDefinitions(orderedToolNames)
}

// ExecuteToolFromAICall 解析 AI 返回的 FunctionToolCall 并执行对应工具
// 返回工具执行结果的文本输出，供回填到对话历史的 tool 消息中
func (s *AIService) ExecuteToolFromAICall(call ai.FunctionToolCall, connID, dbName, userQuestion string, schema *ai.SchemaContext) aiToolExecutionResult {
	return s.ExecuteToolFromAICallContext(context.Background(), call, connID, dbName, userQuestion, schema)
}

//wails:ignore
func (s *AIService) ExecuteToolFromAICallContext(ctx context.Context, call ai.FunctionToolCall, connID, dbName, userQuestion string, schema *ai.SchemaContext) aiToolExecutionResult {
	if ctx == nil {
		ctx = context.Background()
	}
	toolName := normalizeToolName(call.Name)
	args := parseToolCallArgs(call.Arguments)
	mentions := parseMentions(userQuestion)

	logger.Info("[AIService][Tools] ReAct 执行工具: tool=%s callID=%s args=%s", toolName, call.ID, call.Arguments)
	result := s.executeTool(ctx, toolName, connID, dbName, userQuestion, schema, mentions, args)
	result.ToolCallID = call.ID
	return result
}

// executeTool 工具执行分发器
func (s *AIService) executeTool(ctx context.Context, toolName, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs) aiToolExecutionResult {
	begin := time.Now()
	if err := ctx.Err(); err != nil {
		return cancelledToolResult(toolName, begin, err)
	}
	switch toolName {
	case "table_fuzzy_match":
		return s.execToolTableFuzzyMatch(userQuestion, schema, args, begin)
	case "table_describe":
		return s.execToolTableDescribe(userQuestion, schema, mentions, args, begin)
	case "sql_readonly_execute":
		return s.execToolSQLReadonlyExecute(ctx, connID, dbName, userQuestion, schema, mentions, args, begin)
	case "table_ddl":
		return s.execToolTableDDL(userQuestion, schema, mentions, args, begin)
	case "table_stats":
		return s.execToolTableStats(ctx, connID, dbName, userQuestion, schema, mentions, args, begin)
	case "table_sample":
		return s.execToolTableSample(ctx, connID, dbName, userQuestion, schema, mentions, args, begin)
	case "table_profile":
		return s.execToolTableProfile(ctx, connID, dbName, userQuestion, schema, mentions, args, begin)
	case "sql_explain_plan":
		return s.execToolSQLExplainPlan(ctx, connID, dbName, args, begin)
	default:
		return aiToolExecutionResult{
			ToolName:   toolName,
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("未知工具: %s", toolName),
		}
	}
}

func cancelledToolResult(toolName string, begin time.Time, err error) aiToolExecutionResult {
	return aiToolExecutionResult{
		ToolName:   toolName,
		DurationMs: time.Since(begin).Milliseconds(),
		Err:        err,
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

func (s *AIService) execToolSQLReadonlyExecute(ctx context.Context, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
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

	check := database.CheckAutoExecutableReadOnlySingleSQL(sqlStr)
	if !check.Allowed {
		return aiToolExecutionResult{
			ToolName:   "sql_readonly_execute",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    sqlStr,
			Err:        fmt.Errorf("只允许单条只读 SQL，检测结果: %s %s", check.ReasonCode, check.Verb),
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

	result, err := s.query.ExecuteSQLPagedContext(ctx, connID, dbName, sqlStr, 1, limit)
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
			Err:        errors.New(result.Error),
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
	ddl := ai.BuildTablesDDL(targets)
	return aiToolExecutionResult{
		ToolName:   "table_ddl",
		ToolSQL:    "-- metadata: show create table",
		ToolOutput: "### 工具 table_ddl 结果\n\n```sql\n" + ddl + "\n```",
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableStats(ctx context.Context, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, tableStatsMaxTables)
	logger.Debug("[AIService][Tools] table_stats: table_names=%v picked=%d", args.TableNames, len(targets))
	if len(targets) == 0 {
		return aiToolExecutionResult{
			ToolName:   "table_stats",
			DurationMs: time.Since(begin).Milliseconds(),
			ToolSQL:    "-- metadata: table stats",
			ToolOutput: "### 工具 table_stats 结果\n- 未找到目标表，跳过",
		}
	}

	dbService := NewDatabaseService(s.manager, s.schema)
	lines := make([]string, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, tableStatsMaxConcurrency)
	for i, t := range targets {
		i, t := i, t
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := ctx.Err(); err != nil {
				lines[i] = fmt.Sprintf("- %s: 获取统计取消(%v)", t.Name, err)
				return
			}
			stats, err := dbService.GetTableStats(connID, dbName, t.Name)
			if err != nil || stats == nil {
				lines[i] = fmt.Sprintf("- %s: 获取统计失败(%v)", t.Name, err)
				return
			}
			lines[i] = fmt.Sprintf("- %s: rows=%d totalSize=%d engine=%s", t.Name, stats.RowCount, stats.TotalSize, stats.Engine)
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return cancelledToolResult("table_stats", begin, err)
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = fmt.Sprintf("- %s: 获取统计失败(无结果)", targets[i].Name)
		}
	}

	return aiToolExecutionResult{
		ToolName:   "table_stats",
		ToolSQL:    "-- metadata: get table statistics",
		ToolOutput: "### 工具 table_stats 结果\n" + strings.Join(lines, "\n"),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func (s *AIService) execToolTableSample(ctx context.Context, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	table, ok := pickSingleTableForTool(schema, args.TableName, args.TableNames, userQuestion, mentions)
	if !ok {
		return jsonToolResult("table_sample", "-- query: table sample", begin, map[string]any{
			"ok":      false,
			"summary": "未找到目标表",
			"rows":    []map[string]any{},
		})
	}
	limit := clampInt(args.Limit, 20, 1, tableSampleMaxRows)
	quotedTable := database.QuoteTableName(schema.DatabaseType, table.Name)
	sqlStr := fmt.Sprintf("SELECT * FROM %s LIMIT %d", quotedTable, limit)
	result, err := s.executeToolQuery(ctx, connID, dbName, sqlStr, limit)
	if err != nil {
		return aiToolExecutionResult{ToolName: "table_sample", ToolSQL: sqlStr, DurationMs: time.Since(begin).Milliseconds(), Err: err}
	}
	columns := queryResultColumnNames(result)
	payload := map[string]any{
		"ok":          true,
		"tool":        "table_sample",
		"table":       table.Name,
		"limit":       limit,
		"sql":         sqlStr,
		"columns":     columns,
		"rows":        result.Rows,
		"rowCount":    len(result.Rows),
		"total":       result.Total,
		"durationMs":  result.Duration,
		"autoLimited": result.AutoLimited,
		"summary":     fmt.Sprintf("表 %s 抽样返回 %d 行", table.Name, len(result.Rows)),
	}
	return jsonToolResult("table_sample", sqlStr, begin, payload)
}

func (s *AIService) execToolTableProfile(ctx context.Context, connID, dbName, userQuestion string, schema *ai.SchemaContext, mentions []string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	targets := pickTargetTablesWithArgs(schema, args.TableNames, userQuestion, mentions, tableProfileMaxTables)
	if len(targets) == 0 {
		return jsonToolResult("table_profile", "-- metadata: table profile", begin, map[string]any{
			"ok":      false,
			"summary": "未找到目标表",
			"tables":  []any{},
		})
	}
	var tables []map[string]any
	var sqls []string
	for _, table := range targets {
		if err := ctx.Err(); err != nil {
			return cancelledToolResult("table_profile", begin, err)
		}
		profile, tableSQLs := s.profileOneTable(ctx, connID, dbName, schema.DatabaseType, table, args.Columns)
		tables = append(tables, profile)
		sqls = append(sqls, tableSQLs...)
	}
	payload := map[string]any{
		"ok":      true,
		"tool":    "table_profile",
		"tables":  tables,
		"summary": fmt.Sprintf("已生成 %d 张表的字段画像", len(tables)),
	}
	return jsonToolResult("table_profile", strings.Join(sqls, ";\n"), begin, payload)
}

func (s *AIService) execToolSQLExplainPlan(ctx context.Context, connID, dbName string, args aiToolCallArgs, begin time.Time) aiToolExecutionResult {
	sqlStr := strings.TrimSpace(args.SQL)
	if sqlStr == "" {
		return aiToolExecutionResult{
			ToolName:   "sql_explain_plan",
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("sql_explain_plan 需要提供 sql"),
		}
	}
	check := database.CheckAutoExecutableReadOnlySingleSQL(sqlStr)
	if !check.Allowed {
		return aiToolExecutionResult{
			ToolName:   "sql_explain_plan",
			ToolSQL:    sqlStr,
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("EXPLAIN 仅允许单条只读 SQL，检测结果: %s %s", check.ReasonCode, check.Verb),
		}
	}
	if strings.Contains(strings.ToLower(sqlStr), "explain analyze") {
		return aiToolExecutionResult{
			ToolName:   "sql_explain_plan",
			ToolSQL:    sqlStr,
			DurationMs: time.Since(begin).Milliseconds(),
			Err:        fmt.Errorf("sql_explain_plan 不允许 EXPLAIN ANALYZE"),
		}
	}
	if strings.EqualFold(database.SQLLeadingVerb(sqlStr), "explain") {
		targetSQL := stripExplainPrefix(sqlStr)
		targetCheck := database.CheckAutoExecutableReadOnlySingleSQL(targetSQL)
		if !targetCheck.Allowed {
			return aiToolExecutionResult{
				ToolName:   "sql_explain_plan",
				ToolSQL:    sqlStr,
				DurationMs: time.Since(begin).Milliseconds(),
				Err:        fmt.Errorf("EXPLAIN 目标语句必须是只读 SQL，检测结果: %s %s", targetCheck.ReasonCode, targetCheck.Verb),
			}
		}
	}
	explainSQL := buildExplainSQL(sqlStr)
	result, err := s.executeToolQuery(ctx, connID, dbName, explainSQL, 100)
	if err != nil {
		return aiToolExecutionResult{ToolName: "sql_explain_plan", ToolSQL: explainSQL, DurationMs: time.Since(begin).Milliseconds(), Err: err}
	}
	payload := map[string]any{
		"ok":         true,
		"tool":       "sql_explain_plan",
		"sql":        sqlStr,
		"explainSql": explainSQL,
		"columns":    queryResultColumnNames(result),
		"rows":       result.Rows,
		"rowCount":   len(result.Rows),
		"durationMs": result.Duration,
		"summary":    fmt.Sprintf("EXPLAIN 返回 %d 行执行计划", len(result.Rows)),
	}
	return jsonToolResult("sql_explain_plan", explainSQL, begin, payload)
}

func (s *AIService) profileOneTable(ctx context.Context, connID, dbName, dbType string, table ai.TableSchema, requestedColumns []string) (map[string]any, []string) {
	quotedTable := database.QuoteTableName(dbType, table.Name)
	countSQL := fmt.Sprintf("SELECT COUNT(*) AS row_count FROM %s", quotedTable)
	sqls := []string{countSQL}
	rowCount := int64(0)
	countResult, err := s.executeToolQuery(ctx, connID, dbName, countSQL, 1)
	if err == nil && len(countResult.Rows) > 0 {
		rowCount = asInt64(firstRowValue(countResult.Rows[0], "row_count"))
	}
	columns := pickProfileColumns(table, requestedColumns)
	columnProfiles := make([]map[string]any, len(columns))
	statSQLs := make([]string, len(columns))
	var wg sync.WaitGroup
	sem := make(chan struct{}, tableProfileMaxConcurrency)
	for i, col := range columns {
		i, col := i, col
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := ctx.Err(); err != nil {
				columnProfiles[i] = map[string]any{"name": col.Name, "type": col.Type, "nullable": col.Nullable, "error": err.Error()}
				return
			}
			quotedColumn := database.QuoteIdent(dbType, col.Name)
			profile := map[string]any{
				"name":     col.Name,
				"type":     col.Type,
				"nullable": col.Nullable,
			}
			metrics := []string{
				fmt.Sprintf("SUM(CASE WHEN %s IS NULL THEN 1 ELSE 0 END) AS null_count", quotedColumn),
			}
			includeDistinct := rowCount <= tableProfileDistinctRowThreshold
			if includeDistinct {
				metrics = append(metrics, fmt.Sprintf("COUNT(DISTINCT %s) AS distinct_count", quotedColumn))
			} else {
				profile["distinctSkipped"] = true
				profile["distinctSkipReason"] = fmt.Sprintf("row_count>%d", tableProfileDistinctRowThreshold)
			}
			if isNumericColumnType(col.Type) {
				metrics = append(metrics, fmt.Sprintf("MIN(%s) AS min_value", quotedColumn), fmt.Sprintf("MAX(%s) AS max_value", quotedColumn))
			}
			statSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(metrics, ", "), quotedTable)
			statSQLs[i] = statSQL
			statResult, err := s.executeToolQuery(ctx, connID, dbName, statSQL, 1)
			if err != nil {
				profile["error"] = err.Error()
				columnProfiles[i] = profile
				return
			}
			if len(statResult.Rows) > 0 {
				row := statResult.Rows[0]
				profile["nullCount"] = firstRowValue(row, "null_count")
				if includeDistinct {
					profile["distinctCount"] = firstRowValue(row, "distinct_count")
				}
				if isNumericColumnType(col.Type) {
					profile["min"] = firstRowValue(row, "min_value")
					profile["max"] = firstRowValue(row, "max_value")
				}
			}
			columnProfiles[i] = profile
		}()
	}
	wg.Wait()
	for i, col := range columns {
		if strings.TrimSpace(statSQLs[i]) != "" {
			sqls = append(sqls, statSQLs[i])
		}
		if columnProfiles[i] == nil {
			columnProfiles[i] = map[string]any{"name": col.Name, "type": col.Type, "nullable": col.Nullable, "error": "未返回画像结果"}
		}
	}
	return map[string]any{
		"name":                table.Name,
		"rowCount":            rowCount,
		"columns":             columnProfiles,
		"columnLimit":         tableProfileMaxColumns,
		"profiledColumnCount": len(columnProfiles),
	}, sqls
}

// parseMentions 从用户问题中解析 @xxx 提及
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

func pickSingleTableForTool(schema *ai.SchemaContext, tableName string, tableNames []string, userQuestion string, mentions []string) (ai.TableSchema, bool) {
	if strings.TrimSpace(tableName) != "" {
		if table, ok := findTableByName(schema, tableName); ok {
			return table, true
		}
	}
	targets := pickTargetTablesWithArgs(schema, tableNames, userQuestion, mentions, 1)
	if len(targets) == 0 {
		return ai.TableSchema{}, false
	}
	return targets[0], true
}

func findTableByName(schema *ai.SchemaContext, tableName string) (ai.TableSchema, bool) {
	if schema == nil {
		return ai.TableSchema{}, false
	}
	want := strings.ToLower(strings.TrimSpace(tableName))
	for _, table := range schema.Tables {
		if strings.ToLower(table.Name) == want {
			return table, true
		}
	}
	return ai.TableSchema{}, false
}

func pickProfileColumns(table ai.TableSchema, requested []string) []ai.ColumnSchema {
	limit := tableProfileMaxColumns
	if limit <= 0 {
		limit = len(table.Columns)
	}
	if len(requested) > 0 {
		byName := make(map[string]ai.ColumnSchema, len(table.Columns))
		for _, col := range table.Columns {
			byName[strings.ToLower(col.Name)] = col
		}
		var picked []ai.ColumnSchema
		seen := map[string]bool{}
		for _, raw := range requested {
			key := strings.ToLower(strings.TrimSpace(raw))
			if key == "" || seen[key] {
				continue
			}
			if col, ok := byName[key]; ok {
				picked = append(picked, col)
				seen[key] = true
			}
			if len(picked) >= limit {
				break
			}
		}
		if len(picked) > 0 {
			return picked
		}
	}
	if len(table.Columns) <= limit {
		return table.Columns
	}
	return table.Columns[:limit]
}

func (s *AIService) executeToolQuery(ctx context.Context, connID, dbName, sqlStr string, pageSize int) (*database.QueryResult, error) {
	if s.query == nil {
		return nil, fmt.Errorf("QueryService 未注入，无法执行查询")
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	result, err := s.query.ExecuteSQLPagedContext(ctx, connID, dbName, sqlStr, 1, pageSize)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("执行结果为空")
	}
	if strings.TrimSpace(result.Error) != "" {
		return nil, errors.New(result.Error)
	}
	return result, nil
}

func jsonToolResult(toolName, sqlStr string, begin time.Time, payload any) aiToolExecutionResult {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"ok":false,"error":%q}`, err.Error()))
	}
	return aiToolExecutionResult{
		ToolName:   toolName,
		ToolSQL:    sqlStr,
		ToolOutput: string(data),
		DurationMs: time.Since(begin).Milliseconds(),
	}
}

func clampInt(value, defaultValue, minValue, maxValue int) int {
	if value <= 0 {
		value = defaultValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func queryResultColumnNames(result *database.QueryResult) []string {
	if result == nil {
		return nil
	}
	columns := make([]string, 0, len(result.Columns))
	for _, col := range result.Columns {
		if strings.TrimSpace(col.Name) != "" {
			columns = append(columns, col.Name)
		}
	}
	if len(columns) == 0 && len(result.Rows) > 0 {
		for key := range result.Rows[0] {
			columns = append(columns, key)
		}
		sort.Strings(columns)
	}
	return columns
}

func buildExplainSQL(sqlStr string) string {
	clean := strings.TrimRight(strings.TrimSpace(sqlStr), "; \t\r\n")
	if strings.EqualFold(database.SQLLeadingVerb(clean), "explain") {
		return clean
	}
	return "EXPLAIN " + clean
}

func stripExplainPrefix(sqlStr string) string {
	clean := strings.TrimRight(strings.TrimSpace(sqlStr), "; \t\r\n")
	fields := strings.Fields(clean)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "explain") {
		return clean
	}
	rest := strings.TrimSpace(clean[len(fields[0]):])
	lower := strings.ToLower(rest)
	if strings.HasPrefix(lower, "query plan ") {
		return strings.TrimSpace(rest[len("query plan "):])
	}
	if strings.HasPrefix(lower, "format=json ") {
		return strings.TrimSpace(rest[len("format=json "):])
	}
	return rest
}

func isNumericColumnType(columnType string) bool {
	t := strings.ToLower(columnType)
	numericMarkers := []string{"int", "decimal", "numeric", "number", "float", "double", "real", "serial"}
	for _, marker := range numericMarkers {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

func firstRowValue(row map[string]interface{}, name string) interface{} {
	if row == nil {
		return nil
	}
	if val, ok := row[name]; ok {
		return val
	}
	want := strings.ToLower(name)
	for key, val := range row {
		if strings.ToLower(key) == want {
			return val
		}
	}
	return nil
}

func asInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case []byte:
		var n int64
		_, _ = fmt.Sscan(string(v), &n)
		return n
	case string:
		var n int64
		_, _ = fmt.Sscan(v, &n)
		return n
	default:
		return 0
	}
}

func normalizeToolName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
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
					"required":             []string{"keywords", "limit"},
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
					"required":             []string{"table_names"},
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
					"required":             []string{"sql", "table_names", "limit"},
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
					"required":             []string{"table_names"},
					"additionalProperties": false,
				},
			})
		case "table_stats":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "查看指定表统计信息（行数、大小等）。单次最多传 20 张表；后端最多 4 并发获取统计；如果要看多张表，请尽量一次性放入 table_names。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_names": map[string]any{
							"type":     "array",
							"items":    map[string]any{"type": "string"},
							"maxItems": tableStatsMaxTables,
						},
					},
					"required":             []string{"table_names"},
					"additionalProperties": false,
				},
			})
		case "table_sample":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "安全抽样查看指定表前 N 行。只接受 table_name 和 limit，后端只生成 SELECT * FROM <table> LIMIT n，不接受 WHERE。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_name": map[string]any{"type": "string"},
						"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": tableSampleMaxRows},
					},
					"required":             []string{"table_name", "limit"},
					"additionalProperties": false,
				},
			})
		case "table_profile":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "查看表字段画像：row count、null count、distinct count，数字列补 min/max。单次最多 3 张表、每表最多 8 列；后端字段统计最多 3 并发。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_names": map[string]any{
							"type":     "array",
							"items":    map[string]any{"type": "string"},
							"maxItems": tableProfileMaxTables,
						},
						"columns": map[string]any{
							"type":     "array",
							"items":    map[string]any{"type": "string"},
							"maxItems": tableProfileMaxColumns,
						},
					},
					"required":             []string{"table_names", "columns"},
					"additionalProperties": false,
				},
			})
		case "sql_explain_plan":
			defs = append(defs, ai.FunctionToolDefinition{
				Name:        tool,
				Description: "为单条只读 SQL 生成 EXPLAIN 执行计划。拒绝写操作、多语句和 EXPLAIN ANALYZE。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sql": map[string]any{"type": "string"},
					},
					"required":             []string{"sql"},
					"additionalProperties": false,
				},
			})
		}
	}
	return defs
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
	args.TableName = strings.TrimSpace(extractString(payload, []string{"table_name", "tableName", "table"}))
	args.TableNames = normalizeStringSlice(extractStringArray(payload, []string{"table_names", "tableNames", "tables"}))
	if args.TableName != "" && len(args.TableNames) == 0 {
		args.TableNames = []string{args.TableName}
	}
	args.Columns = normalizeStringSlice(extractStringArray(payload, []string{"columns", "column_names", "columnNames"}))
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
