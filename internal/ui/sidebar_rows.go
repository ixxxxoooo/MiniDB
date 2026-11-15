package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 侧栏对象列表的分组标题和表行（设计稿 A 的 .sec-h / .tr）：分组标题 24px、10.5px 大写灰字、右端计数；
// 表行 26px、图标 13、12.5px 次要文字色，选中的是强调淡底 + 强调文字；列表两侧各留 6px。

// sideInset 是侧栏列表行距侧栏边缘的距离。
const sideInset = 6

func (w *Workspace) sectionHeader(c *ui.Context, p tokens, r sideRow) {
	open := !w.side.collapsed[r.section]
	row := ui.Row(c.Key("sec-"+r.section)).Gap(spaceSM-2).AlignItems(ui.Center).Padding(0, spaceSM).Height(sizeSideSub).Radius(radiusBtn).Margin(spaceXS, sideInset, 0, sideInset)
	if row.Hovered() {
		row.Background(p.hover)
	}
	// 悬浮提示告诉用户这一栏可以收起 / 展开
	tip := locales.SidebarCollapse()
	if !open {
		tip = locales.SidebarExpand()
	}
	tipOn(c, row, tip)
	hover := row.Hovered()
	overview, newTbl := false, false
	row.Children(func() {
		ic := icChevronD
		if !open {
			ic = icChevronR
		}
		ui.Icon(c, ic).FontSize(fontCaption).TextColor(p.fgFaint)
		ui.Text(c, r.label).FontSize(fontCaption - 0.5).FontWeight(600).LetterSpacing(0.5).TextColor(p.fgMuted)
		ui.Spacer(c)
		// 表 / 视图这两栏：悬停时右端出现「项目概览」入口（右键标题也有），平时是计数
		if hover && r.section == "tables" && !w.cfg.ReadOnly && app.SupportsVisualCreateTable(w.cfg.Type) {
			if tipBtn(c.Key("new-"+r.section), icPlus, locales.TableNew()).Size(sizeChipSq, sizeChipSq).Clicked() {
				newTbl = true
			}
		}
		if hover && (r.section == "tables" || r.section == "views" || r.section == "pinned") {
			if tipBtn(c.Key("ov-"+r.section), icTable, locales.OverviewTitle()).Size(sizeChipSq, sizeChipSq).Clicked() {
				overview = true
			}
		} else {
			ui.Text(c, itoa(r.count)).FontSize(fontCaption - 0.5).FontWeight(500).TextColor(p.fgMuted)
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.OverviewTitle()).Chosen() {
			defer w.openOverview()
		}
	})
	if newTbl {
		w.newTable()
		return
	}
	if overview {
		w.openOverview()
		return
	}
	if row.Clicked() {
		if w.side.collapsed == nil {
			w.side.collapsed = map[string]bool{}
		}
		w.side.collapsed[r.section] = open
	}
}

func (w *Workspace) itemRow(c *ui.Context, p tokens, r sideRow) {
	if r.routineName != "" {
		w.routineRow(c, p, r)
		return
	}
	info := r.info
	picked := w.side.picked == info.Name
	if tt, ok := w.activeTab().(*tableTab); ok && !tt.isNew && tt.table != "" {
		picked = tt.table == info.Name // 设计稿：侧栏高亮的是当前标签页打开的那张表
	}
	kind := KindTable
	if isView(info) {
		kind = KindView
	}
	ic := objectIcon(kind)
	row := ui.Row(c.Key(r.section+"/"+info.Name)).Gap(spaceSM-1).AlignItems(ui.Center).Padding(0, spaceSM).Height(sizeRow).Radius(radiusBtn).Margin(0, sideInset)
	// 悬浮显示完整表名（名字被侧栏截断时也看得全），有备注就附在下面
	tip := info.Name
	if strings.TrimSpace(info.Comment) != "" {
		tip += "\n" + strings.TrimSpace(info.Comment)
	}
	tipOn(c, row, tip)
	dragTable(row, info.Name) // 可以拖进 AI 助手的输入框引用
	row.Focusable()           // 点一下选中后键盘归这一行：⌥⌫ 清空、Delete 删除，和 TablePlus 一样
	if picked {
		row.Background(p.accentSoft)
	} else if row.Hovered() {
		row.Background(p.hover) // 鼠标扫过：灰色底，和点选的强调色底区分开
	}
	row.Children(func() {
		col, txt := iconColor(p, kind), p.fgSecondary
		if row.Hovered() {
			txt = p.fg
		}
		if picked {
			txt = p.accentText
		}
		ui.Icon(c, ic).FontSize(btnActIcon).TextColor(col)
		t := ui.Text(c, info.Name).FontSize(fontCtl).TextColor(txt).SingleLine().Grow(1)
		if picked {
			t.FontWeight(550)
		}
		sideRowTail(c, p, r)
	})
	switch {
	case row.DoubleClicked():
		w.side.picked = info.Name
		w.openTable(info.Name)
	case row.Clicked():
		w.side.picked = info.Name
		w.previewTable(info.Name)
	}
	row.ContextMenu(func(m *ui.Menu) { w.tableMenu(m, info.Name) })
	if picked && !w.cfg.ReadOnly {
		w.tableRowKeys(row, info.Name)
	}
}

// tableRowKeys 是选中的表行拿着键盘时的快捷键：⌥⌫ 清空表，单按 Delete 删除表（视图则删除视图），都先确认。
func (w *Workspace) tableRowKeys(row ui.Element, name string) {
	switch {
	case row.Shortcut(ui.Alt, ui.KeyBackspace):
		w.truncateTable(name)
	case row.Shortcut(0, ui.KeyBackspace), row.Shortcut(0, ui.KeyDelete):
		if w.isViewName(name) {
			w.dropObject(KindView, name)
		} else {
			w.dropTable(name)
		}
	}
}
