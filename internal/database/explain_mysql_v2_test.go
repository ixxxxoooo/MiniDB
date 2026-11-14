package database

import (
	"os"
	"strings"
	"testing"

	"minidb/internal/model"
)

// loadPlanFixture 读 testdata 里的真实 EXPLAIN 输出（取自 MySQL 26.10.0 的 FORMAT=JSON）。
func loadPlanFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// findNode 在计划树里按对象名找第一个节点。
func findNode(roots []*model.PlanNode, object string) *model.PlanNode {
	res := &model.ExplainResult{Roots: roots}
	for _, r := range res.Flat() {
		if strings.HasPrefix(r.Node.Object, object) {
			return r.Node
		}
	}
	return nil
}

// 新版 MySQL（explain_json_format_version=2）的计划以前会掉到「显示原文」，现在要解析成树。
func TestParseMySQLJSONPlanV2IndexLookup(t *testing.T) {
	roots, err := parseMySQLJSONPlan(loadPlanFixture(t, "mysql_explain_v2_index_lookup.json"))
	if err != nil || len(roots) != 1 {
		t.Fatalf("roots=%v err=%v", roots, err)
	}
	n := roots[0]
	if n.Op != "Index lookup" || n.Object != "users (idx_age)" || n.Rows != 17 || n.Cost != 4.7 {
		t.Fatalf("索引查找节点不对: %+v", n)
	}
	if !strings.Contains(n.Extra, "age = 30") {
		t.Fatalf("条件应留在说明里: %q", n.Extra)
	}
}

func TestParseMySQLJSONPlanV2JoinGroupSort(t *testing.T) {
	roots, err := parseMySQLJSONPlan(loadPlanFixture(t, "mysql_explain_v2_join_group_sort.json"))
	if err != nil || len(roots) != 1 {
		t.Fatalf("roots=%v err=%v", roots, err)
	}
	if root := roots[0]; root.Op != "Sort" || root.Extra != "sum(o.amount) DESC" {
		t.Fatalf("根应是排序: %+v", root)
	}
	u, o := findNode(roots, "users"), findNode(roots, "orders")
	if u == nil || o == nil {
		t.Fatalf("两张表都应出现在树里: users=%v orders=%v", u, o)
	}
	if u.Op != "Index range scan" || u.Object != "users (idx_age)" || u.Rows != 144 || u.Cost != 65.06 {
		t.Fatalf("users 节点不对: %+v", u)
	}
	if o.Op != "Index lookup" || o.Object != "orders (idx_user)" {
		t.Fatalf("orders 节点不对: %+v", o)
	}
	if got := len((&model.ExplainResult{Roots: roots}).Flat()); got < 5 {
		t.Fatalf("排序/临时表/聚合/循环/两个表至少 5 个节点，实际 %d", got)
	}
}

func TestParseMySQLJSONPlanV2Analyze(t *testing.T) {
	raw := `{"query_plan":{"operation":"Index lookup on users using idx_age (age = 30)","table_name":"users","index_name":"idx_age",
	  "estimated_rows":17.0,"estimated_total_cost":4.7,"actual_rows":17.0,"actual_loops":3,"actual_last_row_ms":0.5},"json_schema_version":"2.1"}`
	roots, err := parseMySQLJSONPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n := roots[0]; n.Rows != 51 || n.ActualMs != 1.5 { // 实际值是单次循环的，要乘循环次数
		t.Fatalf("循环次数没有乘进去: %+v", n)
	}
	if _, err := parseMySQLJSONPlan(`{"x":1}`); err == nil {
		t.Fatal("两种格式都不是应报错，好让上层降级")
	}
}
