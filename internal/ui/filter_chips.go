package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 过滤条件芯片行（设计稿 A）：条件生效后，在数据网格上方显示「图标 + 条件芯片（× 可摘除）+ 添加条件 +
// 匹配行数 + 应用」。点「添加条件」打开完整的筛选条编辑；芯片的 × 摘掉对应条件并立刻重新查询。

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

// chipsVisible 为真时在网格上方画条件芯片行：筛选条没打开，而且确实有生效的条件。
func (t *tableTab) chipsVisible() bool {
	return !t.showFilter && (len(t.applied) > 0 || t.appliedRaw != "")
}

// filterChips 画条件芯片行。必须在 chipsVisible 为真时调用。
func (t *tableTab) filterChips(c *ui.Context, p tokens) {
	labels := t.appliedLabels()
	drop := -1
	ui.Row(c).Gap(spaceSM).Padding(spaceSM, spaceLG).AlignItems(ui.Center).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Icon(c, icFilter).FontSize(btnActIcon).TextColor(p.fgMuted)
		for i, l := range labels {
			ui.Row(c.Key("chip"+l)).Height(sizeChip+spaceXS).Padding(0, spaceXS, 0, spaceSM).Gap(spaceXS).AlignItems(ui.Center).Shrink(0).
				Radius((sizeChip + spaceXS) / 2).Background(p.accentSoft).Children(func() {
				ui.Text(c, l).FontSize(fontCaption).Font(mono).TextColor(p.accentText).SingleLine()
				if tipBtn(c.Key("chipx"+l), icX, locales.TableFilterRemove()).Size(sizeChip, sizeChip).Clicked() {
					drop = i
				}
			})
		}
		if b := actBtn(c.Key("chipadd"), icPlus, locales.TableAddCondition()); b.Clicked() {
			t.toggleFilter()
		}
		ui.Spacer(c)
		if t.total >= 0 {
			ui.Text(c, locales.TableMatchRows(thousands(t.total))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		}
		if actPrimary(c.Key("chiprun"), icCheck, locales.TableFilterRun()+" ↵").Clicked() {
			t.applyFilter()
		}
	})
	hairline(c, p.border)
	if drop >= 0 && t.dropApplied(drop) {
		t.applyFilter()
	}
}
