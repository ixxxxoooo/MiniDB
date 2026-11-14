package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

func TestFilterCompile(t *testing.T) {
	f := &filterState{filterOr: true, filters: []filterRow{
		{on: true, col: "name", op: "=", val: "a"},
		{on: false, col: "age", op: ">", val: "1"},                   // 没勾选：忽略
		{on: true, col: "bio", op: "=", val: "  "},                   // 空值：忽略
		{on: true, col: "email", op: "IS NULL"},                      // 不需要值
		{on: true, col: locales.TableFilterAny(), op: ">", val: "x"}, // 任意列只支持 = 和 LIKE，其余退成 LIKE
		{on: true, col: locales.DatagridRawSQL(), val: " id > 3 "},
		{on: true, col: locales.DatagridRawSQL(), val: "ignored"}, // 手写 SQL 只取第一条
	}}
	f.compile()
	if f.appliedRaw != "id > 3" {
		t.Fatalf("手写条件 = %q", f.appliedRaw)
	}
	if len(f.applied) != 3 {
		t.Fatalf("applied = %+v", f.applied)
	}
	if f.applied[0].Or || !f.applied[1].Or || !f.applied[2].Or {
		t.Fatalf("OR 只应加在第一个之后的条件上: %+v", f.applied)
	}
	if f.applied[2].Column != "__any" || f.applied[2].Operator != "LIKE" {
		t.Fatalf("任意列条件 = %+v", f.applied[2])
	}
}

func TestFilterHasAndReset(t *testing.T) {
	f := &filterState{filters: []filterRow{newFilterRow()}}
	if f.hasFilter() {
		t.Fatal("空过滤条不算有过滤")
	}
	f.filters[0].val = "x"
	if !f.hasFilter() {
		t.Fatal("输入了值就算有过滤")
	}
	f.compile()
	f.reset()
	if f.hasFilter() || len(f.filters) != 1 || len(f.applied) != 0 {
		t.Fatalf("reset 之后应回到初始: %+v", f)
	}
}

func TestSortStateAddAndMarks(t *testing.T) {
	var s sortState
	if s.sortList() != nil || s.sortMarks() != nil {
		t.Fatal("没排序时应为空")
	}
	s.sorted, s.sortOrder = true, ui.SortOrder{Column: "a"}
	if l := s.sortList(); len(l) != 1 || l[0].Column != "a" || l[0].Direction != "ASC" {
		t.Fatalf("单列排序 = %+v", l)
	}
	// ⇧ 点击另一列：表头原来显示的排在前面
	s.addSort(ui.SortOrder{Column: "b", Descending: true}, true)
	if len(s.sorts) != 2 || s.sorts[0].Column != "a" || s.sorts[1].Direction != "DESC" {
		t.Fatalf("多列排序 = %+v", s.sorts)
	}
	marks := s.sortMarks()
	if marks["a"].n != 1 || marks["b"].n != 2 || !marks["b"].desc {
		t.Fatalf("序号标记 = %+v", marks)
	}
	// 再点已有的列：改方向，不重复添加
	s.addSort(ui.SortOrder{Column: "a", Descending: true}, true)
	if len(s.sorts) != 2 || s.sorts[0].Direction != "DESC" {
		t.Fatalf("同列应改方向: %+v", s.sorts)
	}
	// 不按 ⇧：只留这一列
	s.addSort(ui.SortOrder{Column: "c"}, false)
	if len(s.sorts) != 1 || s.sorts[0].Column != "c" {
		t.Fatalf("单击应只留一列: %+v", s.sorts)
	}
}
