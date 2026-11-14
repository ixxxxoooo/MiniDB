package ui

// @author ixxxxoooo

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/locales"
)

// 新建表：侧栏菜单里的「新建表」开一个只有结构页的标签，表名填在结构页顶部的「表名」格，
// 列、主键、自增、索引都像编辑已有表一样就地填写，⌘S 时一次建出来。建成之后这个标签就变成这张表的普通标签。

// defaultNewColumn 是新建表时预先给的一列：名为 id 的自增主键。
func defaultNewColumn(dbType model.DBType) database.StructureColumnEdit {
	typ := "bigint"
	if dbType == model.SQLite {
		typ = "INTEGER"
	}
	return database.StructureColumnEdit{UID: "new:0", Name: "id", Type: typ, PrimaryKey: true, AutoIncrement: true}
}

// newTable 打开一个新建表的标签。
func (w *Workspace) newTable() {
	t := newTableTab(w, "")
	t.isNew, t.mode = true, 1
	s := &t.st
	s.loaded, s.newTable = true, true
	s.types = w.sh.app.Databases.GetColumnTypes(w.cfg.ID)
	s.newDefault = defaultNewColumn(w.cfg.Type)
	s.split = 62
	s.trigSplit = 58
	s.reset()
	s.focusName = true
	w.tabs = append(w.tabs, t)
	w.active = len(w.tabs) - 1
	t.keep()
}

// untouchedNew 报告新建表的标签是不是还没动过：名字空着，只有预先给的那一列，没有索引。
func (s *structState) untouchedNew() bool {
	return strings.TrimSpace(s.name) == "" && len(s.cols) == 1 && s.cols[0] == s.newDefault && len(s.idx) == 0
}

// newTableStatements 生成新建表会执行的语句。
func (t *tableTab) newTableStatements() ([]string, error) {
	s := &t.st
	return t.w.sh.app.Databases.PreviewCreateTable(t.w.cfg.ID, s.name, s.working(), s.workingIndexes())
}

// commitNewTable 校验后在安全模式的把关下建表。
func (t *tableTab) commitNewTable() {
	stmts, err := t.newTableStatements()
	if err != nil {
		t.w.sh.toastErr(err.Error())
		return
	}
	t.w.sh.safeGate(t.w, stmts, func() { t.applyNewTable(stmts) })
}

// applyNewTable 执行建表语句；成功后这个标签变成这张表的普通标签。
func (t *tableTab) applyNewTable(stmts []string) {
	s := &t.st
	w := t.w
	db, name := w.db, strings.TrimSpace(s.name)
	cols, idx := s.working(), s.workingIndexes()
	s.preview.busy, s.preview.err = true, ""
	started := time.Now()
	var err error
	w.sh.up.bg(func() { err = w.sh.app.Databases.CreateTable(w.cfg.ID, db, name, cols, idx) }, func() {
		s.preview.busy = false
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		w.sh.logCommit(db, stmts, 0, time.Since(started), errText)
		if err != nil {
			s.preview.err = locales.StructApplyFail(err.Error())
			w.sh.toastErr(s.preview.err)
			return
		}
		w.sh.toastOK(locales.StructApplied())
		t.isNew, t.table, t.g.table = false, name, name
		t.st = structState{split: s.split, trigSplit: s.trigSplit}
		t.ddl, t.doc = ddlState{}, docState{}
		t.loadStructure()
		t.st.load(t)
		t.load()
		w.loadTables()
	})
}

// renamePinned 表改名之后，把侧栏里固定的名字和选中的名字也换成新的。
func (w *Workspace) renamePinned(from, to string) {
	if w.side.picked == from {
		w.side.picked = to
	}
	if !w.isPinned(from) {
		return
	}
	for i, n := range w.side.pins {
		if n == from {
			w.side.pins[i] = to
		}
	}
	pins, key := append([]string(nil), w.side.pins...), w.dbKey()
	w.sh.up.bg(func() { _ = w.sh.app.UIState.SavePins(key, pinList{Names: pins}) }, func() {})
}

// structNameRow 是结构页顶部的「表名」一行：已有的表改了名字就是重命名（⌘S 提交），新建表在这里起名。
// 视图和只读连接里只读。
func (t *tableTab) structNameRow(c *ui.Context, p tokens, readOnly bool) {
	s := &t.st
	ro := readOnly || (!t.isNew && t.w.isViewName(t.table))
	ui.Row(c.Key("nameRow")).Gap(8).Padding(0, 12).AlignItems(ui.Center).Height(sizeBtn).FillWidth().Shrink(0).Background(p.surface).Children(func() {
		ui.Text(c, locales.StructTableName()).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary).SingleLine()
		in := ui.TextInput(c.Key("tname"), &s.name).Width(260).Height(sizeBtnSm-2).Padding(0, 8).Font(mono).FontSize(font2XS).ReadOnly(ro)
		if t.isNew {
			in.Placeholder(locales.StructTableNamePh())
		}
		if s.focusName {
			s.focusName = false
			in.Focus()
		}
	})
	hairline(c, p.borderSubtle)
}
