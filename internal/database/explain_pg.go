package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"minidb/internal/model"
)

// Explain PostgreSQL：EXPLAIN (FORMAT JSON)，ANALYZE 时加 ANALYZE 与 BUFFERS。
func (postgresDialect) Explain(ctx context.Context, db Executor, sqlText string, analyze bool) (*model.ExplainResult, error) {
	q := "EXPLAIN (FORMAT JSON) " + sqlText
	if analyze {
		q = "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + sqlText
	}
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		return nil, err
	}
	raw := ""
	if len(rows) > 0 && len(rows[0]) > 0 {
		raw = rows[0][0]
	}
	res := &model.ExplainResult{SQL: q, Raw: raw, Columns: cols, Rows: rows}
	roots, err := parsePGPlan(raw)
	if err != nil {
		res.Note = "无法解析 JSON 计划，显示原文"
		return res, nil
	}
	res.Roots = roots
	return res, nil
}

// pgExtraKeys 是会附在节点说明里的字段，顺序即显示顺序。
var pgExtraKeys = []string{
	"Join Type", "Strategy", "Index Cond", "Recheck Cond", "Hash Cond", "Merge Cond", "Join Filter",
	"Filter", "Sort Key", "Group Key", "Scan Direction", "Rows Removed by Filter", "Parent Relationship",
}

// parsePGPlan 解析 EXPLAIN (FORMAT JSON) 的输出：[{"Plan": {...}}]。
func parsePGPlan(raw string) ([]*model.PlanNode, error) {
	var top []map[string]any
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return nil, err
	}
	var roots []*model.PlanNode
	for _, t := range top {
		if p, ok := t["Plan"].(map[string]any); ok {
			roots = append(roots, pgNode(p))
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("计划里没有 Plan 节点")
	}
	return roots, nil
}

func pgNode(m map[string]any) *model.PlanNode {
	n := &model.PlanNode{}
	n.Op, _ = m["Node Type"].(string)
	// 访问对象：表（带别名）或索引
	var obj []string
	if r, ok := m["Relation Name"].(string); ok {
		name := r
		if a, ok := m["Alias"].(string); ok && a != r {
			name += " " + a
		}
		obj = append(obj, name)
	}
	if ix, ok := m["Index Name"].(string); ok {
		obj = append(obj, ix)
	}
	if len(obj) == 0 {
		if c, ok := m["CTE Name"].(string); ok {
			obj = append(obj, c)
		}
	}
	n.Object = strings.Join(obj, " / ")
	n.Rows, _ = m["Plan Rows"].(float64)
	if ar, ok := m["Actual Rows"].(float64); ok {
		loops, _ := m["Actual Loops"].(float64)
		if loops < 1 {
			loops = 1
		}
		n.Rows = ar * loops
	}
	n.Cost, _ = m["Total Cost"].(float64)
	if t, ok := m["Actual Total Time"].(float64); ok {
		loops, _ := m["Actual Loops"].(float64)
		if loops < 1 {
			loops = 1
		}
		n.ActualMs = t * loops
	}
	var extra []string
	for _, k := range pgExtraKeys {
		switch v := m[k].(type) {
		case string:
			if v != "" && !(k == "Parent Relationship" && v == "Outer") {
				extra = append(extra, k+": "+v)
			}
		case float64:
			extra = append(extra, fmt.Sprintf("%s: %g", k, v))
		case []any:
			var parts []string
			for _, x := range v {
				parts = append(parts, fmt.Sprint(x))
			}
			extra = append(extra, k+": "+strings.Join(parts, ", "))
		}
	}
	n.Extra = strings.Join(extra, "; ")
	if subs, ok := m["Plans"].([]any); ok {
		for _, s := range subs {
			if sm, ok := s.(map[string]any); ok {
				n.Children = append(n.Children, pgNode(sm))
			}
		}
	}
	return n
}
