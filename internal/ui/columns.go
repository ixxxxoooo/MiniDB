package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// columnsDialog 是列管理（⌘⌥F）：搜索、显示/隐藏、固定、调整顺序、全选和重置。
// 显示顺序和表头里拖动的是同一份，固定的列排在最前。
func (t *tableTab) columnsDialog(c *ui.Context) {
	if !t.colsOpen {
		return
	}
	sh := t.w.sh
	p := sh.tokens(c)
	g := t.g
	sh.dialog(c, &t.colsOpen, dlgOpts{title: locales.TableColumnsTitle(), sub: t.table, width: 460, extra: func() {
		if actBtn(c, icCheckAll, locales.TableColumnsAll()).Clicked() {
			g.hidden = nil
			g.rebuild()
			g.fitWidths()
			g.trackLayout()
		}
		if actBtn(c, icX, locales.TableColumnsNone()).Clicked() {
			for _, name := range g.displayOrder()[1:] { // 至少留第一列
				g.setHidden(name, true)
			}
		}
		if actBtn(c, icUndo, locales.ColReset()).Clicked() {
			g.resetLayout()
		}
	}}, func() {
		ui.SearchField(c.Key("colsq"), &t.colsQ).Placeholder(locales.TableColumnsSearch()).Height(sizeBtn).FillWidth()
		q := strings.ToLower(strings.TrimSpace(t.colsQ))
		dlgBox(c, p).FillWidth().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("colslist")).MaxHeight(340).FillWidth()).Children(func() {
				ui.Column(c).Padding(6).Gap(1).FillWidth().Children(func() {
					for _, name := range g.displayOrder() {
						if q != "" && !strings.Contains(strings.ToLower(name), q) {
							continue
						}
						t.columnRow(c, p, name)
					}
				})
			})
		})
	}, nil)
}

// columnRow 是列管理里的一行：显示开关、名字、类型，右边是固定和上下移动。
func (t *tableTab) columnRow(c *ui.Context, p tokens, name string) {
	g := t.g
	typ := g.colMeta(name).Type
	shown, pinned := !g.hidden[name], g.pinned[name]
	ui.Row(c.Key(name)).Gap(6).AlignItems(ui.Center).Padding(2, 6).FillWidth().Children(func() {
		if ui.Checkbox(c.Key("cb"), &shown, "").Changed() {
			g.setHidden(name, !shown)
		}
		ui.Text(c, name).Grow(1).MinWidth(0).SingleLine().Font(mono).FontSize(font2XS)
		ui.Text(c, typ).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Shrink(0)
		tip := locales.ColPin()
		if pinned {
			tip = locales.ColUnpin()
		}
		if toggleBtn(c.Key("pin"), icPin, tip, pinned).Clicked() {
			g.togglePin(name)
		}
		if tipBtn(c.Key("up"), icSortUp, locales.ColMoveUp()).Size(sizeBtnSm, sizeBtnSm).Disabled(pinned).Clicked() {
			g.moveColumn(name, -1)
		}
		if tipBtn(c.Key("down"), icSortDown, locales.ColMoveDown()).Size(sizeBtnSm, sizeBtnSm).Disabled(pinned).Clicked() {
			g.moveColumn(name, 1)
		}
	})
}

// hiddenCount is the number of columns left out of the grid.
func (t *tableTab) hiddenCount() int { return len(t.g.hidden) }

// layoutKey 是这张表的列布局在 UIStateService 里的键：连接、库、表。
func (t *tableTab) layoutKey() string { return t.w.dbKey() + "\x00" + t.table }

// bindLayout 让网格记住这张表的列布局：载入上次保存的，之后的变化后台存盘。
func (t *tableTab) bindLayout() {
	if t.table == "" || t.isNew {
		return
	}
	key, ui := t.layoutKey(), t.w.sh.app.UIState
	var l colLayout
	if ui.LoadColumns(key, &l) == nil {
		t.g.pending = &l
	}
	t.g.onLayout = func(l colLayout) {
		t.w.sh.up.bg(func() { _ = ui.SaveColumns(key, l) }, func() {})
	}
	t.g.onManage = func() { t.colsOpen = true }
}
