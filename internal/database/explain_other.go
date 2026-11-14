package database

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"minidb/internal/model"
)

// ---------- TiDB ----------

// Explain TiDB：普通 EXPLAIN / EXPLAIN ANALYZE 返回表格，算子层级藏在 id 列的缩进字符里。
func (tidbDialect) Explain(ctx context.Context, db Executor, sqlText string, analyze bool) (*model.ExplainResult, error) {
	q := "EXPLAIN " + sqlText
	if analyze {
		q = "EXPLAIN ANALYZE " + sqlText
	}
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		return nil, err
	}
	res := tableResult(q, cols, rows, "")
	if roots := parseTiDBPlan(cols, rows); len(roots) > 0 {
		res.Roots = roots
	} else {
		res.Note = "无法解析计划树，显示原表"
	}
	return res, nil
}

var (
	reTiDBSuffix = regexp.MustCompile(`_\d+$`)
	reTiDBTime   = regexp.MustCompile(`time:\s*([\d.]+)\s*(ns|µs|us|ms|s)`)
)

// parseTiDBPlan 按 id 列前面的树形字符（└─ ├─ │）还原层级：每层缩进 2 个字符。
func parseTiDBPlan(cols []string, rows [][]string) []*model.PlanNode {
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
	type frame struct {
		depth int
		node  *model.PlanNode
	}
	var roots []*model.PlanNode
	var stack []frame
	for _, r := range rows {
		id := get(r, "id")
		if id == "" {
			continue
		}
		pos := 0
		for i, ch := range []rune(id) {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				pos = i
				break
			}
		}
		depth := pos / 2
		name := strings.TrimSpace(strings.TrimLeftFunc(id, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
		n := &model.PlanNode{
			Op:     reTiDBSuffix.ReplaceAllString(name, ""),
			Object: get(r, "access object"),
			Rows:   parseNum(get(r, "estrows")),
		}
		if act := get(r, "actrows"); act != "" {
			n.Rows = parseNum(act)
		}
		if m := reTiDBTime.FindStringSubmatch(get(r, "execution info")); m != nil {
			n.ActualMs = parseNum(m[1]) * map[string]float64{"ns": 1e-6, "µs": 1e-3, "us": 1e-3, "ms": 1, "s": 1000}[m[2]]
		}
		var extra []string
		if t := get(r, "task"); t != "" {
			extra = append(extra, t)
		}
		if info := get(r, "operator info"); info != "" && info != "N/A" {
			extra = append(extra, info)
		}
		n.Extra = strings.Join(extra, "; ")
		for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			p := stack[len(stack)-1].node
			p.Children = append(p.Children, n)
		}
		stack = append(stack, frame{depth, n})
	}
	return roots
}

// ---------- StarRocks ----------

// Explain StarRocks：文本计划（每行一条），结构不稳定，只给原文。ANALYZE 不支持。
func (starrocksDialect) Explain(ctx context.Context, db Executor, sqlText string, _ bool) (*model.ExplainResult, error) {
	q := "EXPLAIN " + sqlText
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		return nil, err
	}
	_, res := textResult(q, cols, rows)
	res.Note = "StarRocks 的计划只显示原文"
	return res, nil
}

// ---------- SQLite ----------

// Explain SQLite：EXPLAIN QUERY PLAN，返回 id、parent、notused、detail 四列。不支持 ANALYZE。
func (sqliteDialect) Explain(ctx context.Context, db Executor, sqlText string, _ bool) (*model.ExplainResult, error) {
	q := "EXPLAIN QUERY PLAN " + sqlText
	cols, rows, err := fetchTable(ctx, db, q)
	if err != nil {
		return nil, err
	}
	res := tableResult(q, cols, rows, "")
	if roots := parseSQLitePlan(rows); len(roots) > 0 {
		res.Roots = roots
	}
	return res, nil
}

var reSQLiteObject = regexp.MustCompile(`^(SCAN|SEARCH)\s+(?:TABLE\s+)?(\S+)(?:\s+AS\s+\S+)?\s*(.*)$`)

// parseSQLitePlan 用 id / parent 还原树：parent 为 0 的是顶层。
func parseSQLitePlan(rows [][]string) []*model.PlanNode {
	byID := map[string]*model.PlanNode{}
	var roots []*model.PlanNode
	for _, r := range rows {
		if len(r) < 4 {
			return nil
		}
		n := &model.PlanNode{Op: r[3]}
		if m := reSQLiteObject.FindStringSubmatch(r[3]); m != nil {
			n.Op, n.Object, n.Extra = m[1], m[2], strings.TrimSpace(m[3])
		}
		byID[r[0]] = n
		if p, ok := byID[r[1]]; ok && r[1] != "0" {
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots
}
