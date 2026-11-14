package ui

import (
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"

	"minidb/locales"
)

// drawCell draws one data cell: editing input, or text tinted by its state.
func (t *tableTab) drawCell(c *ui.Context, row, col int, selected bool) bool {
	name := t.g.col(col).Name
	if k := t.es.editing; k != nil && k.row == row && k.col == name {
		p := baseTokens(c)
		var in ui.Element
		// 输入框铺满整个单元格（含单元格自己的内边距，和选中态的底色一样大），
		// 行高不变；外面再描一圈强调色。
		in = ui.Element{}
		th := c.Theme()
		box := ui.Row(c).Width(t.g.colWidth(col)).Height(gridRowH).Margin(-th.Space(1.5), -th.Space(2.5)).Background(p.surface)
		if t.g.colWidth(col) <= 0 {
			box = ui.Row(c).FillWidth().Height(gridRowH).Margin(-th.Space(1.5), -th.Space(2.5)).Background(p.surface)
		}
		box.Children(func() {
			in = ui.TextInput(c.Key("cell-edit"), &t.es.buf).AutoFocus().FillWidth().Height(gridRowH).Padding(0, th.Space(2.5)).
				Font(mono).FontSize(font2XS).BorderWidth(0).Background(p.surface).FocusRing(false)
		})
		box.DrawOver(func(pt *ui.Painter, r ui.Rect) { paintEditRing(pt, r, p.accent) })
		if in.Submitted() {
			t.commitEdit()
		}
		// leaving the cell by clicking elsewhere keeps what was typed
		if !in.Focused() && t.es.editFocused {
			t.es.editFocused = false
			t.commitEdit()
			return false
		}
		if in.Focused() {
			t.es.editFocused = true
		}
		if c.Shortcut(0, ui.KeyEscape) {
			t.cancelEdit()
		}
		return true
	}
	p := baseTokens(c)
	text, null := cellText(t.g.rows[row][name])
	color := valueColor(p, t.g.col(col).Type)
	if null {
		color, text = p.fgMuted, locales.TableNull()
	}
	// The original's row states: new rows green, deleted red, edited cells yellow.
	var tint ui.Color
	switch {
	case t.edits.deleted[row]:
		tint = p.rowDelete
	case row >= len(t.es.orig):
		tint = p.rowNew
	default:
		if _, ok := t.edits.cells[cellKey{row, name}]; ok {
			tint = p.cellEdit
		}
	}
	jump := false
	cell := cellBox(c, tint, t.g.colWidth(col), func() {
		tx := ui.Text(c, oneLine(text)).Font(mono).FontSize(font2XS).FixedLineHeight(14).TextColor(color).SingleLine()
		if null {
			tx.Italic()
		}
		// a foreign key shows an arrow once its cell is chosen: it opens the referenced row
		if selected && !null && t.foreignKey(name) != "" {
			ui.Spacer(c)
			arrow := tipOn(c, ui.Button(c.Key("fk"), "").Label(locales.TableGotoForeignKey(t.foreignKey(name))), locales.TableGotoForeignKey(t.foreignKey(name))).Size(16, 14).Padding(0).BorderWidth(0).Background(ui.Color{}).Children(func() {
				ui.Icon(c, icChevronR).FontSize(11).TextColor(p.accent)
			})
			jump = arrow.Clicked()
		}
	})
	if !null && text != "" {
		bigTip(c, cell, clipText(text, 500, 14)) // 悬浮看完整内容，太长就截断
	}
	if jump {
		t.followForeignKey(row, name)
		return true
	}
	if cell.RowDoubleClicked() {
		t.beginEdit(row, name)
	}

	return true
}

// cellMenuExtras adds editing items to a cell's context menu.
// foreignKey is where a column points ("table.column"), or "".
func (t *tableTab) foreignKey(col string) string {
	for _, c := range t.es.info {
		if c.Name == col {
			return c.ForeignKey
		}
	}
	return ""
}

// followForeignKey opens the referenced table filtered to the row this cell points at.
func (t *tableTab) followForeignKey(row int, col string) {
	ref := t.foreignKey(col)
	if ref == "" || row < 0 || row >= len(t.g.rows) {
		return
	}
	v, null := cellText(t.g.rows[row][col])
	if null {
		t.w.sh.toastWarn(locales.TableForeignKeyNull())
		return
	}
	table, refCol := app.SplitForeignKey(ref)
	t.w.openTableFiltered(table, refCol, v)
}

func (t *tableTab) cellMenuExtras(m *ui.Menu, row, col int) {
	if col <= 0 || row < 0 || row >= len(t.g.rows) {
		return
	}
	name := t.g.col(col).Name
	m.Separator()
	if ref := t.foreignKey(name); ref != "" && m.Item(locales.TableGotoForeignKey(ref)).Chosen() {
		defer t.followForeignKey(row, name)
	}
	if m.Item(locales.EditCell()).Chosen() {
		t.beginEdit(row, name)
	}
	t.setValueMenu(m, row, name)
	if m.Item(locales.TableQuickLook()).Shortcut(ui.Cmd, ui.KeyEnter).Chosen() {
		t.openQuickLook(row, name)
	}
	if m.Item(locales.TableDuplicateRows()).Shortcut(ui.Cmd, ui.KeyD).Chosen() {
		if t.g.chosen.Len() == 0 {
			t.g.chosen.Add(row)
		}
		t.duplicateRows()
	}
	if m.Item(locales.EditDeleteRows()).Chosen() {
		if t.g.chosen.Len() == 0 {
			t.g.chosen.Add(row)
		}
		t.deleteSelected()
	}
}
