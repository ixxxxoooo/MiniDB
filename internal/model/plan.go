package model

// PlanNode 是执行计划里的一个算子，各数据库的计划都先翻译成这棵统一的树再展示。
// Rows、Cost、ActualMs 为 0 表示数据库没给出（或没有 ANALYZE）。
type PlanNode struct {
	Op       string  // 算子，如 Seq Scan、Index Lookup、Hash Join
	Object   string  // 访问的表或索引
	Rows     float64 // 估算行数（ANALYZE 时为实际行数）
	Cost     float64 // 估算代价（累计值，含子节点）
	ActualMs float64 // ANALYZE 的实际耗时，毫秒（累计值，含子节点）
	Extra    string  // 条件、排序键等补充说明
	Children []*PlanNode
}

// ExplainResult 是一次 EXPLAIN 的结果。Roots 为空表示没能解析成树，此时看 Raw 和 Columns/Rows。
type ExplainResult struct {
	SQL      string      // 实际发给数据库的解释语句
	Roots    []*PlanNode // 计划树（SQLite 等可能有多个顶层）
	Raw      string      // 数据库返回的原文
	Columns  []string    // 原始表格的列（数据库返回表格时）
	Rows     [][]string  // 原始表格的行
	Analyzed bool        // 语句被真实执行过（带实际耗时）
	Note     string      // 降级或版本不支持等说明，空表示一切正常
}

// PlanRow 是把树拍平后的一行：节点和它的缩进层级。
type PlanRow struct {
	Node  *PlanNode
	Depth int
}

// Flat 按先序把整棵树拍平成带层级的行，供表格展示。
func (r *ExplainResult) Flat() []PlanRow {
	var out []PlanRow
	var walk func(n *PlanNode, depth int)
	walk = func(n *PlanNode, depth int) {
		out = append(out, PlanRow{Node: n, Depth: depth})
		for _, c := range n.Children {
			walk(c, depth+1)
		}
	}
	for _, n := range r.Roots {
		walk(n, 0)
	}
	return out
}

// selfValue 是节点自己的那一份：累计值减去子节点的累计值，不会小于 0。
func selfValue(n *PlanNode, get func(*PlanNode) float64) float64 {
	v := get(n)
	for _, c := range n.Children {
		v -= get(c)
	}
	if v < 0 {
		return 0
	}
	return v
}

// Hottest 返回最费时的节点：有实际耗时按耗时，否则按代价；都没有或全为 0 返回 nil。
// 比较的是节点自己的那一份（扣掉子节点），这样根节点不会因为累计值最大而总是被选中。
func (r *ExplainResult) Hottest() *PlanNode {
	rows := r.Flat()
	for _, get := range []func(*PlanNode) float64{
		func(n *PlanNode) float64 { return n.ActualMs },
		func(n *PlanNode) float64 { return n.Cost },
	} {
		var best *PlanNode
		var bestV float64
		for _, pr := range rows {
			if v := selfValue(pr.Node, get); v > bestV {
				best, bestV = pr.Node, v
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}
