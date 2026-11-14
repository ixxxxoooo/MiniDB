package model

import "testing"

func TestExplainFlatAndHottest(t *testing.T) {
	scanA := &PlanNode{Op: "Seq Scan", Object: "a", Cost: 10, ActualMs: 1}
	scanB := &PlanNode{Op: "Index Scan", Object: "b", Cost: 35, ActualMs: 8}
	root := &PlanNode{Op: "Nested Loop", Cost: 50, ActualMs: 9.5, Children: []*PlanNode{scanA, scanB}}
	r := &ExplainResult{Roots: []*PlanNode{root}}

	flat := r.Flat()
	if len(flat) != 3 || flat[0].Depth != 0 || flat[1].Node != scanA || flat[2].Depth != 1 {
		t.Fatalf("拍平结果不对: %+v", flat)
	}
	// 有实际耗时按耗时比：根自己只占 0.5ms，最费时的是 scanB
	if h := r.Hottest(); h != scanB {
		t.Fatalf("Hottest=%v，期望 scanB", h)
	}
	// 没有实际耗时时按代价比
	scanA.ActualMs, scanB.ActualMs, root.ActualMs = 0, 0, 0
	scanA.Cost = 40
	if h := r.Hottest(); h != scanA {
		t.Fatalf("按代价应是 scanA，得到 %v", h)
	}
	// 什么数据都没有：不标
	if h := (&ExplainResult{Roots: []*PlanNode{{Op: "x"}}}).Hottest(); h != nil {
		t.Fatalf("没有数据不该标热点: %v", h)
	}
	if got := (&ExplainResult{}).Flat(); len(got) != 0 {
		t.Fatal("空结果拍平应为空")
	}
}
