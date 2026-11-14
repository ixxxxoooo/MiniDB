package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// ---- 列 ----

func (t *tableTab) structColumns(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	canMove := t.canReorderColumns()
	t.structNameRow(c, p, readOnly)
	structBar(c, p, locales.StructColumns(), len(s.cols), locales.StructAddColumn(), s.addColumn, readOnly)

	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: locales.StructName(), Width: 200, MinWidth: 90},
		{ID: "type", Title: locales.StructType(), Width: 170, MinWidth: 90},
		{ID: "null", Title: locales.StructNullable(), Width: 70, MinWidth: 56},
	}
	if t.isNew { // 新建表才能定主键和自增
		cols = append(cols, ui.TableColumn{ID: "pk", Title: locales.StructPrimary(), Width: 70, MinWidth: 56},
			ui.TableColumn{ID: "ai", Title: locales.StructAutoInc(), Width: 70, MinWidth: 56})
	}
	cols = append(cols, ui.TableColumn{ID: "def", Title: locales.StructDefault(), Width: 170, MinWidth: 70},
		ui.TableColumn{ID: "comment", Title: locales.StructComment(), Width: 360, MinWidth: 80},
		ui.TableColumn{ID: "x", Width: 34, Fixed: true})
	s.colList.Selected = &s.selCol
	s.colList.Key = func(i int) any { return safeKey(i, len(s.cols), func() any { return s.cols[i].UID }) }
	s.colList.Reorder = nil
	if canMove { // 拖动行上下换位：表头旁的「#」格和各个空白处都能抓
		s.colList.Reorder = func(rows []int, to int) { s.cols = moveItems(s.cols, rows, to) }
	}
	s.colList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row < 0 || row >= len(s.cols) {
			return
		}
		if canMove {
			if m.Item(locales.StructMoveUp()).Disabled(row == 0).Chosen() {
				defer func() { s.cols = moveItems(s.cols, []int{row}, row-1); s.selCol = row - 1 }()
			}
			if m.Item(locales.StructMoveDown()).Disabled(row == len(s.cols)-1).Chosen() {
				defer func() { s.cols = moveItems(s.cols, []int{row}, row+2); s.selCol = row + 1 }()
			}
			m.Separator()
		}
		if m.Item(locales.CommonDelete()).Disabled(readOnly).Chosen() {
			defer t.removeColumn(row)
		}
	}
	structTable(c, p, gt, &s.colList, locales.StructColumns(), cols, len(s.cols), func(row int) ui.Color {
		if row < 0 || row >= len(s.cols) {
			return ui.Color{}
		}
		col := &s.cols[row]
		_, existed := s.origCol(col.UID)
		return structRowTint(p, row, s.selCol == row, structTint(p, !existed, s.deleted[col.UID], s.colModified(*col)))
	}, func(row, ci int) {
		if row >= len(s.cols) {
			return
		}
		col := &s.cols[row]
		_, existed := s.origCol(col.UID)
		deleted := s.deleted[col.UID]
		ro := readOnly || deleted
		focus := false // 单元格里的输入框拿到焦点时描上编辑外框
		cell := func(content func()) {
			ringCellBox(c, p, structRowTint(p, row, s.selCol == row, structTint(p, !existed, deleted, s.colModified(*col))), listColW(&s.colList, cols, ci), &focus, content)
		}
		switch cols[ci].ID {
		case "n":
			cell(func() {
				if canMove {
					ui.Icon(c, icGrip).FontSize(11).TextColor(p.fgMuted.Alpha(0.7))
				}
				if s.isPK(col.UID) && existed {
					tipOn(c, ui.Icon(c, icKey).FontSize(11).TextColor(p.warning), locales.StructPrimary())
				} else {
					ui.Text(c, itoa(row+1)).TextColor(p.fgMuted).FontSize(font2XS)
				}
			})
		case "name":
			cell(func() {
				in := cellInput(c, p, "n", &col.Name, ro)
				if s.focusCol == col.UID {
					s.focusCol = ""
					in.Focus()
				}
				focus = in.Focused()
			})
		case "type":
			cell(func() {
				cb := ui.Combobox(c.Key("t"), &col.Type, s.types).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false).Disabled(ro)
				focus = editInput(cb, p.accent).Focused()
			})
		case "null":
			cell(func() {
				ui.Themed(c, &s.ctl).Children(func() { ui.Checkbox(c.Key("nl"), &col.Nullable, "").Disabled(ro) })
			})
		case "def":
			cell(func() {
				def := ""
				if col.DefaultValue != nil {
					def = *col.DefaultValue
				}
				in := cellInput(c, p, "d", &def, ro).Placeholder("NULL")
				focus = in.Focused()
				if in.Changed() {
					if def == "" {
						col.DefaultValue = nil
					} else {
						col.DefaultValue = &def
					}
				}
			})
		case "comment":
			cell(func() { focus = cellInput(c, p, "c", &col.Comment, ro).Focused() })
		case "pk":
			cell(func() {
				ui.Themed(c, &s.ctl).Children(func() { ui.Checkbox(c.Key("pk"), &col.PrimaryKey, "").Disabled(ro) })
			})
		case "ai":
			cell(func() {
				ui.Themed(c, &s.ctl).Children(func() { ui.Checkbox(c.Key("ai"), &col.AutoIncrement, "").Disabled(ro) })
			})
		case "x":
			cell(func() {
				if s.selCol != row || readOnly {
					return
				}
				ic, tip := icTrash, locales.CommonDelete()
				if deleted {
					ic, tip = icUndo, locales.StructDiscard()
				}
				if tipBtn(c.Key("del"), ic, tip).Size(20, 20).Clicked() {
					defer t.removeColumn(row)
				}
			})
		}
	}, func(tbl ui.Element) {
		// 选中行后按 ⌫ / Delete 删除（焦点在单元格的输入框里时，按键归输入框）
		if !readOnly && (tbl.Shortcut(0, ui.KeyBackspace) || tbl.Shortcut(0, ui.KeyDelete)) {
			t.removeColumn(s.selCol)
		}
	})
}

func (s *structState) addColumn() {
	s.nextUID++
	uid := "new:" + itoa(s.nextUID)
	s.cols = append(s.cols, database.StructureColumnEdit{UID: uid, Name: "column_" + itoa(s.nextUID), Type: "varchar(255)", Nullable: true})
	s.selCol, s.focusCol = len(s.cols)-1, uid
	s.colList.ScrollIntoView(len(s.cols) - 1)
}
