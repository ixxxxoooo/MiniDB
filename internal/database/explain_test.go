package database

import (
	"context"
	"strings"
	"testing"
)

func TestStripExplainPrefix(t *testing.T) {
	cases := map[string]string{
		"select 1":                           "select 1",
		"  SELECT 1 ;; ":                     "SELECT 1",
		"EXPLAIN select * from t":            "select * from t",
		"explain  analyze\nselect 1":         "select 1",
		"EXPLAIN QUERY PLAN select 1":        "select 1",
		"EXPLAIN EXPLAIN select 1":           "select 1",
		"select 'explain' from t":            "select 'explain' from t",
		"EXPLAIN SELECT a  FROM  t WHERE 1;": "SELECT a  FROM  t WHERE 1",
	}
	for in, want := range cases {
		if got := StripExplainPrefix(in); got != want {
			t.Errorf("StripExplainPrefix(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestParsePGPlan(t *testing.T) {
	raw := `[{"Plan":{"Node Type":"Hash Join","Join Type":"Inner","Total Cost":25.5,"Plan Rows":100,"Hash Cond":"(a.id = b.id)","Plans":[
	  {"Node Type":"Seq Scan","Parent Relationship":"Outer","Relation Name":"a","Alias":"a","Total Cost":10.0,"Plan Rows":100,"Filter":"(x > 1)"},
	  {"Node Type":"Hash","Parent Relationship":"Inner","Total Cost":8.0,"Plan Rows":50,"Plans":[
	    {"Node Type":"Index Scan","Relation Name":"b","Alias":"bb","Index Name":"b_pkey","Total Cost":6,"Plan Rows":50,"Index Cond":"(id > 5)"}]}]}}]`
	roots, err := parsePGPlan(raw)
	if err != nil || len(roots) != 1 {
		t.Fatalf("roots=%v err=%v", roots, err)
	}
	j := roots[0]
	if j.Op != "Hash Join" || j.Cost != 25.5 || j.Rows != 100 || !strings.Contains(j.Extra, "Hash Cond: (a.id = b.id)") || !strings.Contains(j.Extra, "Join Type: Inner") {
		t.Fatalf("根节点不对: %+v", j)
	}
	if len(j.Children) != 2 || j.Children[0].Object != "a" || j.Children[0].Extra != "Filter: (x > 1)" {
		t.Fatalf("子节点不对: %+v", j.Children[0])
	}
	idx := j.Children[1].Children[0]
	if idx.Op != "Index Scan" || idx.Object != "b bb / b_pkey" {
		t.Fatalf("索引节点不对: %+v", idx)
	}
	if _, err := parsePGPlan(`not json`); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}

func TestParsePGPlanAnalyze(t *testing.T) {
	raw := `[{"Plan":{"Node Type":"Nested Loop","Total Cost":50,"Plan Rows":5,"Actual Total Time":9.0,"Actual Rows":4,"Actual Loops":1,"Plans":[
	  {"Node Type":"Seq Scan","Relation Name":"a","Total Cost":10,"Plan Rows":5,"Actual Total Time":1.0,"Actual Rows":4,"Actual Loops":1},
	  {"Node Type":"Index Scan","Relation Name":"b","Total Cost":35,"Plan Rows":1,"Actual Total Time":2.0,"Actual Rows":3,"Actual Loops":4}]}}]`
	roots, err := parsePGPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	inner := roots[0].Children[1]
	if inner.ActualMs != 8 || inner.Rows != 12 { // 每次循环的值要乘循环次数
		t.Fatalf("循环次数没有乘进去: %+v", inner)
	}
}

func TestParseMySQLJSONPlan(t *testing.T) {
	raw := `{"query_block":{"select_id":1,"cost_info":{"query_cost":"2.40"},"ordering_operation":{"using_filesort":true,"nested_loop":[
	  {"table":{"table_name":"a","access_type":"ALL","rows_examined_per_scan":10,"cost_info":{"prefix_cost":"1.0"},"attached_condition":"(a.x > 1)"}},
	  {"table":{"table_name":"b","access_type":"ref","key":"idx_a","rows_examined_per_scan":2,"cost_info":{"prefix_cost":"2.4"}}}]}}}`
	roots, err := parseMySQLJSONPlan(raw)
	if err != nil || len(roots) != 1 {
		t.Fatalf("roots=%v err=%v", roots, err)
	}
	q := roots[0]
	if q.Op != "query_block" || q.Cost != 2.4 || len(q.Children) != 1 {
		t.Fatalf("query_block 不对: %+v", q)
	}
	order := q.Children[0]
	if order.Op != "ordering_operation" || order.Extra != "using_filesort" || len(order.Children) != 1 {
		t.Fatalf("排序节点不对: %+v", order)
	}
	loop := order.Children[0]
	if loop.Op != "nested_loop" || len(loop.Children) != 2 {
		t.Fatalf("嵌套循环不对: %+v", loop)
	}
	a, b := loop.Children[0], loop.Children[1]
	if a.Op != "ALL" || a.Object != "a" || a.Rows != 10 || a.Cost != 1 || !strings.Contains(a.Extra, "attached_condition: (a.x > 1)") {
		t.Fatalf("表 a 不对: %+v", a)
	}
	if b.Op != "ref" || b.Object != "b (idx_a)" {
		t.Fatalf("表 b 不对: %+v", b)
	}
	if _, err := parseMySQLJSONPlan(`{"x":1}`); err == nil {
		t.Fatal("缺 query_block 应报错")
	}
}

func TestParseMySQLTreePlan(t *testing.T) {
	text := "-> Sort: a.x  (cost=2.5 rows=10) (actual time=0.1..0.5 rows=8 loops=1)\n" +
		"    -> Filter: (a.x > 1)  (cost=1.5 rows=10) (actual time=0.1..0.3 rows=8 loops=2)\n" +
		"        -> Table scan on a  (cost=1.5 rows=10) (actual time=0.05..0.2 rows=16 loops=2)\n" +
		"-> Rows fetched before execution  (cost=0.00..0.00 rows=1)"
	roots := parseMySQLTreePlan(text)
	if len(roots) != 2 {
		t.Fatalf("应有两个顶层: %d", len(roots))
	}
	sort := roots[0]
	if sort.Op != "Sort" || sort.Extra != "a.x" || sort.Cost != 2.5 || sort.ActualMs != 0.5 || sort.Rows != 8 {
		t.Fatalf("Sort 不对: %+v", sort)
	}
	scan := sort.Children[0].Children[0]
	if scan.Op != "Table scan" || scan.Object != "a" || scan.ActualMs != 0.4 || scan.Rows != 32 {
		t.Fatalf("Table scan 不对（耗时、行数要乘 loops）: %+v", scan)
	}
}

func TestParseTiDBPlan(t *testing.T) {
	cols := []string{"id", "estRows", "task", "access object", "operator info"}
	rows := [][]string{
		{"Projection_4", "10.00", "root", "", "test.t.a"},
		{"└─TableReader_7", "10.00", "root", "", "data:Selection_6"},
		{"  └─Selection_6", "10.00", "cop[tikv]", "", "gt(test.t.a, 1)"},
		{"    └─TableFullScan_5", "100.00", "cop[tikv]", "table:t", "keep order:false"},
	}
	roots := parseTiDBPlan(cols, rows)
	if len(roots) != 1 || roots[0].Op != "Projection" {
		t.Fatalf("根不对: %+v", roots)
	}
	n := roots[0]
	for _, want := range []string{"TableReader", "Selection", "TableFullScan"} {
		if len(n.Children) != 1 || n.Children[0].Op != want {
			t.Fatalf("期望下一层是 %s: %+v", want, n.Children)
		}
		n = n.Children[0]
	}
	if n.Object != "table:t" || n.Rows != 100 || n.Extra != "cop[tikv]; keep order:false" {
		t.Fatalf("叶子节点不对: %+v", n)
	}

	// ANALYZE：多出 actRows 与 execution info
	cols = []string{"id", "estRows", "actRows", "task", "access object", "execution info", "operator info"}
	rows = [][]string{{"TableReader_5", "10.00", "7", "root", "", "time:1.5ms, loops:2", "data:x"}}
	if r := parseTiDBPlan(cols, rows); len(r) != 1 || r[0].Rows != 7 || r[0].ActualMs != 1.5 {
		t.Fatalf("ANALYZE 列没读到: %+v", r)
	}
}

func TestParseSQLitePlan(t *testing.T) {
	rows := [][]string{
		{"3", "0", "0", "SEARCH t USING INDEX i (a=?)"},
		{"7", "0", "0", "SCAN TABLE u"},
		{"9", "7", "0", "USE TEMP B-TREE FOR ORDER BY"},
	}
	roots := parseSQLitePlan(rows)
	if len(roots) != 2 || roots[0].Op != "SEARCH" || roots[0].Object != "t" || roots[0].Extra != "USING INDEX i (a=?)" {
		t.Fatalf("顶层不对: %+v", roots)
	}
	if roots[1].Op != "SCAN" || roots[1].Object != "u" || len(roots[1].Children) != 1 || roots[1].Children[0].Op != "USE TEMP B-TREE FOR ORDER BY" {
		t.Fatalf("父子关系不对: %+v", roots[1])
	}
}

// 真实的 SQLite：EXPLAIN QUERY PLAN 能跑通并解析出树；ANALYZE 被拒绝；语句有错时报错。
func TestExplainQuerySQLiteReal(t *testing.T) {
	db := newMetaDB(t)
	ctx := context.Background()
	res, err := ExplainQuery(ctx, db, SQLite, "EXPLAIN SELECT * FROM users WHERE email = 'u1@x';", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Roots) == 0 || !strings.Contains(strings.ToLower(res.Roots[0].Extra+res.Roots[0].Op), "search") || res.Roots[0].Object != "users" {
		t.Fatalf("应解析出对 users 的 SEARCH: %+v", res.Roots)
	}
	if res.SQL != "EXPLAIN QUERY PLAN SELECT * FROM users WHERE email = 'u1@x'" || res.Analyzed {
		t.Fatalf("SQL=%q analyzed=%v", res.SQL, res.Analyzed)
	}
	if _, err := ExplainQuery(ctx, db, SQLite, "SELECT 1", true); err == nil {
		t.Fatal("SQLite 不支持 ANALYZE")
	}
	if _, err := ExplainQuery(ctx, db, SQLite, "SELECT * FROM nope", false); err == nil {
		t.Fatal("表不存在应报错")
	}
}
