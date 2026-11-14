package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/database"
)

// tableStatsTool 查看表的行数、大小与引擎（取自数据库统计信息）。
func (e *toolEnv) tableStatsTool() agent.Tool {
	return agent.Tool{
		Name: "table_stats",
		Description: "查看表的行数、占用空间和存储引擎（来自数据库的统计信息，行数可能是估算值）。" +
			"一次最多 20 张表，请把需要的表一次传完。要精确行数用 sql_readonly_execute 跑 COUNT(*)。",
		Parameters: agent.Object(map[string]agent.Prop{
			"table_names": agent.StringList("要查看的表名", statsMaxTables),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			names := in.Strings("table_names")
			if len(names) == 0 {
				return agent.Fail("invalid_arguments", "table_names 不能为空", "传入表名，可先用 table_list 查看有哪些表")
			}
			tables, missing := e.resolveTables(names, statsMaxTables)
			if len(tables) == 0 {
				return e.tablesNotFound(missing)
			}
			dbs := NewDatabaseService(e.s.manager, e.s.schema)
			type stat struct {
				rows, size int64
				engine     string
				err        error
			}
			stats := make([]stat, len(tables))
			sem := make(chan struct{}, statsConcurrency)
			var wg sync.WaitGroup
			for i, t := range tables {
				wg.Add(1)
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					if err := ctx.Err(); err != nil {
						stats[i].err = err
						return
					}
					s, err := dbs.GetTableStats(e.connID, e.dbName, t.Name)
					if err != nil || s == nil {
						stats[i].err = fmt.Errorf("读取失败: %v", err)
						return
					}
					stats[i] = stat{rows: s.RowCount, size: s.TotalSize, engine: s.Engine}
				}()
			}
			wg.Wait()
			var model strings.Builder
			var rows []map[string]any
			for i, t := range tables {
				if stats[i].err != nil {
					model.WriteString(fmt.Sprintf("- %s: %v\n", t.Name, stats[i].err))
					continue
				}
				model.WriteString(fmt.Sprintf("- %s: %d 行 · %s · %s\n", t.Name, stats[i].rows, humanBytes(stats[i].size), stats[i].engine))
				rows = append(rows, map[string]any{"表": t.Name, "行数": stats[i].rows, "大小": humanBytes(stats[i].size), "引擎": stats[i].engine})
			}
			res := agent.Rows([]string{"表", "行数", "大小", "引擎"}, rows, int64(len(rows)), fmt.Sprintf("%d 张表", len(rows)))
			res.ModelText = strings.TrimSpace(model.String()) + e.missingNote(missing)
			return res
		},
	}
}

// tableProfileTool 查看表的字段画像：行数、空值、不同值与数值范围。
func (e *toolEnv) tableProfileTool() agent.Tool {
	return agent.Tool{
		Name: "table_profile",
		Description: "查看表的字段画像：总行数、各列的空值数、不同值个数，数字列还有最小/最大值。用于判断列的质量与分布。" +
			"一次最多 3 张表，每表最多 8 列（用 columns 指定想看的列，不传则取前 8 列）；行数超过 10 万的表不统计不同值。",
		Parameters: agent.Object(map[string]agent.Prop{
			"table_names": agent.StringList("要分析的表名", profileMaxTables),
			"columns":     agent.Optional(agent.StringList("只分析这些列", profileMaxColumns)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			names := in.Strings("table_names")
			if len(names) == 0 {
				return agent.Fail("invalid_arguments", "table_names 不能为空", "传入表名，可先用 table_list 查看有哪些表")
			}
			tables, missing := e.resolveTables(names, profileMaxTables)
			if len(tables) == 0 {
				return e.tablesNotFound(missing)
			}
			var model strings.Builder
			var rows []map[string]any
			var sqls []string
			for _, t := range tables {
				if ctx.Err() != nil {
					return agent.Fail("cancelled", "已取消", "")
				}
				prof := e.profileTable(ctx, t, in.Strings("columns"))
				model.WriteString(prof.model)
				rows = append(rows, prof.rows...)
				sqls = append(sqls, prof.sqls...)
			}
			res := agent.Rows([]string{"表", "列", "类型", "空值数", "不同值数", "最小值", "最大值"}, rows, int64(len(rows)), fmt.Sprintf("%d 张表 · %d 列", len(tables), len(rows)))
			res.ModelText = strings.TrimSpace(model.String()) + e.missingNote(missing)
			res.SQL = strings.Join(sqls, ";\n")
			return res
		},
	}
}

// tableProfile 一张表的画像结果。
type tableProfile struct {
	model string
	rows  []map[string]any
	sqls  []string
}

// profileTable 统计一张表：先数总行数，再并发统计每列（并发度有上限，避免占满连接池）。
func (e *toolEnv) profileTable(ctx context.Context, t ai.TableSchema, requested []string) tableProfile {
	dbType := e.dbType()
	qt := database.QuoteTableName(dbType, t.Name)
	countSQL := "SELECT COUNT(*) AS row_count FROM " + qt
	prof := tableProfile{sqls: []string{countSQL}}
	var rowCount int64
	if r, fail := e.runQuery(ctx, countSQL, 1); fail == nil && len(r.Rows) > 0 {
		rowCount = toInt64(cell(r.Rows[0], "row_count"))
	}
	cols := profileColumns(t, requested)
	lines := make([]string, len(cols))
	rows := make([]map[string]any, len(cols))
	sqls := make([]string, len(cols))
	sem := make(chan struct{}, profileConcurrency)
	var wg sync.WaitGroup
	for i, c := range cols {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			lines[i], rows[i], sqls[i] = e.profileColumn(ctx, t.Name, qt, c, rowCount)
		}()
	}
	wg.Wait()
	prof.model = fmt.Sprintf("## %s（共 %d 行）\n%s\n\n", t.Name, rowCount, strings.Join(lines, "\n"))
	prof.rows = rows
	for _, s := range sqls {
		if s != "" {
			prof.sqls = append(prof.sqls, s)
		}
	}
	return prof
}

// profileColumn 统计一列：空值数、不同值数（大表跳过）、数字列的最小/最大值。
func (e *toolEnv) profileColumn(ctx context.Context, table, qt string, c ai.ColumnSchema, rowCount int64) (string, map[string]any, string) {
	q := database.QuoteIdent(e.dbType(), c.Name)
	metrics := []string{fmt.Sprintf("SUM(CASE WHEN %s IS NULL THEN 1 ELSE 0 END) AS null_count", q)}
	distinct := rowCount <= profileDistinctLimit
	if distinct {
		metrics = append(metrics, fmt.Sprintf("COUNT(DISTINCT %s) AS distinct_count", q))
	}
	numeric := isNumericType(c.Type)
	if numeric {
		metrics = append(metrics, fmt.Sprintf("MIN(%s) AS min_value", q), fmt.Sprintf("MAX(%s) AS max_value", q))
	}
	sqlText := fmt.Sprintf("SELECT %s FROM %s", strings.Join(metrics, ", "), qt)
	row := map[string]any{"表": table, "列": c.Name, "类型": c.Type}
	r, fail := e.runQuery(ctx, sqlText, 1)
	if fail != nil || len(r.Rows) == 0 {
		msg := "统计失败"
		if fail != nil {
			msg += ": " + fail.Text
		}
		return fmt.Sprintf("- %s %s: %s", c.Name, c.Type, msg), row, sqlText
	}
	stat := r.Rows[0]
	row["空值数"] = cell(stat, "null_count")
	parts := []string{fmt.Sprintf("空值 %v", cell(stat, "null_count"))}
	if distinct {
		row["不同值数"] = cell(stat, "distinct_count")
		parts = append(parts, fmt.Sprintf("不同值 %v", cell(stat, "distinct_count")))
	} else {
		parts = append(parts, fmt.Sprintf("不同值: 已跳过（行数超过 %d）", profileDistinctLimit))
	}
	if numeric {
		row["最小值"], row["最大值"] = cell(stat, "min_value"), cell(stat, "max_value")
		parts = append(parts, fmt.Sprintf("范围 %v ~ %v", cell(stat, "min_value"), cell(stat, "max_value")))
	}
	return fmt.Sprintf("- %s %s: %s", c.Name, c.Type, strings.Join(parts, ", ")), row, sqlText
}

// profileColumns 选出要统计的列：指定了就按指定的（忽略不存在的），否则取前几列。
func profileColumns(t ai.TableSchema, requested []string) []ai.ColumnSchema {
	if len(requested) > 0 {
		var picked []ai.ColumnSchema
		for _, want := range requested {
			for _, c := range t.Columns {
				if strings.EqualFold(c.Name, want) && len(picked) < profileMaxColumns {
					picked = append(picked, c)
				}
			}
		}
		if len(picked) > 0 {
			return picked
		}
	}
	return t.Columns[:min(len(t.Columns), profileMaxColumns)]
}

// isNumericType 判断列类型是否为数字（决定是否统计最小/最大值）。
func isNumericType(typ string) bool {
	t := strings.ToLower(typ)
	for _, marker := range []string{"int", "decimal", "numeric", "number", "float", "double", "real", "serial"} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

// cell 按列名取值，列名大小写不敏感（部分数据库会改写别名大小写）。
func cell(row map[string]any, name string) any {
	if v, ok := row[name]; ok {
		return v
	}
	for k, v := range row {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return nil
}

// toInt64 把驱动返回的数字（各种整型、浮点、字节串、字符串）转成 int64。
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		x, _ := strconv.ParseInt(strings.TrimSpace(string(n)), 10, 64)
		return x
	case string:
		x, _ := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return x
	}
	return 0
}

// humanBytes 把字节数写成 KB/MB/GB。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
