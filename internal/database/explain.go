package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"minidb/internal/model"
)

// Explainer 由能生成执行计划的方言实现（Caps().Explain 为 true）。
// sqlText 是已经去掉 EXPLAIN 前缀和结尾分号的单条语句。
// 实现要尽量降级而不是报错：拿不到树就返回 Raw 和原始表格，只有语句本身有错时才返回 error。
type Explainer interface {
	Explain(ctx context.Context, db Executor, sqlText string, analyze bool) (*model.ExplainResult, error)
}

// ExplainQuery 为一条语句生成执行计划。analyze 为 true 时数据库会真实执行语句，
// 调用方必须先确认语句是只读的（见 app.ExplainService）。
func ExplainQuery(ctx context.Context, db Executor, dbType DBType, sqlText string, analyze bool) (*model.ExplainResult, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	ex, ok := d.(Explainer)
	if !ok || !d.Caps().Explain {
		return nil, model.Errf(model.CodeExplainUnsupported, "%s 不支持执行计划", d.Label())
	}
	if analyze && !d.Caps().ExplainAnalyze {
		return nil, model.Errf(model.CodeExplainAnalyzeUnsupported, "%s 不支持 EXPLAIN ANALYZE", d.Label())
	}
	res, err := ex.Explain(ctx, db, StripExplainPrefix(sqlText), analyze)
	if err != nil {
		return nil, err
	}
	res.Analyzed = analyze
	return res, nil
}

// StripExplainPrefix 去掉语句开头已有的 EXPLAIN [ANALYZE]（以及 SQLite 的 QUERY PLAN）和结尾的分号。
func StripExplainPrefix(sqlText string) string {
	s := strings.TrimRight(strings.TrimSpace(sqlText), "; \t\r\n")
	for {
		fields := strings.Fields(s)
		if len(fields) == 0 {
			return s
		}
		skip := 0
		switch strings.ToLower(fields[0]) {
		case "explain":
			skip = 1
			if len(fields) > 1 && strings.EqualFold(fields[1], "analyze") {
				skip = 2
			} else if len(fields) > 2 && strings.EqualFold(fields[1], "query") && strings.EqualFold(fields[2], "plan") {
				skip = 3
			}
		default:
			return s
		}
		// 去掉前 skip 个词（保持后面语句里的空白不变）
		rest := s
		for i := 0; i < skip; i++ {
			rest = strings.TrimLeft(rest, " \t\r\n")
			rest = rest[len(fields[i]):]
		}
		s = strings.TrimSpace(rest)
	}
}

// fetchTable 执行语句并把结果读成文本表格（NULL 为空串）。
func fetchTable(ctx context.Context, db Executor, query string) (cols []string, rows [][]string, err error) {
	rs, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	cols, err = rs.Columns()
	if err != nil {
		return nil, nil, err
	}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		cells := make([]string, len(cols))
		for i, v := range vals {
			switch x := v.(type) {
			case nil:
			case []byte:
				cells[i] = string(x)
			default:
				cells[i] = fmt.Sprint(x)
			}
		}
		rows = append(rows, cells)
	}
	return cols, rows, rs.Err()
}

// tableResult 把原始表格包成「没解析成树」的结果；Raw 是把各行按制表符拼起来的文本。
func tableResult(query string, cols []string, rows [][]string, note string) *model.ExplainResult {
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Join(r, "\t"))
	}
	return &model.ExplainResult{SQL: query, Columns: cols, Rows: rows, Raw: b.String(), Note: note}
}

// textResult 是只有一列文本（每行一条）的计划，如 MySQL 的 FORMAT=TREE。
func textResult(query string, cols []string, rows [][]string) (text string, res *model.ExplainResult) {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		if len(r) > 0 {
			lines = append(lines, r[0])
		}
	}
	text = strings.Join(lines, "\n")
	return text, &model.ExplainResult{SQL: query, Raw: text, Columns: cols, Rows: rows}
}

// parseNum 读一个数字，读不出为 0。能容忍千分位逗号和 "1.5e+3" 之类写法。
func parseNum(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	return f
}
