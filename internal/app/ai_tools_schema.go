package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/schemaindex"
)

// tableListTool 列出库里的表，可按关键词过滤并按相关度排序。
func (e *toolEnv) tableListTool() agent.Tool {
	return agent.Tool{
		Name: "table_list",
		Description: "列出库里的表（表名、注释、列数）。想了解库里有什么、或总览没列全（表很多时）时使用。" +
			"keyword 可选：按表名和注释过滤并按相关度排序，不传则按表名顺序列出。想按含义找列、或不确定数据存在哪张表，用 schema_search。",
		Parameters: agent.Object(map[string]agent.Prop{
			"keyword": agent.Optional(agent.String("只列表名或注释与它相关的表")),
			"limit":   agent.Optional(agent.Integer("最多返回几张表，默认 100", 1, tableListMaxRows)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			limit := clampInt(in.Int("limit"), 100, 1, tableListMaxRows)
			keyword := in.String("keyword")
			var hits []tableRow
			total := 0
			if keyword == "" {
				total = len(e.schema.Tables)
				for _, t := range e.schema.Tables {
					hits = append(hits, tableRow{t.Name, t.Comment, len(t.Columns)})
				}
				sort.SliceStable(hits, func(i, j int) bool { return hits[i].name < hits[j].name })
			} else {
				all := e.search.Tables([]string{keyword}, 0)
				if len(all) == 0 {
					all = e.substringTables(keyword)
				}
				total = len(all)
				for _, h := range all {
					hits = append(hits, tableRow{h.Name, h.Comment, h.Columns})
				}
			}
			if len(hits) > limit {
				hits = hits[:limit]
			}
			return tableListResult(keyword, total, hits)
		},
	}
}

// tableRow 表清单里的一行。
type tableRow struct {
	name, comment string
	columns       int
}

// substringTables 检索没命中时的兜底：表名或注释包含关键词的表。
func (e *toolEnv) substringTables(keyword string) []schemaindex.TableHit {
	kw := strings.ToLower(keyword)
	var out []schemaindex.TableHit
	for _, t := range e.schema.Tables {
		if strings.Contains(strings.ToLower(t.Name), kw) || strings.Contains(strings.ToLower(t.Comment), kw) {
			out = append(out, schemaindex.TableHit{Name: t.Name, Comment: t.Comment, Columns: len(t.Columns)})
		}
	}
	return out
}

// tableListResult 组装表清单结果；没有匹配时告诉模型换条路。
func tableListResult(keyword string, total int, rows []tableRow) *agent.ToolResult {
	if len(rows) == 0 {
		res := agent.Text("没有匹配的表", fmt.Sprintf("没有与 %q 相关的表", keyword))
		res.ModelText = res.Text + "。可以换同义词或英文名，或不带 keyword 列出全部表，或用 schema_search 按列名查找。"
		return res
	}
	var model strings.Builder
	head := fmt.Sprintf("共 %d 张表", total)
	if keyword != "" {
		head = fmt.Sprintf("与 %q 相关的表共 %d 张", keyword, total)
	}
	if total > len(rows) {
		head += fmt.Sprintf("，仅展示前 %d 张（加 keyword 缩小范围）", len(rows))
	}
	model.WriteString(head + "\n")
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		line := "- " + r.name
		if r.comment != "" {
			line += "（" + r.comment + "）"
		}
		model.WriteString(fmt.Sprintf("%s · %d 列\n", line, r.columns))
		data = append(data, map[string]any{"表": r.name, "注释": r.comment, "列数": r.columns})
	}
	res := agent.Rows([]string{"表", "注释", "列数"}, data, int64(total), fmt.Sprintf("%d 张表", total))
	res.ModelText = strings.TrimSpace(model.String())
	return res
}

// schemaSearchTool 在表名、列名及其注释里检索，同时返回相关的表和列。
func (e *toolEnv) schemaSearchTool() agent.Tool {
	return agent.Tool{
		Name: "schema_search",
		Description: "按关键词在表名、表注释、列名、列注释中检索，同时返回相关的表和列（支持中英文，如 订单金额、paid_at）。" +
			"不确定数据在哪张表、哪一列时先用它。keywords 传 1-5 个词，可同时给中文和英文同义词；结果按相关度排序。",
		Parameters: agent.Object(map[string]agent.Prop{
			"keywords": agent.StringList("检索关键词", 5),
			"limit":    agent.Optional(agent.Integer("表和列各最多返回几条，默认 10", 1, schemaSearchMaxItems)),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			keywords := in.Strings("keywords")
			if len(keywords) == 0 {
				return agent.Fail("invalid_arguments", "keywords 不能为空", "传入 1-5 个关键词")
			}
			limit := clampInt(in.Int("limit"), 10, 1, schemaSearchMaxItems)
			tables := e.search.Tables(keywords, limit)
			columns := e.search.Columns(keywords, nil, limit)
			if len(tables) == 0 && len(columns) == 0 {
				res := agent.Text("没有匹配", fmt.Sprintf("没有找到与 %s 相关的表或列", strings.Join(keywords, "、")))
				res.ModelText = res.Text + "。换同义词、英文名或更短的词再试；或用 table_list 浏览全部表。"
				return res
			}
			return schemaSearchResult(tables, columns)
		},
	}
}

// schemaSearchResult 把表与列的命中项组装成结果。
func schemaSearchResult(tables []schemaindex.TableHit, columns []schemaindex.ColumnHit) *agent.ToolResult {
	var model strings.Builder
	var rows []map[string]any
	if len(tables) > 0 {
		model.WriteString("匹配的表:\n")
		for _, t := range tables {
			model.WriteString(fmt.Sprintf("- %s%s · %d 列\n", t.Name, commentSuffix(t.Comment), t.Columns))
			rows = append(rows, map[string]any{"对象": t.Name, "种类": "表", "数据类型": "", "注释": t.Comment})
		}
	}
	if len(columns) > 0 {
		model.WriteString("匹配的列:\n")
		for _, c := range columns {
			model.WriteString(fmt.Sprintf("- %s.%s %s%s\n", c.Table, c.Column, c.Type, commentSuffix(c.Comment)))
			rows = append(rows, map[string]any{"对象": c.Table + "." + c.Column, "种类": "列", "数据类型": c.Type, "注释": c.Comment})
		}
	}
	res := agent.Rows([]string{"对象", "种类", "数据类型", "注释"}, rows, int64(len(rows)), fmt.Sprintf("%d 张表 · %d 列", len(tables), len(columns)))
	res.ModelText = strings.TrimSpace(model.String())
	return res
}

// commentSuffix 把注释写成 "（注释）" 形式；没有注释返回空串。
func commentSuffix(comment string) string {
	if comment == "" {
		return ""
	}
	return "（" + comment + "）"
}

// columnLine 一列的单行描述：名称 类型 [PK] [NOT NULL] [DEFAULT x] [→ 外键] [「注释」]。
func columnLine(c ai.ColumnSchema) string {
	var sb strings.Builder
	sb.WriteString(c.Name + " " + c.Type)
	if c.IsPrimary {
		sb.WriteString(" PK")
	}
	if !c.Nullable {
		sb.WriteString(" NOT NULL")
	}
	if c.DefaultValue != "" {
		sb.WriteString(" DEFAULT " + c.DefaultValue)
	}
	if c.ForeignKey != "" {
		sb.WriteString(" → " + c.ForeignKey)
	}
	if c.Comment != "" {
		sb.WriteString(" 「" + c.Comment + "」")
	}
	return sb.String()
}
