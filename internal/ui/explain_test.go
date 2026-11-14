package ui

import (
	"testing"
)

// @author ixxxxoooo

// TestExplainPlanPage 在查询标签里解释光标所在的语句：出现「执行计划」页，显示计划树；
// 关闭后回到数据页；语句不能解释时给出错误提示。
func TestExplainPlanPage(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM users WHERE id = 1")
	e.tt.Frame()
	q.explain()
	e.tt.Frame()
	e.tt.Frame()
	if q.plan == nil || q.plan.loading || q.plan.err != "" || q.page != pagePlan {
		t.Fatalf("应进入执行计划页: %+v page=%d", q.plan, q.page)
	}
	if q.plan.res == nil || len(q.plan.res.Roots) == 0 {
		t.Fatalf("SQLite 的计划应解析成树: %+v", q.plan.res)
	}
	e.has("操作")
	e.has("users")
	e.has("个节点")
	e.shot("25-explain")

	// 切到原文页签
	q.plan.tab = 1
	e.tt.Frame()
	e.has("SEARCH")

	// 关闭计划页：回到数据页，计划清空
	q.closePlan()
	if q.plan != nil || q.page != pageData {
		t.Fatalf("关闭后应回到数据页: plan=%v page=%d", q.plan, q.page)
	}
}

func TestExplainErrorAndSingleStatement(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM no_such_table")
	e.tt.Frame()
	q.explain()
	e.tt.Frame()
	e.tt.Frame()
	if q.plan == nil || q.plan.err == "" {
		t.Fatalf("表不存在应显示错误: %+v", q.plan)
	}
	e.has("no_such_table")

	// 选中两条语句：只提示，不发起解释
	q2 := w.newQuery("SELECT 1;\nSELECT 2")
	e.tt.Frame()
	q2.ed.Select(0, len("SELECT 1;\nSELECT 2"))
	q2.explain()
	if q2.plan != nil {
		t.Fatal("多条语句不应发起解释")
	}
}

func TestPlanNumFormat(t *testing.T) {
	cases := map[float64]string{0: "", -1: "", 7: "7", 2.5: "2.50", 12000: "12.0K", 3400000: "3.4M", 5e9: "5.0G"}
	for in, want := range cases {
		if got := planNum(in); got != want {
			t.Errorf("planNum(%v) = %q, 期望 %q", in, got, want)
		}
	}
	if planMs(0) != "" || planMs(1.234) != "1.23 ms" || planMs(2500) != "2.50 s" {
		t.Errorf("planMs 格式不对: %q %q %q", planMs(0), planMs(1.234), planMs(2500))
	}
}
