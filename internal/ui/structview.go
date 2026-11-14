package ui

import (
	"slices"
	"strconv"
	"strings"
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

// working returns the columns with their Status derived from the originals.
func (s *structState) working() []app.StructureColumnEdit {
	out := make([]app.StructureColumnEdit, 0, len(s.cols))
	for _, c := range s.cols {
		o, existed := s.origCol(c.UID)
		switch {
		case !existed:
			c.Status = "new"
		case s.deleted[c.UID]:
			c.Status = "deleted"
		case c.Name != o.Name || c.Type != o.Type || c.Nullable != o.Nullable || c.Comment != o.Comment || !sameDefault(c.DefaultValue, o.DefaultValue):
			c.Status = "modified"
		default:
			c.Status = ""
		}
		out = append(out, c)
	}
	return out
}

func (s *structState) workingIndexes() []app.StructureIndexEdit {
	out := make([]app.StructureIndexEdit, 0, len(s.idx))
	for _, ix := range s.idx {
		switch {
		case !hasUID(s.origIdx, ix.UID):
			ix.Status = "new"
			if strings.TrimSpace(ix.Name) == "" && len(ix.Columns) > 0 {
				ix.Name = defaultIndexName(s.table, ix.Columns, ix.IsUnique)
			}
		case s.idxDel[ix.UID]:
			ix.Status = "deleted"
		default:
			ix.Status = ""
			if o, ok := origIndex(s.origIdx, ix.UID); ok && !ix.IsPrimary &&
				(ix.Name != o.Name || ix.IsUnique != o.IsUnique || !slices.Equal(ix.Columns, o.Columns)) {
				ix.Status = "modified" // 已有的索引被改了：提交时先删旧的再按新的建
			}
		}
		out = append(out, ix)
	}
	return out
}

// origIndex 按原名找到读出来的那个索引。
func origIndex(list []app.StructureIndexEdit, uid string) (app.StructureIndexEdit, bool) {
	for _, x := range list {
		if x.UID == uid {
			return x, true
		}
	}
	return app.StructureIndexEdit{}, false
}

func hasUID(list []app.StructureIndexEdit, uid string) bool {
	for _, x := range list {
		if x.UID == uid {
			return true
		}
	}
	return false
}

// orderChanged 报告已有的列（不含新增的和待删除的）相对原来的顺序是不是变了。
func (s *structState) orderChanged() bool {
	var uids []string
	for _, c := range s.cols {
		if _, existed := s.origCol(c.UID); existed && !s.deleted[c.UID] {
			uids = append(uids, c.UID)
		}
	}
	i := 0
	for _, o := range s.orig {
		if s.deleted[o.UID] {
			continue
		}
		if i >= len(uids) || uids[i] != o.UID {
			return true
		}
		i++
	}
	return false
}

// changeCount counts edited columns, indexes, foreign keys and triggers, and 1 more for a new column order.
func (s *structState) changeCount() int {
	if s.newTable {
		if s.untouchedNew() {
			return 0
		}
	}
	n := s.obj.changeCount()
	if s.renameTo() != "" {
		n++
	}
	if s.orderChanged() {
		n++
	}
	for _, c := range s.working() {
		if c.Status != "" {
			n++
		}
	}
	for _, ix := range s.workingIndexes() {
		if ix.Status == "new" && len(ix.Columns) == 0 {
			continue // 刚点了「添加索引」、还没选列的空行，不算修改
		}
		if ix.Status != "" {
			n++
		}
	}
	return n
}

// renameTo 是已有的表要改成的新名字；没改（或新建表）时为空。
func (s *structState) renameTo() string {
	n := strings.TrimSpace(s.name)
	if s.newTable || n == "" || n == s.table {
		return ""
	}
	return n
}

// defaultIndexName 给没起名字的新索引取个名：idx_表_列（唯一索引用 uniq_ 开头）。
func defaultIndexName(table string, cols []string, unique bool) string {
	prefix := "idx_"
	if unique {
		prefix = "uniq_"
	}
	return prefix + table + "_" + strings.Join(cols, "_")
}

// moveItems 把 rows（升序的原下标）指向的元素整体挪到原下标 to 之前，它们之间的顺序不变；
// to 等于 len(list) 表示挪到末尾。
func moveItems[T any](list []T, rows []int, to int) []T {
	moving := map[int]bool{}
	for _, r := range rows {
		if r >= 0 && r < len(list) {
			moving[r] = true
		}
	}
	var picked, rest []T
	at := -1
	for i, v := range list {
		if i == to {
			at = len(rest)
		}
		if moving[i] {
			picked = append(picked, v)
		} else {
			rest = append(rest, v)
		}
	}
	if at < 0 {
		at = len(rest)
	}
	out := make([]T, 0, len(list))
	out = append(out, rest[:at]...)
	out = append(out, picked...)
	return append(out, rest[at:]...)
}

func itoa(n int) string { return strconv.Itoa(n) }

// 结构视图里几张网格共用的外观：和数据网格一样的密排等宽字体、斑马纹，
// 列宽可以拖表头的边缘调整，列的次序可以拖表头挪动。

// colModified 报告已有的列和读出来的原样比有没有被改过（名称、类型、可空、默认值、备注）。
func (s *structState) colModified(c app.StructureColumnEdit) bool {
	o, ok := s.origCol(c.UID)
	return ok && (c.Name != o.Name || c.Type != o.Type || c.Nullable != o.Nullable || c.Comment != o.Comment || !sameDefault(c.DefaultValue, o.DefaultValue))
}

// idxModified 报告已有的索引有没有被改过（名称、列、唯一）；主键不能改。
func (s *structState) idxModified(ix app.StructureIndexEdit) bool {
	o, ok := origIndex(s.origIdx, ix.UID)
	return ok && !ix.IsPrimary && (ix.Name != o.Name || ix.IsUnique != o.IsUnique || !slices.Equal(ix.Columns, o.Columns))
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
	readOnly := t.w.cfg.ReadOnly
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
	return !t.w.cfg.ReadOnly && app.IsMySQLCompatible(t.w.cfg.Type)
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
