package database

import (
	"fmt"

	"minidb/internal/model"
)

// 本文件解析 MySQL 新版 EXPLAIN FORMAT=JSON（explain_json_format_version=2，新版服务器默认）：
//
//	{"query": "...", "query_plan": {"operation": "Sort: x", "inputs": [ {...子节点...} ], ...}, "json_schema_version": "2.1"}
//
// 老格式（query_block 嵌套）在 explain_mysql.go。

// parseMySQLJSONPlanV2 解析 v2 计划；top 是整个 JSON 对象，没有 query_plan 时报错。
func parseMySQLJSONPlanV2(top map[string]any) ([]*model.PlanNode, error) {
	qp, ok := top["query_plan"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("计划里没有 query_plan")
	}
	return []*model.PlanNode{mysqlPlanNode(qp)}, nil
}

// mysqlPlanNode 把 v2 的一个节点转成 PlanNode。
// operation 是服务器写好的算子描述（与 FORMAT=TREE 的文字一致），交给 TREE 的文本解析拆出
// 算子、对象和说明；行数、代价、耗时取结构化字段，比从文字里抠更可靠。
func mysqlPlanNode(m map[string]any) *model.PlanNode {
	op, _ := m["operation"].(string)
	n := mysqlTreeNode(op)
	if t, ok := m["table_name"].(string); ok && t != "" {
		n.Object = t
		if idx, ok := m["index_name"].(string); ok && idx != "" {
			n.Object += " (" + idx + ")"
		}
	}
	n.Cost = jsonNum(m["estimated_total_cost"])
	n.Rows = jsonNum(m["estimated_rows"])
	if _, analyzed := m["actual_rows"]; analyzed { // EXPLAIN ANALYZE：数值都是单次循环的，要乘循环次数
		loops := max(jsonNum(m["actual_loops"]), 1)
		n.Rows = jsonNum(m["actual_rows"]) * loops
		n.ActualMs = jsonNum(m["actual_last_row_ms"]) * loops
	}
	if h, ok := m["heading"].(string); ok && h != "" { // 子查询等块的标题
		n.Extra = joinExtra(h, n.Extra)
	}
	inputs, _ := m["inputs"].([]any)
	for _, in := range inputs {
		if child, ok := in.(map[string]any); ok {
			n.Children = append(n.Children, mysqlPlanNode(child))
		}
	}
	return n
}

// joinExtra 把两段说明用 "; " 连起来，忽略空的。
func joinExtra(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}
