package ui

import (
	"github.com/egoist/mygo/ui"
)

// listColW 是网格第 i 列此刻的宽度：用户拖过的，或者起初定的。
func listColW(ls *ui.ListState, cols []ui.TableColumn, i int) float32 {
	col := cols[i]
	if w, ok := ls.Columns.Widths[col.ID]; ok && !col.Fixed {
		return w
	}
	return col.Width
}

// structTable 画结构视图里的一张网格。keys 在表格元素上登记键盘快捷键（比如 ⌫ 删除选中的行）。
//
// 列宽加起来不满一行时，右边另有一列自动补满（filler），tint 给出一行的底色，
// 让斑马纹和新增 / 删除的底色从左到右连成一片，不留空白。
func structTable(c *ui.Context, p tokens, gt *ui.Theme, tl *tailList, label string, cols []ui.TableColumn, n int,
	tint func(row int) ui.Color, cell func(row, col int), keys func(tbl ui.Element)) {
	filled := append(append([]ui.TableColumn(nil), cols...), ui.TableColumn{ID: "fill", Fixed: true})
	real := len(cols)
	body := cell
	cell = func(row, col int) {
		if col < real {
			body(row, col)
			return
		}
		if row < n {
			t := tint(row)
			recordRowTail(tl, cellBox(c, t, 0, func() {}), t)
		}
	}
	cols = filled
	ui.Themed(c, gt).Background(ui.Transparent).FixedLineHeight(14).Grow(1).FillWidth().MinWidth(0).Children(func() {
		tbl := ui.Table(c, &tl.ListState, cols, n, cell).Grow(1).MinWidth(0).Label(label)
		tbl = listBars(c, tbl, &tl.ListState)
		paintGridBg(tbl, n, p.surfaceSecondary, c.Theme().Space(8)+1, showZebra, func(pt *ui.Painter, r ui.Rect) {
			paintRowTails(pt, tl, r.X+r.W)
		})
		if keys != nil {
			keys(tbl)
		}
	})
}

// safeKey 给列表行取稳定的键：行号越界时（这一帧里数据刚被删短，列表还在按旧行数取）退回行号本身，不崩溃。
//
// 参数：i 行号；n 当前数据的长度；key 行号有效时取键的函数。
func safeKey(i, n int, key func() any) any {
	if i < 0 || i >= n {
		return i
	}
	return key()
}

// structRowTint 是一行的底色：新增 / 待删除的行用状态色，其余的行按斑马纹隔行铺底。
// 表格没有开单元格选择（要留给拖动排序），库自带的斑马纹用不上，所以由单元格自己铺；
// 选中的行不铺，免得盖住选中的底色。
func structRowTint(p tokens, row int, selected bool, state ui.Color) ui.Color {
	if state != (ui.Color{}) {
		return state
	}
	if showZebra && row%2 == 1 && !selected {
		return p.surfaceSecondary
	}
	return ui.Color{}
}

// structTint 是结构页里行的状态底色，和数据行一致：新增绿、待删除红、改过黄（提交之前一直留着）。
// 没有状态则返回零值。
func structTint(p tokens, isNew, deleted, modified bool) ui.Color {
	switch {
	case deleted:
		return p.rowDelete
	case isNew:
		return p.rowNew
	case modified:
		return p.cellEdit
	}
	return ui.Color{}
}

// cellInput is a text field that looks like a grid cell until it is used.
func cellInput(c *ui.Context, p tokens, key string, v *string, ro bool) ui.Element {
	in := ui.TextInput(c.Key(key), v).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false).ReadOnly(ro)
	return editInput(in, p.accent)
}

// structBar 是一块网格上方的标题条：标题、个数，右边是添加按钮。
func structBar(c *ui.Context, p tokens, title string, n int, addLabel string, add func(), disabled bool) {
	ui.Row(c.Key("bar-"+title)).Gap(6).AlignItems(ui.Center).Padding(0, 6, 1, 12).Height(sizeTab).FillWidth().Shrink(0).Background(p.toolbar).Children(func() {
		ui.Text(c, title).FontWeight(600).FontSize(font2XS)
		ui.Text(c, "("+itoa(n)+")").FontSize(font2XS).TextColor(p.fgMuted)
		ui.Spacer(c)
		if actBtn(c.Key("add"), icPlus, addLabel).Disabled(disabled).Clicked() {
			add()
		}
	})
	hairline(c, p.border)
}
