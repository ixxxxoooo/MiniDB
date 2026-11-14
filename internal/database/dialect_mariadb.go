package database

import (
	"context"
	"strings"

	"minidb/internal/model"
)

// mariadbDialect 与 MySQL 同一套协议和驱动，元数据与 SQL 写法也一致，只在这几处不同：
// 账号的资源限制子句写法不同，不开放；EXPLAIN 的 JSON 与 TREE 格式和 MySQL 不一样，统一用传统表格，也没有 EXPLAIN ANALYZE。
type mariadbDialect struct{ mysqlDialect }

func (mariadbDialect) Label() string { return "MariaDB" }

func (mariadbDialect) Caps() Caps {
	c := mysqlDialect{}.Caps()
	c.UserResourceLimits = false
	c.ExplainAnalyze = false
	return c
}

// Explain 只用传统的表格计划。
func (mariadbDialect) Explain(ctx context.Context, db Executor, sqlText string, analyze bool) (*model.ExplainResult, error) {
	q := "EXPLAIN " + sqlText
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		return nil, err
	}
	res := tableResult(q, cols, rows, "MariaDB 使用传统表格计划")
	res.Roots = traditionalPlanRoots(cols, rows)
	return res, nil
}

// traditionalPlanRoots 把传统 EXPLAIN 表格（id、select_type、table、type、key、rows、Extra）的每一行变成一个顶层节点。
// 这种计划没有层级，只能平铺；缺哪一列就留空。
func traditionalPlanRoots(cols []string, rows [][]string) []*model.PlanNode {
	idx := map[string]int{}
	for i, c := range cols {
		idx[strings.ToLower(c)] = i
	}
	get := func(r []string, name string) string {
		if i, ok := idx[name]; ok && i < len(r) {
			return r[i]
		}
		return ""
	}
	var out []*model.PlanNode
	for _, r := range rows {
		op := get(r, "type")
		if op == "" {
			op = get(r, "select_type")
		}
		extra := get(r, "extra")
		if k := get(r, "key"); k != "" {
			extra = strings.TrimSpace("key=" + k + " " + extra)
		}
		out = append(out, &model.PlanNode{Op: op, Object: get(r, "table"), Rows: parseNum(get(r, "rows")), Extra: extra})
	}
	return out
}
