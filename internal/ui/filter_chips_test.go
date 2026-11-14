package ui

import (
	"testing"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 条件芯片的文字与摘除逻辑。

func TestFilterChipsLabelsAndDrop(t *testing.T) {
	f := &filterState{filters: []filterRow{
		{on: true, col: "status", op: "=", val: "paid"},
		{on: true, col: "total", op: ">", val: "100"},
		{on: true, col: locales.DatagridRawSQL(), val: "id < 9"},
	}}
	f.compile()
	got := f.appliedLabels()
	want := []string{"status = paid", "total > 100", "id < 9"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("芯片文字 = %v，想要 %v", got, want)
	}
	if f.filterSummary() != "status = paid total > 100 id < 9" {
		t.Fatalf("摘要 = %q", f.filterSummary())
	}
	if !f.dropApplied(1) {
		t.Fatal("摘除第 2 个条件应成功")
	}
	f.compile()
	if got := f.appliedLabels(); len(got) != 2 || got[0] != "status = paid" || got[1] != "id < 9" {
		t.Fatalf("摘除后 = %v", got)
	}
	if !f.dropApplied(1) { // 现在下标 1 是手写 SQL
		t.Fatal("摘除手写 SQL 应成功")
	}
	f.compile()
	if got := f.appliedLabels(); len(got) != 1 || got[0] != "status = paid" {
		t.Fatalf("再次摘除后 = %v", got)
	}
}
