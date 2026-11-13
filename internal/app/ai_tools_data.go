package app

import (
	"context"
	"fmt"
	"strings"

	"minidb/internal/agent"
	"minidb/internal/database"
)

// sqlExecuteTool 执行一条只读 SQL。
func (e *toolEnv) sqlExecuteTool() agent.Tool {
	return agent.Tool{
		Name: "sql_readonly_execute",
		Description: "执行一条只读 SQL（SELECT / WITH / SHOW / DESCRIBE / EXPLAIN）并返回结果。只能单条语句，写操作会被拒绝。" +
			"结果默认最多 50 行，用 limit 调整；大表请加 WHERE 或聚合。SQL 必须符合当前数据库方言，表名和列名必须来自总览或工具返回。",
		Parameters: agent.Object(map[string]agent.Prop{
			"sql":   agent.String("要执行的单条只读 SQL"),
			"limit": agent.Optional(agent.Integer("最多返回几行，默认 50", 1, sqlMaxRows)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			sqlText := in.String("sql")
			if denied := agent.GuardReadOnlySQL(sqlText); denied != nil {
				denied.SQL = sqlText
				return denied
			}
			result, fail := e.runQuery(ctx, sqlText, clampInt(in.Int("limit"), 50, 1, sqlMaxRows))
			if fail != nil {
				fail.SQL = sqlText
				return fail
			}
			return rowsResult(result, sqlText)
		},
	}
}

// tableSampleTool 抽样查看一张表的前 N 行。
func (e *toolEnv) tableSampleTool() agent.Tool {
	return agent.Tool{
		Name: "table_sample",
		Description: "查看一张表的前 N 行样例，用来了解数据长什么样。只接受 table_name 和 limit，不支持过滤条件；" +
			"需要条件、排序或指定列时用 sql_readonly_execute。",
		Parameters: agent.Object(map[string]agent.Prop{
			"table_name": agent.String("表名"),
			"limit":      agent.Optional(agent.Integer("返回几行，默认 10", 1, sampleMaxRows)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			table, fail := e.oneTable(in.String("table_name"))
			if fail != nil {
				return fail
			}
			limit := clampInt(in.Int("limit"), 10, 1, sampleMaxRows)
			sqlText := fmt.Sprintf("SELECT * FROM %s LIMIT %d", database.QuoteTableName(e.schema.DatabaseType, table), limit)
			result, fail := e.runQuery(ctx, sqlText, limit)
			if fail != nil {
				fail.SQL = sqlText
				return fail
			}
			res := rowsResult(result, sqlText)
			res.Summary = fmt.Sprintf("%s · %d 行", table, len(result.Rows))
			return res
		},
	}
}

// columnValuesTool 返回一列最常见的取值及次数。
func (e *toolEnv) columnValuesTool() agent.Tool {
	return agent.Tool{
		Name: "column_values",
		Description: "查看一张表某一列最常见的取值及出现次数（按次数降序）。写 WHERE 条件前用它确认状态码、枚举、类型字段的真实写法" +
			"（是“已支付”还是 paid 还是 2），不要猜。",
		Parameters: agent.Object(map[string]agent.Prop{
			"table_name": agent.String("表名"),
			"column":     agent.String("列名"),
			"limit":      agent.Optional(agent.Integer("最多返回几个取值，默认 20", 1, valuesMaxRows)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			name, fail := e.oneTable(in.String("table_name"))
			if fail != nil {
				return fail
			}
			tbl, _ := e.search.Table(name)
			column := ""
			for _, c := range tbl.Columns {
				if strings.EqualFold(c.Name, in.String("column")) {
					column = c.Name
				}
			}
			if column == "" {
				return agent.Fail("column_not_found", fmt.Sprintf("表 %s 没有列 %q", tbl.Name, in.String("column")),
					"该表的列: "+columnNames(tbl.Columns, 30))
			}
			limit := clampInt(in.Int("limit"), 20, 1, valuesMaxRows)
			q := database.QuoteIdent(e.schema.DatabaseType, column)
			sqlText := fmt.Sprintf("SELECT %s AS value, COUNT(*) AS cnt FROM %s GROUP BY %s ORDER BY cnt DESC LIMIT %d",
				q, database.QuoteTableName(e.schema.DatabaseType, tbl.Name), q, limit)
			result, fail := e.runQuery(ctx, sqlText, limit)
			if fail != nil {
				fail.SQL = sqlText
				return fail
			}
			res := rowsResult(result, sqlText)
			res.Summary = fmt.Sprintf("%s.%s · %d 个取值", tbl.Name, column, len(result.Rows))
			return res
		},
	}
}

// sqlExplainTool 生成执行计划。
func (e *toolEnv) sqlExplainTool() agent.Tool {
	return agent.Tool{
		Name: "sql_explain_plan",
		Description: "为一条只读 SQL 生成 EXPLAIN 执行计划（不会真正执行查询），用于分析性能、判断索引是否生效。" +
			"不允许 EXPLAIN ANALYZE。",
		Parameters: agent.Object(map[string]agent.Prop{
			"sql": agent.String("要分析的单条只读 SQL，可以带或不带 EXPLAIN 前缀"),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			sqlText := in.String("sql")
			if denied := agent.GuardReadOnlySQL(sqlText); denied != nil {
				denied.SQL = sqlText
				return denied
			}
			if strings.Contains(strings.ToLower(sqlText), "explain analyze") {
				return agent.Fail("guarded", "不允许 EXPLAIN ANALYZE（它会真正执行查询）", "去掉 ANALYZE，用普通 EXPLAIN")
			}
			explain := buildExplainSQL(sqlText)
			result, fail := e.runQuery(ctx, explain, 100)
			if fail != nil {
				fail.SQL = explain
				return fail
			}
			res := rowsResult(result, explain)
			res.Summary = fmt.Sprintf("%d 行执行计划", len(result.Rows))
			return res
		},
	}
}

// oneTable 解析单表参数，返回表名；表不存在时返回带建议的失败结果。
func (e *toolEnv) oneTable(name string) (string, *agent.ToolResult) {
	if name == "" {
		return "", agent.Fail("invalid_arguments", "table_name 不能为空", "传入表名，可先用 table_list / schema_search 查找")
	}
	t, ok := e.search.Table(name)
	if !ok {
		return "", e.tablesNotFound([]string{name})
	}
	return t.Name, nil
}

// buildExplainSQL 给 SQL 加上 EXPLAIN 前缀（已有则保持），并去掉结尾分号。
func buildExplainSQL(sqlText string) string {
	clean := strings.TrimRight(strings.TrimSpace(sqlText), "; \t\r\n")
	if strings.EqualFold(database.SQLLeadingVerb(clean), "explain") {
		return clean
	}
	return "EXPLAIN " + clean
}
