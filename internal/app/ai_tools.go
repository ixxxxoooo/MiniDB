package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/schemaindex"
)

// 工具的数量与体量上限。上限同时写进工具描述，模型据此一次把活干完，不必为凑数多绕几轮。
const (
	maxToolRounds         = 10 // 一次回答最多调用几轮工具
	tableListMaxRows      = 300
	schemaSearchMaxItems  = 30
	describeMaxTables     = 5
	statsMaxTables        = 20
	profileMaxTables      = 3
	profileMaxColumns     = 8
	sampleMaxRows         = 100
	valuesMaxRows         = 50
	sqlMaxRows            = 200
	statsConcurrency      = 6
	profileConcurrency    = 5
	profileDistinctLimit  = 100000 // 行数超过它的表跳过 COUNT(DISTINCT)，避免全表扫描
	statementTimeout      = 30 * time.Second
	describeMaxRelations  = 30
	suggestionsPerMissing = 3
)

// toolEnv 一次 run 里工具共享的上下文：连接、库、表结构快照与检索器。
//
// @author ixxxxoooo
type toolEnv struct {
	s      *AIService
	connID string
	dbName string
	schema *ai.SchemaContext
	search *schemaindex.Searcher
}

// registry 构建本次 run 的工具注册表。工具清单、描述与参数 schema 只在这里定义一次，
// 发给模型的函数定义和界面的工具列表都由它派生。
func (e *toolEnv) registry() *agent.ToolRegistry {
	reg := agent.NewToolRegistry()
	for _, t := range []agent.Tool{
		e.tableListTool(), e.schemaSearchTool(), e.tableDescribeTool(), e.tableStatsTool(),
		e.tableSampleTool(), e.columnValuesTool(), e.tableProfileTool(),
		e.sqlExecuteTool(), e.sqlExplainTool(),
	} {
		reg.MustRegister(t)
	}
	return reg
}

// toolDefinitions 把注册表里的工具转成发给模型的函数定义。
func toolDefinitions(reg *agent.ToolRegistry) []ai.FunctionToolDefinition {
	tools := reg.List()
	defs := make([]ai.FunctionToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, ai.FunctionToolDefinition{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	return defs
}

// resolveTables 按名字取表（不区分大小写），返回找到的表和没找到的名字；最多取 max 张。
func (e *toolEnv) resolveTables(names []string, max int) (found []ai.TableSchema, missing []string) {
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if t, ok := e.search.Table(name); ok {
			if len(found) < max {
				found = append(found, t)
			}
		} else {
			missing = append(missing, name)
		}
	}
	return found, missing
}

// tablesNotFound 构造"表不存在"的失败结果，并附上拼写相近或检索相关的表名，让模型下一次能改对。
func (e *toolEnv) tablesNotFound(missing []string) *agent.ToolResult {
	var parts []string
	for _, name := range missing {
		line := fmt.Sprintf("%q", name)
		if sug := e.search.Suggest(name, suggestionsPerMissing); len(sug) > 0 {
			line += "（可能想找: " + strings.Join(sug, ", ") + "）"
		}
		parts = append(parts, line)
	}
	return agent.Fail("table_not_found", "表不存在: "+strings.Join(parts, "；"), "用 table_list 或 schema_search 查到正确的表名后重试")
}

// missingNote 部分表不存在时附在成功结果后面的提醒，避免模型误以为所有表都查到了。
func (e *toolEnv) missingNote(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	return "\n注意：以下表不存在，已忽略: " + strings.Join(missing, ", ")
}

// runQuery 在只读事务里执行一条查询，带语句超时。成功返回结果；失败返回给模型的失败结果。
func (e *toolEnv) runQuery(ctx context.Context, sqlText string, pageSize int) (*database.QueryResult, *agent.ToolResult) {
	if e.s.query == nil {
		return nil, agent.Fail("internal", "查询服务不可用", "")
	}
	ctx, cancel := context.WithTimeout(ctx, statementTimeout)
	defer cancel()
	result, err := e.s.query.ExecuteReadOnlySQLContext(ctx, e.connID, e.dbName, sqlText, 1, pageSize)
	if err == nil && result != nil && strings.TrimSpace(result.Error) != "" {
		err = errors.New(result.Error)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return nil, agent.Fail("timeout", fmt.Sprintf("查询超过 %s 被中止", statementTimeout), "加 WHERE 条件、减少 JOIN 或改用聚合后重试")
	case errors.Is(err, context.Canceled):
		return nil, agent.Fail("cancelled", "已取消", "")
	case err != nil:
		return nil, agent.Fail("sql_error", err.Error(), "检查表名、列名与当前数据库方言（可用 table_describe 确认列名），修正后重试")
	case result == nil:
		return nil, agent.Fail("sql_error", "执行结果为空", "")
	}
	return result, nil
}

// queryColumns 取查询结果的列名；驱动没返回列元数据时退回到第一行的键（排序以保证稳定）。
func queryColumns(r *database.QueryResult) []string {
	cols := make([]string, 0, len(r.Columns))
	for _, c := range r.Columns {
		if strings.TrimSpace(c.Name) != "" {
			cols = append(cols, c.Name)
		}
	}
	if len(cols) == 0 && len(r.Rows) > 0 {
		for k := range r.Rows[0] {
			cols = append(cols, k)
		}
		sort.Strings(cols)
	}
	return cols
}

// rowsResult 把查询结果转成表格工具结果，摘要形如 "12 行 · 34 ms"。
func rowsResult(r *database.QueryResult, sqlText string) *agent.ToolResult {
	total := r.Total
	if total < int64(len(r.Rows)) {
		total = int64(len(r.Rows))
	}
	summary := fmt.Sprintf("%d 行", len(r.Rows))
	if total > int64(len(r.Rows)) {
		summary = fmt.Sprintf("%d / %d 行", len(r.Rows), total)
	}
	res := agent.Rows(queryColumns(r), r.Rows, total, summary)
	res.SQL = sqlText
	return res
}

// clampInt 把 value 夹到 [lo, hi]；value<=0 视为没给，取 def。
func clampInt(value, def, lo, hi int) int {
	if value <= 0 {
		value = def
	}
	return max(lo, min(hi, value))
}

// cutRunes 按字符数截断字符串，超出时以省略号收尾。
func cutRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// columnNames 列出表的列名（最多 n 个，超出用省略收尾），用于"列不存在"时提示可用的列。
func columnNames(cols []ai.ColumnSchema, n int) string {
	names := make([]string, 0, min(len(cols), n))
	for i, c := range cols {
		if i >= n {
			names = append(names, fmt.Sprintf("…共 %d 列", len(cols)))
			break
		}
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}
