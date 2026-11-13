// @author ixxxxoooo
package ui

import (
	"slices"
	"strings"

	"minidb/internal/database"
	"minidb/locales"
)

// 结构视图里外键与触发器的工作副本。和列、索引一样：原始行用原名当 UID，
// 新增的行是 "new:N"；改动都只留在本地，⌘S 时和列、索引一起提交。

// canEditFK reports whether the database can add or drop a foreign key on an existing table.
func canEditFK(dbType string) bool { return dbType != "sqlite" && dbType != "starrocks" }

// canTriggers reports whether the database has triggers this app can list.
func canTriggers(dbType string) bool { return dbType != "starrocks" }

// objState 是外键和触发器的工作副本：orig 是读出来的原样，fks / trigs 是正在改的。
type objState struct {
	origFK   []database.StructureFKEdit
	fks      []database.StructureFKEdit
	fkDel    map[string]bool
	origTrig []database.StructureTriggerEdit
	trigs    []database.StructureTriggerEdit
	trigDel  map[string]bool

	refCols map[string][]string // 被引用表的列，按表名缓存
	refBusy map[string]bool     // 正在读列的表
}

// loadObjects 读出表的外键和触发器，作为原始行并重置工作副本。
func (s *structState) loadObjects(t *tableTab) {
	w, db := t.w, t.w.db
	var fks []database.ForeignKeyInfo
	var trigs []database.TriggerInfo
	w.sh.up.bg(func() {
		fks, _ = w.sh.app.Databases.GetForeignKeys(w.cfg.ID, db, t.table)
		trigs, _ = w.sh.app.Databases.GetTriggers(w.cfg.ID, db, t.table)
	}, func() {
		s.obj.origFK, s.obj.origTrig = nil, nil
		for _, f := range fks {
			s.obj.origFK = append(s.obj.origFK, database.StructureFKEdit{
				UID: f.Name, Name: f.Name, Columns: f.Columns, RefTable: f.RefTable, RefColumns: f.RefColumns,
				OnDelete: fkAction(f.OnDelete), OnUpdate: fkAction(f.OnUpdate),
			})
		}
		for _, tr := range trigs {
			s.obj.origTrig = append(s.obj.origTrig, database.StructureTriggerEdit{
				UID: tr.Name, Name: tr.Name, Timing: tr.Timing, Event: tr.Event, Definition: tr.Definition,
			})
		}
		s.obj.reset()
	})
}

// fkAction 把读出来的外键动作规整成可选项之一，空的当作 NO ACTION。
func fkAction(a string) string {
	if a = strings.ToUpper(strings.TrimSpace(a)); a == "" {
		return "NO ACTION"
	}
	return a
}

// reset 丢掉未提交的改动，工作副本回到原样。
func (o *objState) reset() {
	o.fks = cloneFKs(o.origFK)
	o.trigs = append([]database.StructureTriggerEdit(nil), o.origTrig...)
	o.fkDel, o.trigDel = map[string]bool{}, map[string]bool{}
}

func cloneFKs(in []database.StructureFKEdit) []database.StructureFKEdit {
	out := append([]database.StructureFKEdit(nil), in...)
	for i := range out {
		out[i].Columns = slices.Clone(out[i].Columns)
		out[i].RefColumns = slices.Clone(out[i].RefColumns)
	}
	return out
}

func (o *objState) origFKOf(uid string) (database.StructureFKEdit, bool) {
	for _, f := range o.origFK {
		if f.UID == uid {
			return f, true
		}
	}
	return database.StructureFKEdit{}, false
}

func (o *objState) origTrigOf(uid string) (database.StructureTriggerEdit, bool) {
	for _, tr := range o.origTrig {
		if tr.UID == uid {
			return tr, true
		}
	}
	return database.StructureTriggerEdit{}, false
}

// workingFKs 给每个外键定状态：新增、待删除、改过（和原来比有不同）或没动。
// 新增的外键名字留空时按表名 table 取默认名。
func (o *objState) workingFKs(table string) []database.StructureFKEdit {
	out := make([]database.StructureFKEdit, 0, len(o.fks))
	for _, f := range o.fks {
		old, existed := o.origFKOf(f.UID)
		switch {
		case !existed:
			f.Status = "new"
			if strings.TrimSpace(f.Name) == "" && len(f.Columns) > 0 {
				f.Name = defaultFKName(table, f.Columns)
			}
		case o.fkDel[f.UID]:
			f.Status = "deleted"
		case f.Name != old.Name || f.RefTable != old.RefTable || f.OnDelete != old.OnDelete || f.OnUpdate != old.OnUpdate ||
			!slices.Equal(f.Columns, old.Columns) || !slices.Equal(f.RefColumns, old.RefColumns):
			f.Status = "modified"
		default:
			f.Status = ""
		}
		out = append(out, f)
	}
	return out
}

// workingTrigs 给每个触发器定状态；已有的只有语句能改。
func (o *objState) workingTrigs() []database.StructureTriggerEdit {
	out := make([]database.StructureTriggerEdit, 0, len(o.trigs))
	for _, tr := range o.trigs {
		old, existed := o.origTrigOf(tr.UID)
		switch {
		case !existed:
			tr.Status = "new"
		case o.trigDel[tr.UID]:
			tr.Status = "deleted"
		case strings.TrimSpace(tr.Definition) != strings.TrimSpace(old.Definition):
			tr.Status = "modified"
		default:
			tr.Status = ""
		}
		out = append(out, tr)
	}
	return out
}

// edits 是这次提交里外键和触发器的全部编辑。
func (o *objState) edits(table string) database.StructureObjectEdits {
	return database.StructureObjectEdits{FKs: o.workingFKs(table), Triggers: o.workingTrigs()}
}

// changeCount 数有改动的外键和触发器（还没填内容的空白新行不算）。
func (o *objState) changeCount() int {
	n := 0
	for _, f := range o.workingFKs("") {
		if f.Status != "" && !f.Blank() {
			n++
		}
	}
	for _, tr := range o.workingTrigs() {
		if tr.Status != "" && !tr.Blank() {
			n++
		}
	}
	return n
}

// defaultFKName names a foreign key the way most schemas do.
func defaultFKName(table string, cols []string) string {
	return "fk_" + table + "_" + strings.Join(cols, "_")
}

// addFK 在外键表末尾加一行空白，名字格拿到焦点，就地填写。
func (t *tableTab) addFK() {
	s := &t.st
	s.nextUID++
	uid := "new:" + itoa(s.nextUID)
	s.obj.fks = append(s.obj.fks, database.StructureFKEdit{UID: uid, OnDelete: "NO ACTION", OnUpdate: "NO ACTION"})
	s.sub, s.selFK, s.focusFK = 1, len(s.obj.fks)-1, uid
	s.fkList.ScrollIntoView(len(s.obj.fks) - 1)
}

// addTrigger 在触发器表末尾加一行空白，名字格拿到焦点，语句在右边的编辑器里写。
func (t *tableTab) addTrigger() {
	s := &t.st
	s.nextUID++
	uid := "new:" + itoa(s.nextUID)
	s.obj.trigs = append(s.obj.trigs, database.StructureTriggerEdit{UID: uid, Timing: "BEFORE", Event: "INSERT"})
	s.sub, s.selTrig, s.focusTrig = 2, len(s.obj.trigs)-1, uid
	s.trigList.ScrollIntoView(len(s.obj.trigs) - 1)
}

// removeFK 删除第 i 个外键：已有的标成待删除（再点撤销），新增的直接去掉。
func (t *tableTab) removeFK(i int) {
	s := &t.st
	if i < 0 || i >= len(s.obj.fks) || !canEditFK(t.w.cfg.Type) {
		return
	}
	uid := s.obj.fks[i].UID
	if _, existed := s.obj.origFKOf(uid); existed {
		s.obj.fkDel[uid] = !s.obj.fkDel[uid]
		return
	}
	s.obj.fks = append(s.obj.fks[:i], s.obj.fks[i+1:]...)
	s.selFK = -1
}

// removeTrigger 删除第 i 个触发器，规则同 removeFK。
func (t *tableTab) removeTrigger(i int) {
	s := &t.st
	if i < 0 || i >= len(s.obj.trigs) {
		return
	}
	uid := s.obj.trigs[i].UID
	if _, existed := s.obj.origTrigOf(uid); existed {
		s.obj.trigDel[uid] = !s.obj.trigDel[uid]
		return
	}
	s.obj.trigs = append(s.obj.trigs[:i], s.obj.trigs[i+1:]...)
	s.selTrig = -1
}

// refColumnsOf 返回被引用表的列名；还没读过就在后台读，读完前返回 nil。
func (t *tableTab) refColumnsOf(table string) []string {
	o := &t.st.obj
	if table == "" {
		return nil
	}
	if cols, ok := o.refCols[table]; ok {
		return cols
	}
	if o.refBusy[table] {
		return nil
	}
	if o.refCols == nil {
		o.refCols, o.refBusy = map[string][]string{}, map[string]bool{}
	}
	o.refBusy[table] = true
	w, db := t.w, t.w.db
	var cols []database.ColumnInfo
	w.sh.up.bg(func() { cols, _ = w.sh.app.Databases.GetColumns(w.cfg.ID, db, table) }, func() {
		names := make([]string, 0, len(cols))
		for _, c := range cols {
			names = append(names, c.Name)
		}
		o.refCols[table] = names
		delete(o.refBusy, table)
	})
	return nil
}

// tableNames 是能被外键引用的表（不含视图）。
func (t *tableTab) tableNames() []string {
	var out []string
	for _, tb := range t.w.tables {
		if !isView(tb) {
			out = append(out, tb.Name)
		}
	}
	return out
}

// objUnsupportedTip 是外键页「添加」按钮置灰时的提示。
func objUnsupportedTip(dbType string) string {
	if canEditFK(dbType) {
		return ""
	}
	return locales.StructFKUnsupported()
}
