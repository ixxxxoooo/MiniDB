package ui

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

type cellKey struct {
	row int
	col string
}

// tableEdits are the uncommitted changes of a table tab. Rows are indexes
// into the displayed rows: those below len(orig) are loaded rows, the rest
// are rows added here.
type tableEdits struct {
	cells   map[cellKey]any
	deleted map[int]bool
}

func (e *tableEdits) count() int { return len(e.cells) + len(e.deleted) }
func (e *tableEdits) clear()     { e.cells, e.deleted = nil, nil }

// tableEditState is what the table tab keeps for editing.
type tableEditState struct {
	info        []model.ColumnInfo
	pk          []string
	orig        []map[string]any // the rows as loaded
	added       int              // rows appended after orig
	defaults    map[string]any
	editing     *cellKey
	editFocused bool // the edit input has had the focus (so losing it ends the edit)
	buf         string
	preview     struct {
		open  bool
		stmts []string
		err   string
		busy  bool
	}
}

func cloneRow(r map[string]any) map[string]any {
	out := make(map[string]any, len(r))
	for k, v := range r {
		out[k] = v
	}
	return out
}

// setRows installs freshly loaded rows and resets all edit state.
func (t *tableTab) setRows(cols []model.ColumnMeta, rows []map[string]any) {
	keep := t.chosenIdentities() // 刷新数据后保持选中的行（行预览要接着看最新的值）
	t.es.orig = rows
	display := make([]map[string]any, len(rows))
	for i, r := range rows {
		display[i] = cloneRow(r)
	}
	t.edits.clear()
	t.es.added, t.es.editing = 0, nil
	t.g.set(cols, display)
	t.restoreChosen(keep)
}

// rowIdentity 用主键值认出一行；没有主键时认不出，返回空串。
func (t *tableTab) rowIdentity(row map[string]any) string {
	if len(t.es.pk) == 0 {
		return ""
	}
	var b strings.Builder
	for _, k := range t.es.pk {
		v, _ := cellText(row[k])
		b.WriteString(v)
		b.WriteByte(0)
	}
	return b.String()
}

// chosenIdentities 记下选中行的主键，供刷新后找回；没有主键的表认不出行，不保留。
func (t *tableTab) chosenIdentities() map[string]bool {
	var ids map[string]bool
	for _, r := range t.chosenRows() {
		if id := t.rowIdentity(t.g.rows[r]); id != "" {
			if ids == nil {
				ids = map[string]bool{}
			}
			ids[id] = true
		}
	}
	return ids
}

// restoreChosen 在新数据里选回刚才选中的行。
func (t *tableTab) restoreChosen(ids map[string]bool) {
	if len(ids) == 0 {
		return
	}
	for i, r := range t.g.rows {
		if ids[t.rowIdentity(r)] {
			t.g.chosen.Add(i)
		}
	}
}

func (t *tableTab) loadStructure() {
	w := t.w
	db := w.db
	var info []model.ColumnInfo
	var defaults map[string]any
	w.sh.up.bg(func() {
		info, _ = w.sh.app.Databases.GetColumns(w.cfg.ID, db, t.table)
		defaults, _ = w.sh.app.Queries.BuildNewRowDefaults(w.cfg.ID, db, t.table)
	}, func() {
		t.es.info, t.es.defaults = info, defaults
		t.es.pk = nil
		for _, c := range info {
			if c.IsPrimary {
				t.es.pk = append(t.es.pk, c.Name)
			}
		}
	})
}

func (t *tableTab) hasPK() bool { return len(t.es.pk) > 0 }

// writable says whether edits are allowed, toasting why not.
func (t *tableTab) writable(row int) bool {
	if t.w.cfg.ReadOnly {
		t.w.sh.toastWarn(locales.EditReadOnly())
		return false
	}
	if row < len(t.es.orig) && !t.hasPK() {
		t.w.sh.toastWarn(locales.EditNoPrimaryKey())
		return false
	}
	return true
}

func (t *tableTab) beginEdit(row int, col string) {
	if !t.writable(row) || t.edits.deleted[row] {
		return
	}
	v, null := cellText(t.g.rows[row][col])
	if null {
		v = ""
	}
	t.es.editing, t.es.buf, t.es.editFocused = &cellKey{row, col}, v, false
}

func (t *tableTab) cancelEdit() { t.es.editing = nil }

// commitEdit stores the typed text as the cell's new value. Typing nothing
// into a NULL cell leaves it NULL.
func (t *tableTab) commitEdit() {
	k := t.es.editing
	t.es.editing = nil
	if k == nil || k.row >= len(t.g.rows) {
		return
	}
	cur := t.g.rows[k.row][k.col]
	if cur == nil && t.es.buf == "" {
		return
	}
	t.setCell(k.row, k.col, t.es.buf)
}

func (t *tableTab) setCell(row int, col string, v any) {
	if row >= len(t.g.rows) {
		return
	}
	t.g.rows[row][col] = v
	if row >= len(t.es.orig) { // an added row: its display value is the value
		return
	}
	key := cellKey{row, col}
	if reflect.DeepEqual(plainText(t.es.orig[row][col]), plainText(v)) && (t.es.orig[row][col] == nil) == (v == nil) {
		delete(t.edits.cells, key)
		return
	}
	if t.edits.cells == nil {
		t.edits.cells = map[cellKey]any{}
	}
	t.edits.cells[key] = v
}

func plainText(v any) string { s, _ := cellText(v); return s }

func (t *tableTab) addRow() {
	if !t.writable(len(t.es.orig)) {
		return
	}
	row := map[string]any{}
	for _, c := range t.g.cols {
		row[c.Name] = nil
	}
	for k, v := range t.es.defaults {
		row[k] = v
	}
	t.g.rows = append(t.g.rows, row)
	t.es.added++
	t.g.chosen.Clear()
	t.g.chosen.Add(len(t.g.rows) - 1)
	t.g.list.ScrollToEnd()
}

func (t *tableTab) deleteSelected() {
	var rows []int
	for r := range t.g.chosen.All() {
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		t.w.sh.toastWarn(locales.EditNothingSelected())
		return
	}
	sort.Sort(sort.Reverse(sort.IntSlice(rows)))
	for _, r := range rows {
		if !t.writable(r) {
			return
		}
	}
	for _, r := range rows {
		switch {
		case r >= len(t.es.orig): // an added row just goes away
			t.removeAdded(r)
		case t.edits.deleted[r]:
			delete(t.edits.deleted, r)
		default:
			if t.edits.deleted == nil {
				t.edits.deleted = map[int]bool{}
			}
			t.edits.deleted[r] = true
		}
	}
	t.g.chosen.Clear()
}

func (t *tableTab) removeAdded(r int) {
	t.g.rows = append(t.g.rows[:r], t.g.rows[r+1:]...)
	t.es.added--
}

func (t *tableTab) revert() {
	t.setRows(t.g.cols, t.es.orig)
}

// pendingCount includes added rows, which `count` (cells and deletes) lacks.
func (t *tableTab) pendingCount() int { return t.edits.count() + t.es.added }

// changeSet turns the edits into the arguments of CommitTableDataChanges.
func (t *tableTab) changeSet() (deletes, inserts []map[string]any, updates []app.RowUpdate) {
	pkOf := func(row int) map[string]any {
		pk := make(map[string]any, len(t.es.pk))
		for _, c := range t.es.pk {
			pk[c] = t.es.orig[row][c]
		}
		return pk
	}
	var delRows []int
	for r := range t.edits.deleted {
		delRows = append(delRows, r)
	}
	sort.Ints(delRows)
	for _, r := range delRows {
		deletes = append(deletes, pkOf(r))
	}
	for r := len(t.es.orig); r < len(t.g.rows); r++ {
		row := map[string]any{}
		for _, c := range t.g.cols {
			if v := t.g.rows[r][c.Name]; v != nil {
				row[c.Name] = v
			}
		}
		if len(row) > 0 {
			inserts = append(inserts, row)
		}
	}
	byRow := map[int]map[string]any{}
	for k, v := range t.edits.cells {
		if t.edits.deleted[k.row] {
			continue
		}
		if byRow[k.row] == nil {
			byRow[k.row] = map[string]any{}
		}
		byRow[k.row][k.col] = v
	}
	var rows []int
	for r := range byRow {
		rows = append(rows, r)
	}
	sort.Ints(rows)
	for _, r := range rows {
		updates = append(updates, app.RowUpdate{PrimaryKey: pkOf(r), Changes: byRow[r]})
	}
	return
}

// buildPreview fills in the statements a commit would run (for tests and logs).
func (t *tableTab) buildPreview() {
	deletes, inserts, updates := t.changeSet()
	rc := make([]app.RowChange, len(updates))
	for i, u := range updates {
		rc[i] = app.RowChange{PrimaryKey: u.PrimaryKey, Changes: u.Changes}
	}
	t.es.preview.stmts = app.PreviewChanges(t.w.cfg.Type, t.w.db, t.table, deletes, inserts, rc)
	t.es.preview.err = ""
}

func (t *tableTab) commit() {
	p := &t.es.preview
	deletes, inserts, updates := t.changeSet()
	w := t.w
	db := w.db
	n := len(deletes) + len(inserts) + len(updates)
	p.busy, p.err = true, ""
	stmts := append([]string(nil), p.stmts...)
	started := time.Now()
	var err error
	w.sh.up.bg(func() {
		err = w.sh.app.Queries.CommitTableDataChanges(w.cfg.ID, db, t.table, deletes, inserts, updates)
	}, func() {
		p.busy = false
		errText := ""
		if err != nil {
			errText = errMessage(err)
		}
		w.sh.logCommit(db, stmts, n, time.Since(started), errText)
		if err != nil {
			p.err = locales.EditCommitFail(errMessage(err))
			w.sh.toastErr(p.err)
			return
		}
		w.sh.toastOK(locales.EditCommitted(n))
		t.load()
	})
}

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

// commitNow writes the pending edits at once, as the Commit button and ⌘S do,
// without a dialog in between; the connection's Safe Mode may ask first.
func (t *tableTab) commitNow() {
	if t.es.editing != nil {
		t.commitEdit() // the cell being typed in counts too
	}
	if t.es.preview.busy || t.pendingCount() == 0 {
		return
	}
	t.buildPreview()
	t.w.sh.safeGate(t.w, t.es.preview.stmts, t.commit)
}

var _ = strings.TrimSpace
