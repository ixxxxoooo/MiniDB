package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 数据页的筛选（设计稿「筛选」）：工具条里一个灰色圆角的「筛选组」——没有条件时是「添加过滤… ⌘F」，
// 有条件时是一排条件芯片（× 摘除）+「再加一条」；点它（或 ⌘F）从组的下面弹出筛选浮层：
// 标题与 且/或、每行 [列 | 运算符 | 值 | ×]、「再加一条」、带高亮的 WHERE 预览、底部「把筛选放进新查询 / 取消筛选 / 应用 ↵」。

// filterGroup 画工具条里的筛选组并在它下面挂出浮层。
func (t *tableTab) filterGroup(c *ui.Context, p tokens) {
	labels := t.appliedLabels()
	drop := -1
	g := ui.Row(c.Key("tt-filter")).Grow(4).MaxWidth(sizeFilterMax).MinWidth(0).Height(sizeBtn).Padding(0, spaceSM).Gap(spaceSM - 2).AlignItems(ui.Center).
		Radius(radiusView).Background(p.segTrack).Clip()
	if g.Hovered() || t.showFilter {
		g.Border(1, p.border)
	} else {
		g.Border(1, ui.Transparent)
	}
	g.Children(func() {
		ui.Icon(c, icFilter).FontSize(btnActIcon).TextColor(p.fgMuted).Shrink(0)
		for i, l := range labels {
			i, l := i, l
			// 设计稿 fchip：淡强调色底 + 强调色等宽文字的药丸，没有描边；增删时淡入淡出并轻微缩放（chipMotion）
			ui.Row(c.Key("chip"+l)).Height(sizeChip+spaceXS).Padding(0, spaceXS, 0, spaceSM).Gap(spaceXS).AlignItems(ui.Center).Shrink(0).
				Radius((sizeChip + spaceXS) / 2).Background(p.accentSoft).Transition(chipMotion).Children(func() {
				ui.Text(c, l).FontSize(fontCaption).Font(mono).FontWeight(500).TextColor(p.accentText).SingleLine()
				if tipBtn(c.Key("chipx"+l), icX, locales.TableFilterRemove()).Size(sizeChip, sizeChip).Clicked() {
					drop = i
				}
			})
		}
		if len(labels) > 0 {
			ui.Text(c, locales.TableFilterAddRow()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1)
		} else {
			ui.Text(c, locales.TableFilterPill()).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().Shrink(1)
		}
		ui.Spacer(c)
		kbd(c, "⌘F")
	})
	if g.Clicked() {
		t.toggleFilter()
	}
	t.filterPop(c, p, g)
	if drop >= 0 && t.dropApplied(drop) {
		t.applyFilter()
	}
}

// filterCols 是「列」下拉里的选项：手写 SQL、任意列，然后是表的各列。
func (t *tableTab) filterCols() []string {
	cols := []string{locales.DatagridRawSQL(), locales.TableFilterAny()}
	for _, col := range t.g.cols {
		cols = append(cols, col.Name)
	}
	return cols
}

// filterPop 画筛选浮层。草稿就是 t.filters；点外面或 Esc 只是收起，草稿留着，下次打开还在。
func (t *tableTab) filterPop(c *ui.Context, p tokens, anchor ui.Element) {
	if !t.showFilter {
		t.filterFocused = false
		return
	}
	apply, toQuery, unset := false, false, false
	focused, remove, add := -1, -1, false
	ui.PopoverBase(c, anchor, &t.showFilter, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft).Margin(spaceXS, 0, 0, 0).Width(sizeFilterPop).
			Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		enterPose(c, panel, posePop, ui.Snappy).Transition(popExit)
		ui.Column(c).FillWidth().Children(func() {
			t.filterHead(c, p)
			cols := t.filterCols()
			ui.Column(c).FillWidth().Gap(spaceSM).Padding(0, spaceLG, spaceSM, spaceLG).Children(func() {
				for i := range t.filters {
					i := i
					f := &t.filters[i]
					raw := f.col == locales.DatagridRawSQL()
					k := func(s string) string { return fmt.Sprint("f", i, s) }
					ui.Row(c.Key(k("row"))).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
						ui.Select(c.Key(k("col")), &f.col, cols).MinWidth(0).Width(sizeFilterCol).Height(sizeBtn)
						if !raw {
							ui.Select(c.Key(k("op")), &f.op, filterOps).MinWidth(0).Width(sizeFilterOp).Height(sizeBtn)
						}
						ph := locales.TableFilterValue()
						if raw {
							ph = locales.TableRawPlaceholder()
						}
						in := textField(c.Key(k("val")), &f.val).Placeholder(ph).Grow(1).MinWidth(0).Height(sizeBtn).Font(mono)
						if t.focusFilter && i == 0 {
							t.focusFilter = false
							in.Focus()
						}
						if in.Focused() {
							focused = i
						}
						if in.Submitted() {
							apply = true
						}
						if in.Changed() {
							f.on = true // 填了值就算数
						}
						if tipBtn(c.Key(k("rm")), icX, locales.TableFilterRemove()).Size(sizeBtn, sizeBtn).Clicked() {
							remove = i
						}
					})
				}
				if ghostBtn(c.Key("f-add"), icPlus, locales.TableFilterAddRow()).Clicked() {
					add = true
				}
			})
			t.filterWhere(c, p)
			hairline(c, p.borderSubtle)
			ui.Row(c).Gap(spaceSM).Padding(spaceSM+2, spaceLG).AlignItems(ui.Center).FillWidth().Children(func() {
				if footBtn(c.Key("f-toq"), locales.TableFilterToQuery()).Clicked() {
					toQuery = true
				}
				ui.Spacer(c)
				if footBtn(c.Key("f-unset"), locales.TableFilterUnset()).Disabled(!t.hasFilter()).Clicked() {
					unset = true
				}
				if footPrimary(c.Key("f-apply"), locales.TableFilterRun()+" ↵").Clicked() {
					apply = true
				}
			})
		})
	})
	t.filterFocused = focused >= 0
	t.filterEdits(c, focused, remove, add)
	switch {
	case unset:
		t.showFilter = false
		t.clearFilter()
	case toQuery:
		a, r := t.applied, t.appliedRaw
		t.compile() // 只为生成 SQL：不改变网格当前的筛选
		sql := t.tableSelectSQL()
		t.applied, t.appliedRaw = a, r
		t.showFilter = false
		t.w.newQuery(sql)
	case apply || c.Shortcut(ui.Cmd, ui.KeyEnter):
		t.showFilter = false
		t.applyFilter()
	}
}

// filterHead 是浮层的标题行：「筛选」+ 当前连接方式的说明，右边是 且 / 或 分段。
func (t *tableTab) filterHead(c *ui.Context, p tokens) {
	ui.Row(c).Gap(spaceSM).Padding(spaceLG-2, spaceLG, spaceSM+2, spaceLG).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.TableFilterTitle()).FontSize(fontBody).FontWeight(600)
		hint := locales.TableFilterMatchAll()
		if t.filterOr {
			hint = locales.TableFilterMatchAny()
		}
		ui.Text(c, hint).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		ui.Spacer(c)
		or := 0
		if t.filterOr {
			or = 1
		}
		if segmented(c, &or, locales.TableFilterAnd(), locales.TableFilterOr()) {
			t.filterOr = or == 1
		}
	})
}

// filterWhere 是带语法高亮的 WHERE 预览框；还没有任何条件时不画。
func (t *tableTab) filterWhere(c *ui.Context, p tokens) {
	typ := t.w.cfg.Type
	txt := t.whereText(func(s string) string { return app.QuoteIdent(typ, s) }, func(s string) string { return app.QuoteString(typ, s) })
	if txt == "" {
		return
	}
	txt = strings.ReplaceAll(txt, " AND ", "\nAND ")
	txt = strings.ReplaceAll(txt, " OR ", "\nOR ")
	ui.RichText(c.Key("f-where"), sqlSpans(p, txt, p.fg)...).Font(mono).FontSize(fontDense).Selectable().Padding(spaceSM+2, spaceLG-2).Radius(radiusBtn).
		Background(p.surfaceSecondary).Margin(0, spaceLG, spaceLG-4, spaceLG).Width(sizeFilterPop - 2*spaceLG)
}

// filterEdits 落实这一帧里对条件行的增删（等界面树画完再改，免得遍历到一半切片变短）和浮层里的键盘：
// ⌘I 加一条（输入框有焦点时不是打开 AI 面板），⌘⇧I 删掉当前这一条。
func (t *tableTab) filterEdits(c *ui.Context, focused, remove int, add bool) {
	if focused >= 0 {
		switch {
		case c.Shortcut(ui.Cmd, ui.KeyI):
			add = true
		case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyI):
			remove = focused
		}
	}
	if remove >= 0 && remove < len(t.filters) {
		if len(t.filters) == 1 {
			t.filters[0] = newFilterRow() // 只剩一行时 × 是清空这一行
		} else {
			t.filters = append(t.filters[:remove], t.filters[remove+1:]...)
		}
	}
	if add {
		t.filters = append(t.filters, newFilterRow())
	}
}
