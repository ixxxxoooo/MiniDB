// @author ixxxxoooo
package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// structTriggers 画触发器页：左边是表（名称、时机、事件、语句的第一行），右边是选中那个触发器的语句编辑器，
// 和 TablePlus 一样。新增的触发器名称、时机、事件都在单元格里填，语句写在右边，写的是触发的内容；
// 已有的触发器只能改语句，右边是完整的 CREATE TRIGGER，提交时先删旧的再执行它。
//
// 参数：c 上下文；p 原主题的色板；gt 网格主题；readOnly 连接只读。
func (t *tableTab) structTriggers(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	ui.SplitPercent(c, &s.trigSplit, func() {
		ui.Column(c).Fill().Children(func() { t.trigGrid(c, p, gt, readOnly) })
	}, func() {
		ui.Column(c).Fill().Background(p.surface).Children(func() { t.trigEditor(c, p, readOnly) })
	}).Grow(1).FillWidth()
}

// trigGrid 是触发器的表格。
func (t *tableTab) trigGrid(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	o := &s.obj
	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: locales.StructName(), Width: 180, MinWidth: 90},
		{ID: "timing", Title: locales.StructTriggerTiming(), Width: 100, MinWidth: 60},
		{ID: "event", Title: locales.StructTriggerEvent(), Width: 100, MinWidth: 60},
		{ID: "body", Title: locales.StructTriggerBody(), Width: 300, MinWidth: 100},
		{ID: "x", Width: 34, Fixed: true},
	}
	s.trigList.Selected = &s.selTrig
	s.trigList.Key = func(i int) any { return safeKey(i, len(o.trigs), func() any { return o.trigs[i].UID }) }
	s.trigList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(o.trigs) && m.Item(locales.CommonDelete()).Disabled(readOnly).Chosen() {
			defer t.removeTrigger(row)
		}
	}
	tintOf := func(row int) ui.Color {
		if row < 0 || row >= len(o.trigs) {
			return ui.Color{}
		}
		tr := &o.trigs[row]
		_, existed := o.origTrigOf(tr.UID)
		return structRowTint(p, row, s.selTrig == row, structTint(p, !existed, o.trigDel[tr.UID], o.trigModified(*tr)))
	}
	structTable(c, p, gt, &s.trigList, locales.StructTriggers(), cols, len(o.trigs), tintOf, func(row, ci int) {
		if row >= len(o.trigs) {
			return
		}
		tr := &o.trigs[row]
		_, existed := o.origTrigOf(tr.UID)
		deleted := o.trigDel[tr.UID]
		edit := !existed && !readOnly // 已有的触发器名称、时机、事件不能改，要改请删了重建
		focus := false
		cell := func(content func()) {
			ringCellBox(c, p, tintOf(row), listColW(&s.trigList.ListState, cols, ci), &focus, content)
		}
		pick := func(el ui.Element) ui.Element {
			return editInput(el.FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false), p.accent)
		}
		switch ci {
		case 0:
			cell(func() { ui.Text(c, itoa(row+1)).TextColor(p.fgMuted).FontSize(font2XS) })
		case 1:
			cell(func() {
				if !edit {
					ui.Text(c, tr.Name).SingleLine()
					return
				}
				in := cellInput(c, p, "trn", &tr.Name, false)
				if s.focusTrig == tr.UID {
					s.focusTrig = ""
					in.Focus()
				}
				focus = in.Focused()
			})
		case 2:
			cell(func() {
				if !edit {
					ui.Text(c, tr.Timing).SingleLine().TextColor(p.fgSecondary)
					return
				}
				focus = pick(ui.Combobox(c.Key("trt"), &tr.Timing, app.TriggerTimings)).Focused()
			})
		case 3:
			cell(func() {
				if !edit {
					ui.Text(c, tr.Event).SingleLine().TextColor(p.fgSecondary)
					return
				}
				focus = pick(ui.Combobox(c.Key("tre"), &tr.Event, app.TriggerEvents)).Focused()
			})
		case 4:
			cell(func() { ui.Text(c, oneLine(firstLines(tr.Definition, 1))).SingleLine().TextColor(p.fgSecondary) })
		case 5:
			cell(func() {
				if s.selTrig != row || readOnly {
					return
				}
				ic, tip := icTrash, locales.CommonDelete()
				if deleted {
					ic, tip = icUndo, locales.StructDiscard()
				}
				if tipBtn(c.Key("trgdel"), ic, tip).Size(20, 20).Clicked() {
					defer t.removeTrigger(row)
				}
			})
		}
	}, func(tbl ui.Element) {
		if !readOnly && (tbl.Shortcut(0, ui.KeyBackspace) || tbl.Shortcut(0, ui.KeyDelete)) {
			t.removeTrigger(s.selTrig)
		}
	})
}

// trigEditor 是右边的语句编辑器：编辑选中那个触发器的语句，没选中时给一句提示。
func (t *tableTab) trigEditor(c *ui.Context, p tokens, readOnly bool) {
	s := &t.st
	o := &s.obj
	if s.selTrig < 0 || s.selTrig >= len(o.trigs) {
		ui.Text(c, locales.StructTriggerPick()).TextColor(p.fgMuted).FontSize(font2XS).Padding(12)
		return
	}
	tr := &o.trigs[s.selTrig]
	_, existed := o.origTrigOf(tr.UID)
	hint := locales.StructTriggerBody()
	if existed {
		hint = locales.StructTriggerFullHint()
	}
	ui.Row(c.Key("trgbar")).Padding(0, 12).Height(sizeTab).FillWidth().Shrink(0).AlignItems(ui.Center).Background(p.toolbar).Children(func() {
		ui.Text(c, hint).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine()
	})
	hairline(c, p.border)
	ro := readOnly || o.trigDel[tr.UID]
	in := ui.TextArea(c.Key("trgdef-"+tr.UID), &tr.Definition).Font(mono).FontSize(font2XS+1).Padding(8, 12).
		Grow(1).FillWidth().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).ReadOnly(ro)
	editInput(in, p.accent)
}
