package ui

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// structState is the working copy of a table's columns and indexes while the
// user edits its structure. Original rows keep their UID (the original name),
// which is how the backend matches them; new rows get "new:N".
type structState struct {
	loaded  bool
	loading bool
	err     string

	orig    []app.StructureColumnEdit
	cols    []app.StructureColumnEdit
	deleted map[string]bool
	info    []model.ColumnInfo // read-only facts: primary key, auto increment

	origIdx []app.StructureIndexEdit
	idx     []app.StructureIndexEdit
	idxDel  map[string]bool

	types []string
	table string // 表名，给索引起默认名用
	name  string // 顶部「表名」格里的名字：已有的表改了就是重命名，新建表就是要建的表名
	// 新建表（tablenew.go）：没有原始行，预先给一列 newDefault
	newTable   bool
	newDefault app.StructureColumnEdit
	focusName  bool // 刚打开的新建表，等着让表名格拿到焦点
	shownAt    time.Time
	nextUID    int

	obj objState // foreign keys and triggers

	// 四张网格的滚动 / 选中状态：列、索引、外键、触发器。选中的是行号，-1 表示没有。
	colList, idxList, fkList, trigList tailList
	selCol, selIdx, selFK, selTrig     int
	sub                                int      // 下半区当前的标签：0 索引、1 外键、2 触发器
	split                              float32  // 上下两块之间分隔条的位置（上半区占的百分比）
	focusCol, focusIdx                 string   // 刚新增、等着拿到输入焦点的行（UID）
	focusFK, focusTrig                 string   // 同上：外键、触发器
	trigSplit                          float32  // 触发器页左右两块之间分隔条的位置（表格占的百分比）
	ctl                                ui.Theme // 网格里控件（勾选框）用的主题，保留原来的强调色
	preview                            struct {
		open  bool
		stmts []string
		err   string
		busy  bool
	}
}

func (s *structState) load(t *tableTab) {
	w := t.w
	db := w.db
	s.loading, s.err = true, ""
	var info []model.ColumnInfo
	var idx []model.IndexInfo
	var types []string
	var err error
	w.sh.up.bg(func() {
		info, err = w.sh.app.Databases.GetColumns(w.cfg.ID, db, t.table)
		if err == nil {
			idx, err = w.sh.app.Databases.GetIndexes(w.cfg.ID, db, t.table)
		}
		types = w.sh.app.Databases.GetColumnTypes(w.cfg.ID)
	}, func() {
		s.loading = false
		if err != nil {
			s.err = errMessage(err)
			return
		}
		s.loaded, s.info, s.types, s.table, s.name = true, info, types, t.table, t.table
		if s.split == 0 {
			s.split = 62
		}
		if s.trigSplit == 0 {
			s.trigSplit = 58
		}
		s.orig = s.orig[:0]
		for _, c := range info {
			s.orig = append(s.orig, app.StructureColumnEdit{
				UID: c.Name, Name: c.Name, Type: c.Type, Nullable: c.Nullable, DefaultValue: c.DefaultValue, Comment: c.Comment,
				AutoIncrement: c.IsAutoIncrement, OnUpdate: app.ParseOnUpdate(c.Extra),
			})
		}
		s.origIdx = s.origIdx[:0]
		for _, ix := range idx {
			s.origIdx = append(s.origIdx, app.StructureIndexEdit{
				UID: ix.Name, Name: ix.Name, Type: ix.Type, IsUnique: ix.IsUnique, IsPrimary: ix.IsPrimary, Columns: ix.Columns,
			})
		}
		s.reset()
		s.loadObjects(t)
	})
}

func (s *structState) reset() {
	s.cols = append([]app.StructureColumnEdit(nil), s.orig...)
	for i := range s.cols {
		if s.cols[i].DefaultValue != nil {
			v := *s.cols[i].DefaultValue
			s.cols[i].DefaultValue = &v
		}
	}
	s.idx = append([]app.StructureIndexEdit(nil), s.origIdx...)
	for i := range s.idx {
		s.idx[i].Columns = slices.Clone(s.idx[i].Columns)
	}
	s.deleted, s.idxDel = map[string]bool{}, map[string]bool{}
	s.obj.reset()
	s.selCol, s.selIdx, s.selFK, s.selTrig = -1, -1, -1, -1
	s.name = s.table
	if s.newTable {
		s.name, s.cols, s.idx = "", []app.StructureColumnEdit{s.newDefault}, nil
	}
}

func (s *structState) isPK(name string) bool {
	for _, c := range s.info {
		if c.Name == name && c.IsPrimary {
			return true
		}
	}
	return false
}

func (s *structState) origCol(uid string) (app.StructureColumnEdit, bool) {
	for _, c := range s.orig {
		if c.UID == uid {
			return c, true
		}
	}
	return app.StructureColumnEdit{}, false
}

func sameDefault(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (t *tableTab) structureView(c *ui.Context) {
	th := c.Theme()
	s := &t.st
	if !s.loaded && !s.loading && s.err == "" {
		s.load(t)
	}
	switch {
	case s.err != "":
		ui.Text(c, locales.TableLoadFail(s.err)).TextColor(th.Danger).Padding(16).Selectable()
		return
	case !s.loaded:
		quietLoading(c, baseTokens(c), &s.shownAt)
		return
	}
	// 只读连接，或该库不支持可视化改结构（如 StarRocks）时，结构页只读
	readOnly := t.w.cfg.ReadOnly || (!t.isNew && !app.SupportsVisualAlterTable(t.w.cfg.Type))
	p := baseTokens(c)
	// 网格自己的主题：12px 等宽字、表头用正文色，和数据网格、TablePlus 的结构页一致
	gt := *th
	gt.Spacing, gt.FontSize, gt.Font = gridSpacing, font2XS, mono
	gt.TextMuted = th.Text
	// 选中的行不整行刷成强调色（里面的输入框、勾选框会看不清），只铺一层淡淡的底；
	// 但网格里的勾选框要保持原来的强调色，所以另留一份控件主题
	s.ctl = gt
	soft := baseTokens(c).accent.Alpha(0.16)
	gt.Accent, gt.AccentHover, gt.AccentPressed, gt.InactiveAccent, gt.AccentText = soft, soft, soft, soft, th.Text
	// 上面是列，下面是索引 / 外键 / 触发器，中间的分隔条可以上下拖
	ui.SplitVerticalPercent(c, &s.split, func() {
		ui.Column(c).Fill().Children(func() { t.structColumns(c, p, &gt, readOnly) })
	}, func() {
		ui.Column(c).Fill().Children(func() { t.structLower(c, p, &gt, readOnly) })
	}).Grow(1).FillWidth()
}

// canReorderColumns 报告这个库能不能在已有的表里调整列的顺序：
// MySQL 系用 AFTER / FIRST，其余的库（SQLite、PostgreSQL）改不了列序。
func (t *tableTab) canReorderColumns() bool {
	return !t.w.cfg.ReadOnly && app.SupportsColumnReorder(t.w.cfg.Type)
}

func (t *tableTab) removeColumn(i int) {
	s := &t.st
	if i < 0 || i >= len(s.cols) {
		return
	}
	col := s.cols[i]
	_, existed := s.origCol(col.UID)
	switch {
	case !existed:
		s.cols = append(s.cols[:i], s.cols[i+1:]...)
		s.selCol = -1
	case s.isPK(col.UID):
		t.w.sh.toastWarn(locales.StructPKLocked())
	default:
		s.deleted[col.UID] = !s.deleted[col.UID]
	}
}
