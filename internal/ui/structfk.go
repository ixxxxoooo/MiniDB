// @author ixxxxoooo
package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// structFKs 画外键表：名称、列、引用的表和列、删除 / 更新时的动作都是单元格里的控件，
// 已有的外键也能改（提交时先删旧的再建新的）；新增和待删除的行铺色，⌘S 统一提交。
//
// 参数：c 上下文；p 原主题的色板；gt 网格主题；readOnly 连接只读；canFK 这个库能不能改外键。
func (t *tableTab) structFKs(c *ui.Context, p tokens, gt *ui.Theme, readOnly, canFK bool) {
	s := &t.st
	o := &s.obj
	var names []string
	for _, col := range s.cols {
		names = append(names, col.Name)
	}
	tables := t.tableNames()
	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: locales.StructName(), Width: 200, MinWidth: 90},
		{ID: "cols", Title: locales.StructIndexColumns(), Width: 200, MinWidth: 80},
		{ID: "reft", Title: locales.StructFKRefTable(), Width: 170, MinWidth: 80},
		{ID: "refc", Title: locales.StructFKRefColumns(), Width: 200, MinWidth: 80},
		{ID: "del", Title: locales.StructFKOnDelete(), Width: 130, MinWidth: 70},
		{ID: "upd", Title: locales.StructFKOnUpdate(), Width: 130, MinWidth: 70},
		{ID: "x", Width: 34, Fixed: true},
	}
	s.fkList.Selected = &s.selFK
	s.fkList.Key = func(i int) any { return o.fks[i].UID }
	s.fkList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(o.fks) && m.Item(locales.CommonDelete()).Disabled(readOnly || !canFK).Chosen() {
			defer t.removeFK(row)
		}
	}
	tintOf := func(row int) ui.Color {
		fk := &o.fks[row]
		_, existed := o.origFKOf(fk.UID)
		return structRowTint(p, row, s.selFK == row, structTint(p, !existed, o.fkDel[fk.UID]))
	}
	structTable(c, p, gt, &s.fkList, locales.StructForeignKeys(), cols, len(o.fks), tintOf, func(row, ci int) {
		if row >= len(o.fks) {
			return
		}
		fk := &o.fks[row]
		deleted := o.fkDel[fk.UID]
		ro := readOnly || !canFK || deleted
		focus := false
		cell := func(content func()) {
			ringCellBox(c, p, tintOf(row), listColW(&s.fkList, cols, ci), &focus, content)
		}
		drop := func(el ui.Element) ui.Element {
			return editInput(el.FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false), p.accent)
		}
		switch ci {
		case 0:
			cell(func() { ui.Text(c, itoa(row+1)).TextColor(p.fgMuted).FontSize(font2XS) })
		case 1:
			cell(func() {
				in := cellInput(c, p, "fkn", &fk.Name, ro).PlaceholderColor(p.fgMuted)
				if len(fk.Columns) > 0 {
					in.Placeholder(defaultFKName(t.table, fk.Columns))
				}
				if s.focusFK == fk.UID {
					s.focusFK = ""
					in.Focus()
				}
				focus = in.Focused()
			})
		case 2:
			cell(func() {
				tf := drop(ui.TokenField(c.Key("fkc"), &fk.Columns, names))
				if len(fk.Columns) == 0 {
					tf.Placeholder(locales.StructPickColumns())
				}
				focus = tf.Focused()
			})
		case 3:
			cell(func() {
				cb := drop(ui.Combobox(c.Key("fkr"), &fk.RefTable, tables)).Disabled(ro)
				if cb.Changed() {
					fk.RefColumns = nil
				}
				focus = cb.Focused()
			})
		case 4:
			cell(func() {
				tf := drop(ui.TokenField(c.Key("fkrc"), &fk.RefColumns, t.refColumnsOf(fk.RefTable)))
				focus = tf.Focused()
			})
		case 5:
			cell(func() {
				focus = drop(ui.Combobox(c.Key("fkd"), &fk.OnDelete, database.FKActions)).Disabled(ro).Focused()
			})
		case 6:
			cell(func() {
				focus = drop(ui.Combobox(c.Key("fku"), &fk.OnUpdate, database.FKActions)).Disabled(ro).Focused()
			})
		case 7:
			cell(func() {
				if s.selFK != row || readOnly || !canFK {
					return
				}
				ic, tip := icTrash, locales.CommonDelete()
				if deleted {
					ic, tip = icUndo, locales.StructDiscard()
				}
				if tipBtn(c.Key("fkdel"), ic, tip).Size(20, 20).Clicked() {
					defer t.removeFK(row)
				}
			})
		}
	}, func(tbl ui.Element) {
		if !readOnly && canFK && (tbl.Shortcut(0, ui.KeyBackspace) || tbl.Shortcut(0, ui.KeyDelete)) {
			t.removeFK(s.selFK)
		}
	})
}
