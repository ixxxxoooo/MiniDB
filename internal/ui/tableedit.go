package ui

import (
	"reflect"
	"sort"
	"strings"

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
	if !t.w.rowEditable() {
		t.w.sh.toastWarn(locales.EditEngineReadOnly(t.w.engineLabel()))
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
	if _, ok := blobOf(t.g.colMeta(col), t.g.rows[row][col]); ok {
		t.openQuickLook(row, col) // 二进制值在 Quick Look 里用十六进制改
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
