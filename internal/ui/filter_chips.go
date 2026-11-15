package ui

import (
	"strings"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 过滤条件芯片的文字与摘除逻辑（芯片本身画在工具条的筛选组里，见 filter_pop.go）：
// 芯片的 × 摘掉对应条件并立刻重新查询。

// appliedLabels 返回已生效的条件文字，顺序与 compile 生成的一致；手写的 SQL 片段排在最后。
func (f *filterState) appliedLabels() []string {
	var out []string
	for _, a := range f.applied {
		col := a.Column
		if col == "__any" {
			col = locales.TableFilterAny()
		}
		txt := col + " " + a.Operator
		if a.Operator != "IS NULL" && a.Operator != "IS NOT NULL" {
			txt += " " + a.Value
		}
		out = append(out, txt)
	}
	if f.appliedRaw != "" {
		out = append(out, f.appliedRaw)
	}
	return out
}

// filterSummary 是条栏过滤框里显示的摘要（各条件用空格连接）；没有生效条件时返回空串。
func (f *filterState) filterSummary() string {
	return strings.Join(f.appliedLabels(), " ")
}

// hasApplied 报告有没有已经生效的条件（不含还在浮层里编辑、没点「应用」的草稿）。
func (f *filterState) hasApplied() bool { return len(f.applied) > 0 || f.appliedRaw != "" }

// dropApplied 摘掉第 i 个已生效的条件：把对应的筛选行取消勾选并重新编译。
// 返回是否真的摘掉了东西。
func (f *filterState) dropApplied(i int) bool {
	if i >= len(f.applied) { // 手写 SQL 片段
		for j := range f.filters {
			if f.filters[j].col == locales.DatagridRawSQL() && f.filters[j].on {
				f.filters[j].on = false
				return true
			}
		}
		return false
	}
	n := 0
	for j := range f.filters {
		r := &f.filters[j]
		if !r.on || r.col == locales.DatagridRawSQL() {
			continue
		}
		if strings.TrimSpace(r.val) == "" && r.op != "IS NULL" && r.op != "IS NOT NULL" {
			continue
		}
		if n == i {
			r.on = false
			return true
		}
		n++
	}
	return false
}
