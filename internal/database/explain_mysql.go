package database

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"minidb/internal/model"
)

// Explain MySQL：先试 FORMAT=JSON，不行（旧版本、MariaDB 等）降级 FORMAT=TREE（8.0.16+），
// 再不行用普通 EXPLAIN 表格。ANALYZE 是 8.0.18+ 的 EXPLAIN ANALYZE，输出是树形文本。
func (d mysqlDialect) Explain(ctx context.Context, db Executor, sqlText string, analyze bool) (*model.ExplainResult, error) {
	if analyze {
		q := "EXPLAIN ANALYZE " + sqlText
		cols, rows, err := fetchTable(ctx, db, q)
		if err != nil {
			return nil, err
		}
		return mysqlTreeResult(q, cols, rows), nil
	}
	q := "EXPLAIN FORMAT=JSON " + sqlText
	cols, rows, firstErr := fetchTable(ctx, db, q)
	if firstErr == nil && len(rows) > 0 && len(rows[0]) > 0 {
		res := &model.ExplainResult{SQL: q, Raw: rows[0][0], Columns: cols, Rows: rows}
		if roots, err := parseMySQLJSONPlan(rows[0][0]); err == nil {
			res.Roots = roots
			return res, nil
		}
		res.Note = "无法解析 JSON 计划，显示原文"
		return res, nil
	}
	q = "EXPLAIN FORMAT=TREE " + sqlText
	if cols, rows, err := fetchTable(ctx, db, q); err == nil {
		res := mysqlTreeResult(q, cols, rows)
		res.Note = "服务器不支持 JSON 计划，已改用树形文本"
		return res, nil
	}
	q = "EXPLAIN " + sqlText
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		if firstErr != nil {
			return nil, firstErr // 语句本身有错时，第一次的错误最能说明问题
		}
		return nil, err
	}
	return tableResult(q, cols, rows, "服务器只支持传统表格计划"), nil
}

// mysqlTreeResult 解析 FORMAT=TREE / ANALYZE 的单列文本；解析不出算子时只留原文。
func mysqlTreeResult(q string, cols []string, rows [][]string) *model.ExplainResult {
	text, res := textResult(q, cols, rows)
	if roots := parseMySQLTreePlan(text); len(roots) > 0 {
		res.Roots = roots
	} else {
		res.Note = "无法解析树形计划，显示原文"
	}
	return res
}

var (
	reTreeCost   = regexp.MustCompile(`\(cost=([\d.eE+-]+)(?: rows=([\d.eE+-]+))?\)`)
	reTreeActual = regexp.MustCompile(`\(actual time=([\d.]+)\.\.([\d.]+) rows=([\d.eE+-]+) loops=(\d+)\)`)
	reTreeObject = regexp.MustCompile(`^(.*?) on (\S+)(.*)$`)
)

// parseMySQLTreePlan 解析 "-> 算子  (cost=.. rows=..)" 的缩进文本（每层缩进 4 个空格）。
func parseMySQLTreePlan(text string) []*model.PlanNode {
	type frame struct {
		indent int
		node   *model.PlanNode
	}
	var roots []*model.PlanNode
	var stack []frame
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, "-> ") {
			continue
		}
		indent := len(line) - len(trimmed)
		n := mysqlTreeNode(strings.TrimPrefix(trimmed, "-> "))
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			p := stack[len(stack)-1].node
			p.Children = append(p.Children, n)
		}
		stack = append(stack, frame{indent, n})
	}
	return roots
}

func mysqlTreeNode(s string) *model.PlanNode {
	n := &model.PlanNode{}
	if m := reTreeActual.FindStringSubmatch(s); m != nil {
		loops := parseNum(m[4])
		n.ActualMs = parseNum(m[2]) * loops
		n.Rows = parseNum(m[3]) * loops
	}
	if m := reTreeCost.FindStringSubmatch(s); m != nil {
		n.Cost = parseNum(m[1])
		if n.Rows == 0 {
			n.Rows = parseNum(m[2])
		}
	}
	// 去掉括号里的统计，剩下 "算子[: 说明]" 或 "算子 on 表 [using 索引 ...]"
	desc := strings.TrimSpace(reTreeActual.ReplaceAllString(reTreeCost.ReplaceAllString(s, ""), ""))
	if i := strings.Index(desc, ": "); i > 0 {
		n.Op, n.Extra = desc[:i], strings.TrimSpace(desc[i+2:])
	} else {
		n.Op = desc
	}
	if m := reTreeObject.FindStringSubmatch(n.Op); m != nil {
		n.Op, n.Object = strings.TrimSpace(m[1]), m[2]
		if rest := strings.TrimSpace(m[3]); rest != "" {
			if n.Extra != "" {
				n.Extra = rest + "; " + n.Extra
			} else {
				n.Extra = rest
			}
		}
	}
	return n
}

// parseMySQLJSONPlan 解析 EXPLAIN FORMAT=JSON：{"query_block": {...}}。
func parseMySQLJSONPlan(raw string) ([]*model.PlanNode, error) {
	var top map[string]any
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return nil, err
	}
	qb, ok := top["query_block"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("计划里没有 query_block")
	}
	return []*model.PlanNode{mysqlBlock("query_block", qb)}, nil
}

// mysqlBlock 把 JSON 里的一个块转成节点：table 块是表访问，其余块是容器（嵌套循环、排序、分组、子查询…）。
func mysqlBlock(name string, m map[string]any) *model.PlanNode {
	n := &model.PlanNode{Op: name}
	var extra []string
	if name == "table" {
		n.Op, _ = m["access_type"].(string)
		n.Object, _ = m["table_name"].(string)
		if k, ok := m["key"].(string); ok && k != "" {
			n.Object += " (" + k + ")"
		}
		n.Rows = jsonNum(m["rows_examined_per_scan"])
		if ci, ok := m["cost_info"].(map[string]any); ok {
			n.Cost = jsonNum(ci["prefix_cost"])
		}
		for _, k := range []string{"attached_condition", "index_condition", "used_key_parts", "using_index", "filtered"} {
			if v, ok := m[k]; ok {
				extra = append(extra, fmt.Sprintf("%s: %v", k, v))
			}
		}
	} else if ci, ok := m["cost_info"].(map[string]any); ok {
		n.Cost = jsonNum(ci["query_cost"])
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := m[k].(type) {
		case map[string]any:
			if k == "cost_info" || name == "table" && k != "materialized_from_subquery" {
				continue
			}
			n.Children = append(n.Children, mysqlBlock(k, v))
		case []any:
			if name == "table" && k != "attached_subqueries" {
				continue
			}
			// 数组（nested_loop、attached_subqueries 等）自成一层，每一项是它的子节点
			group := &model.PlanNode{Op: k}
			for _, e := range v {
				if em, ok := e.(map[string]any); ok {
					group.Children = append(group.Children, mysqlListItem(em)...)
				}
			}
			n.Children = append(n.Children, group)
		case bool:
			if v && name != "table" {
				extra = append(extra, k)
			}
		case string:
			if name != "table" || k == "message" {
				extra = append(extra, k+": "+v)
			}
		}
	}
	n.Extra = strings.Join(extra, "; ")
	return n
}

// mysqlListItem 展开数组里的一项：{"table": {...}}、{"query_block": {...}} 等各自成为一个子节点。
func mysqlListItem(em map[string]any) []*model.PlanNode {
	keys := make([]string, 0, len(em))
	for k := range em {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []*model.PlanNode
	for _, k := range keys {
		if v, ok := em[k].(map[string]any); ok {
			out = append(out, mysqlBlock(k, v))
		}
	}
	return out
}

// jsonNum 读 JSON 里的数字；MySQL 常把数字写成字符串。
func jsonNum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		return parseNum(x)
	}
	return 0
}
