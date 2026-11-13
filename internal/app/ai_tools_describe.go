package app

import (
	"context"
	"fmt"
	"strings"

	"minidb/internal/agent"
	"minidb/internal/ai"
)

// tableDescribeTool 查看表的完整结构：列、索引、外键与关联关系。
// 它合并了原来的"看列""看 DDL""看关联"三个工具：写 SQL 需要的结构信息一次拿全，省掉几轮往返。
func (e *toolEnv) tableDescribeTool() agent.Tool {
	return agent.Tool{
		Name: "table_describe",
		Description: "查看表的完整结构：列（类型、主键、可空、默认值、注释）、索引、外键与关联关系" +
			"（显式外键，以及按 *_id 命名推断的疑似关联，推断项必须用数据确认）。" +
			"写 SQL 或 JOIN 前、总览没列出某表的列时使用。一次最多 5 张表，请把需要的表一次传完。",
		Parameters: agent.Object(map[string]agent.Prop{
			"table_names": agent.StringList("要查看的表名", describeMaxTables),
		}),
		ReadOnly: true, ResultKind: "rows",
		Handler: func(ctx context.Context, in agent.ToolInput) *agent.ToolResult {
			names := in.Strings("table_names")
			if len(names) == 0 {
				return agent.Fail("invalid_arguments", "table_names 不能为空", "传入要查看的表名，可先用 table_list / schema_search 查找")
			}
			tables, missing := e.resolveTables(names, describeMaxTables)
			if len(tables) == 0 {
				return e.tablesNotFound(missing)
			}
			return e.describeResult(tables, missing)
		},
	}
}

// describeResult 组装结构结果：界面拿到逐列的表格和索引/关联说明，模型拿到紧凑文本。
func (e *toolEnv) describeResult(tables []ai.TableSchema, missing []string) *agent.ToolResult {
	byName := make(map[string]ai.TableSchema, len(e.schema.Tables))
	for _, t := range e.schema.Tables {
		byName[strings.ToLower(t.Name)] = t
	}
	var model, notes strings.Builder
	var rows []map[string]any
	for _, t := range tables {
		model.WriteString(fmt.Sprintf("## %s%s · %d 列\n列:\n", t.Name, commentSuffix(t.Comment), len(t.Columns)))
		for _, c := range t.Columns {
			model.WriteString("- " + columnLine(c) + "\n")
			rows = append(rows, map[string]any{
				"表": t.Name, "列": c.Name, "类型": c.Type, "键": keyMark(c), "可空": yesNo(c.Nullable),
				"默认值": c.DefaultValue, "外键": c.ForeignKey, "注释": c.Comment,
			})
		}
		extra := indexLines(t)
		extra = append(extra, e.relationLines(t, byName)...)
		if len(t.Indexes) == 0 {
			extra = append(extra, "索引: 无信息（可能没有二级索引，或无权限读取）")
		}
		for _, line := range extra {
			model.WriteString(line + "\n")
			notes.WriteString(t.Name + " " + line + "\n")
		}
		model.WriteString("\n")
	}
	res := agent.Rows([]string{"表", "列", "类型", "键", "可空", "默认值", "外键", "注释"}, rows, int64(len(rows)),
		fmt.Sprintf("%d 张表 · %d 列", len(tables), len(rows)))
	res.ModelText = strings.TrimSpace(model.String()) + e.missingNote(missing)
	res.Notes = strings.TrimSpace(notes.String())
	return res
}

// indexLines 一张表的索引说明，每个索引一行。
func indexLines(t ai.TableSchema) []string {
	var out []string
	for _, idx := range t.Indexes {
		kind := ""
		switch {
		case idx.Primary:
			kind = " 主键"
		case idx.Unique:
			kind = " 唯一"
		}
		out = append(out, fmt.Sprintf("索引: %s (%s)%s", idx.Name, strings.Join(idx.Columns, ", "), kind))
	}
	return out
}

// relationLines 一张表的关联关系：显式外键（正向与反向），以及按 *_id 命名推断的疑似关联。
func (e *toolEnv) relationLines(target ai.TableSchema, byName map[string]ai.TableSchema) []string {
	var lines []string
	seen := map[string]bool{}
	add := func(line string) {
		if !seen[line] && len(lines) < describeMaxRelations {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	for _, c := range target.Columns {
		if c.ForeignKey != "" {
			add(fmt.Sprintf("关联: %s.%s → %s（外键）", target.Name, c.Name, c.ForeignKey))
		} else if ref, ok := e.inferReference(c.Name, byName); ok && !strings.EqualFold(ref.Name, target.Name) {
			add(fmt.Sprintf("关联: %s.%s → %s.id（推断：仅依据命名，需确认）", target.Name, c.Name, ref.Name))
		}
	}
	for _, t := range e.schema.Tables {
		if strings.EqualFold(t.Name, target.Name) {
			continue
		}
		for _, c := range t.Columns {
			if refTable, _, _ := strings.Cut(c.ForeignKey, "."); c.ForeignKey != "" && strings.EqualFold(strings.TrimSpace(refTable), target.Name) {
				add(fmt.Sprintf("关联: %s.%s → %s（外键，反向）", t.Name, c.Name, c.ForeignKey))
			} else if c.ForeignKey == "" {
				if ref, ok := e.inferReference(c.Name, byName); ok && strings.EqualFold(ref.Name, target.Name) {
					add(fmt.Sprintf("关联: %s.%s → %s.id（推断，反向：仅依据命名，需确认）", t.Name, c.Name, target.Name))
				}
			}
		}
	}
	return lines
}

// inferReference 由 xxx_id 这样的列名推断它指向的表（xxx、xxxs、xxxes、xxxies，或去掉 t_/tbl_ 等前缀后的同名表）。
func (e *toolEnv) inferReference(column string, byName map[string]ai.TableSchema) (ai.TableSchema, bool) {
	name := strings.ToLower(strings.TrimSpace(column))
	base, ok := strings.CutSuffix(name, "_id")
	if !ok || base == "" {
		return ai.TableSchema{}, false
	}
	candidates := []string{base, base + "s", base + "es"}
	if strings.HasSuffix(base, "y") {
		candidates = append(candidates, strings.TrimSuffix(base, "y")+"ies")
	}
	for _, cand := range candidates {
		if t, ok := byName[cand]; ok {
			return t, true
		}
	}
	for _, t := range e.schema.Tables { // 按表的原始顺序找，结果才是确定的
		short := trimTablePrefix(strings.ToLower(t.Name))
		if short == base || strings.TrimSuffix(short, "s") == base {
			return t, true
		}
	}
	return ai.TableSchema{}, false
}

// trimTablePrefix 去掉常见的表名前缀（tbl_、tb_、t_、sys_）。
func trimTablePrefix(name string) string {
	for _, p := range []string{"tbl_", "tb_", "t_", "sys_"} {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}

// keyMark 列的键标记：主键 PK，外键 FK。
func keyMark(c ai.ColumnSchema) string {
	switch {
	case c.IsPrimary:
		return "PK"
	case c.ForeignKey != "":
		return "FK"
	}
	return ""
}

func yesNo(b bool) string {
	if b {
		return "是"
	}
	return ""
}
