package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 表标签顶部的 34px 条栏（设计稿方案 B）：左边是 数据 / 结构 / DDL / 文档 的下划线标签，
// 右边是当前视图的操作；行数、分页和提交顺序提示不在这里，放在底部状态栏（statusbar.go 的 tabStatus）。

// topBar 画条栏。新建表只有结构页，没有视图切换，也就没有这一条。
func (t *tableTab) topBar(c *ui.Context, p tokens) {
	if t.isNew {
		return
	}
	ui.Toolbar(c, func() {
		ui.Row(c.Key("tt-bar")).Gap(spaceXS).Padding(0, spaceSM+2, 0, spaceSM).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Background(p.surface).Children(func() {
			labels := []string{locales.TableViewData(), locales.TableViewStructure(), locales.TableViewDDL(), locales.TableViewDoc()}
			if i := ulStrip(c, p, "tt-mode", labels, nil, t.mode, sizeBar, false); i >= 0 && i != t.mode {
				t.mode = i
				t.modeChanged()
			}
			switch t.mode {
			case 0:
				vline(c, p.border, 14)
				t.filterPill(c, p)
			case 1:
				t.structNameInline(c, p)
			}
			ui.Spacer(c)
			readOnly := t.w.cfg.ReadOnly || !t.w.rowEditable()
			switch t.mode {
			case 0:
				label := locales.TableColsBtn(itoa(len(t.g.vis)), itoa(len(t.g.cols)))
				if tipOn(c, ghostBtn(c, icSliders, label), locales.TableColumnsTip()+" (⌘⌥F)").Clicked() {
					t.colsOpen = true
				}
				if tipBtn(c, icPlus, locales.EditAddRow()).Size(sizeBtnSm, sizeBtnSm).Disabled(readOnly).Clicked() {
					t.addRow()
				}
				if tipBtn(c, icTrash, locales.EditDeleteRows()).Size(sizeBtnSm, sizeBtnSm).Disabled(readOnly).Clicked() {
					t.deleteSelected()
				}
				vline(c, p.border, 14)
				if tipOn(c, ghostBtn(c, icCode, "SQL"), locales.ToolbarSQL()).Clicked() {
					sql, _ := t.w.sh.app.Queries.DefaultSelectTableSQL(t.w.cfg.ID, t.table, 100)
					t.w.newQuery(sql)
				}
				if toggleBtn(c, icPanel, locales.PreviewToggle(), t.showPreview).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.togglePreview()
				}
				if tipBtn(c, icRefresh, locales.CommonRefresh()+" (⌘R)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.reload(nil)
				}
			case 1:
				if tipBtn(c, icRefresh, locales.CommonRefresh()+" (⌘R)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.st.load(t)
				}
			}
		})
	}).Padding(0)
	hairline(c, p.border)
}

// filterPill 是条栏中间的「添加过滤…」小条（设计稿里的过滤入口）：点一下打开筛选条；
// 筛选条打开或已有生效的条件时描边亮起强调色。
func (t *tableTab) filterPill(c *ui.Context, p tokens) {
	tip := locales.TableAddCondition() + " (⌘F)"
	on := t.showFilter || t.hasFilter()
	border := p.border
	if on {
		border = p.accent
	}
	b := tipOn(c, ui.Button(c.Key("tt-filter"), "").Label(tip), tip).Grow(1).MaxWidth(420).MinWidth(0).Height(sizeBtnSm).Padding(0, 7).Gap(5).
		Radius(radiusSm).Border(1, border).Background(p.surface).Justify(ui.Start).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, icFilter).FontSize(btnActIcon).TextColor(p.fgMuted)
		ui.Text(c, locales.TableFilterPill()).FontSize(fontDense).TextColor(p.fgMuted).SingleLine()
	})
	if b.Clicked() {
		t.toggleFilter()
	}
}

// tabStatus 是状态栏右边属于当前标签的部分：数据页是「第 1–200 行 · 共 N 行 · 耗时」+ 翻页 + 每页行数，
// 结构页是提交顺序的提示。其余标签没有。
func (w *Workspace) tabStatus(c *ui.Context, p tokens) {
	if o, ok := w.activeTab().(*overviewTab); ok { // 项目概览：左边是「7 张表 · 2 个视图 · 大小」
		ui.Text(c, overviewSummary(o.items)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
		return
	}
	if e, ok := w.activeTab().(*erTab); ok && e.full != nil { // ER 图：左边是「N 张表 · M 个外键」
		ui.Text(c, locales.ErStats(len(e.full.Nodes), len(e.full.Edges))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		return
	}
	if q, ok := w.activeTab().(*queryTab); ok {
		if q.page == pagePlan && q.plan != nil && q.plan.res != nil { // 执行计划页：节点数、耗时、是否 ANALYZE
			txt := locales.ExplainNodes(len(q.plan.res.Flat())) + " · " + fmtDur(q.plan.dur)
			if q.plan.res.Analyzed {
				txt += " · ANALYZE"
			}
			ui.Text(c, txt).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			return
		}
		q.queryStatus(c, p)
		return
	}
	t, ok := w.activeTab().(*tableTab)
	if !ok || t.isNew {
		return
	}
	switch t.mode {
	case 0:
		t.dataStatus(c, p)
	case 1:
		ui.Text(c, locales.StructCommitOrderHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
	}
}

// thousands 把整数写成带千分位的文字（12431 → 12,431）。
func thousands(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// dataStatus 画数据页的行范围、耗时和翻页。行范围的宽度随数字变，放在固定宽度的框里，总数晚到时状态栏不会跳。
func (t *tableTab) dataStatus(c *ui.Context, p tokens) {
	size := t.size()
	from := (t.page-1)*size + 1
	to := from + len(t.g.rows) - 1
	if len(t.g.rows) == 0 {
		from, to = 0, 0
	}
	txt := locales.TableRangeUnknown(thousands(int64(from)), thousands(int64(to)))
	if t.total >= 0 {
		txt = locales.TableRange(thousands(int64(from)), thousands(int64(to)), thousands(t.total))
	}
	if t.dur > 0 {
		txt += " · " + fmtDur(t.dur)
	}
	ui.Text(c, txt).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine().Shrink(1).MinWidth(0)
	t.pagination(c, p)
}
