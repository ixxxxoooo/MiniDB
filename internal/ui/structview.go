package ui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
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

	orig    []database.StructureColumnEdit
	cols    []database.StructureColumnEdit
	deleted map[string]bool
	info    []model.ColumnInfo // read-only facts: primary key, auto increment

	origIdx []database.StructureIndexEdit
	idx     []database.StructureIndexEdit
	idxDel  map[string]bool

	types []string
	table string // 表名，给索引起默认名用
	name  string // 顶部「表名」格里的名字：已有的表改了就是重命名，新建表就是要建的表名
	// 新建表（tablenew.go）：没有原始行，预先给一列 newDefault
	newTable   bool
	newDefault database.StructureColumnEdit
	focusName  bool // 刚打开的新建表，等着让表名格拿到焦点
	shownAt    time.Time
	nextUID    int

	obj objState // foreign keys and triggers

	// 四张网格的滚动 / 选中状态：列、索引、外键、触发器。选中的是行号，-1 表示没有。
	colList, idxList, fkList, trigList ui.ListState
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
			s.orig = append(s.orig, database.StructureColumnEdit{
				UID: c.Name, Name: c.Name, Type: c.Type, Nullable: c.Nullable, DefaultValue: c.DefaultValue, Comment: c.Comment,
				AutoIncrement: c.IsAutoIncrement, OnUpdate: database.ParseOnUpdate(c.Extra),
			})
		}
		s.origIdx = s.origIdx[:0]
		for _, ix := range idx {
			s.origIdx = append(s.origIdx, database.StructureIndexEdit{
				UID: ix.Name, Name: ix.Name, Type: ix.Type, IsUnique: ix.IsUnique, IsPrimary: ix.IsPrimary, Columns: ix.Columns,
			})
		}
		s.reset()
		s.loadObjects(t)
	})
}

func (s *structState) reset() {
	s.cols = append([]database.StructureColumnEdit(nil), s.orig...)
	for i := range s.cols {
		if s.cols[i].DefaultValue != nil {
			v := *s.cols[i].DefaultValue
			s.cols[i].DefaultValue = &v
		}
	}
	s.idx = append([]database.StructureIndexEdit(nil), s.origIdx...)
	for i := range s.idx {
		s.idx[i].Columns = slices.Clone(s.idx[i].Columns)
	}
	s.deleted, s.idxDel = map[string]bool{}, map[string]bool{}
	s.obj.reset()
	s.selCol, s.selIdx, s.selFK, s.selTrig = -1, -1, -1, -1
	s.name = s.table
	if s.newTable {
		s.name, s.cols, s.idx = "", []database.StructureColumnEdit{s.newDefault}, nil
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

func (s *structState) origCol(uid string) (database.StructureColumnEdit, bool) {
	for _, c := range s.orig {
		if c.UID == uid {
			return c, true
		}
	}
	return database.StructureColumnEdit{}, false
}

func sameDefault(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// working returns the columns with their Status derived from the originals.
func (s *structState) working() []database.StructureColumnEdit {
	out := make([]database.StructureColumnEdit, 0, len(s.cols))
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

func (s *structState) workingIndexes() []database.StructureIndexEdit {
	out := make([]database.StructureIndexEdit, 0, len(s.idx))
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
func origIndex(list []database.StructureIndexEdit, uid string) (database.StructureIndexEdit, bool) {
	for _, x := range list {
		if x.UID == uid {
			return x, true
		}
	}
	return database.StructureIndexEdit{}, false
}

func hasUID(list []database.StructureIndexEdit, uid string) bool {
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

// listColW 是网格第 i 列此刻的宽度：用户拖过的，或者起初定的。
func listColW(ls *ui.ListState, cols []ui.TableColumn, i int) float32 {
	col := cols[i]
	if w, ok := ls.Columns.Widths[col.ID]; ok && !col.Fixed {
		return w
	}
	return col.Width
}

// structTable 画结构视图里的一张网格。keys 在表格元素上登记键盘快捷键（比如 ⌫ 删除选中的行）。
//
// 列宽加起来不满一行时，右边另有一列自动补满（filler），tint 给出一行的底色，
// 让斑马纹和新增 / 删除的底色从左到右连成一片，不留空白。
func structTable(c *ui.Context, p tokens, gt *ui.Theme, ls *ui.ListState, label string, cols []ui.TableColumn, n int,
	tint func(row int) ui.Color, cell func(row, col int), keys func(tbl ui.Element)) {
	filled := append(append([]ui.TableColumn(nil), cols...), ui.TableColumn{ID: "fill", Fixed: true})
	real := len(cols)
	body := cell
	cell = func(row, col int) {
		if col < real {
			body(row, col)
			return
		}
		if row < n {
			t := tint(row)
			recordRowTail(ls, cellBox(c, t, 0, func() {}), t)
		}
	}
	cols = filled
	ui.Themed(c, gt).Background(ui.Transparent).FixedLineHeight(14).Grow(1).FillWidth().MinWidth(0).Children(func() {
		tbl := ui.Table(c, ls, cols, n, cell).Grow(1).MinWidth(0).Label(label)
		tbl = listBars(c, tbl, ls)
		paintGridBg(tbl, n, p.surfaceSecondary, c.Theme().Space(8)+1, showZebra, func(pt *ui.Painter, r ui.Rect) {
			paintRowTails(pt, ls, r.X+r.W)
		})
		if keys != nil {
			keys(tbl)
		}
	})
}

// safeKey 给列表行取稳定的键：行号越界时（这一帧里数据刚被删短，列表还在按旧行数取）退回行号本身，不崩溃。
//
// 参数：i 行号；n 当前数据的长度；key 行号有效时取键的函数。
func safeKey(i, n int, key func() any) any {
	if i < 0 || i >= n {
		return i
	}
	return key()
}

// structRowTint 是一行的底色：新增 / 待删除的行用状态色，其余的行按斑马纹隔行铺底。
// 表格没有开单元格选择（要留给拖动排序），库自带的斑马纹用不上，所以由单元格自己铺；
// 选中的行不铺，免得盖住选中的底色。
func structRowTint(p tokens, row int, selected bool, state ui.Color) ui.Color {
	if state != (ui.Color{}) {
		return state
	}
	if showZebra && row%2 == 1 && !selected {
		return p.surfaceSecondary
	}
	return ui.Color{}
}

// structTint 是结构页里行的状态底色，和数据行一致：新增绿、待删除红、改过黄（提交之前一直留着）。
// 没有状态则返回零值。
func structTint(p tokens, isNew, deleted, modified bool) ui.Color {
	switch {
	case deleted:
		return p.rowDelete
	case isNew:
		return p.rowNew
	case modified:
		return p.cellEdit
	}
	return ui.Color{}
}

// colModified 报告已有的列和读出来的原样比有没有被改过（名称、类型、可空、默认值、备注）。
func (s *structState) colModified(c database.StructureColumnEdit) bool {
	o, ok := s.origCol(c.UID)
	return ok && (c.Name != o.Name || c.Type != o.Type || c.Nullable != o.Nullable || c.Comment != o.Comment || !sameDefault(c.DefaultValue, o.DefaultValue))
}

// idxModified 报告已有的索引有没有被改过（名称、列、唯一）；主键不能改。
func (s *structState) idxModified(ix database.StructureIndexEdit) bool {
	o, ok := origIndex(s.origIdx, ix.UID)
	return ok && !ix.IsPrimary && (ix.Name != o.Name || ix.IsUnique != o.IsUnique || !slices.Equal(ix.Columns, o.Columns))
}

// cellInput is a text field that looks like a grid cell until it is used.
func cellInput(c *ui.Context, p tokens, key string, v *string, ro bool) ui.Element {
	in := ui.TextInput(c.Key(key), v).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false).ReadOnly(ro)
	return editInput(in, p.accent)
}

// structBar 是一块网格上方的标题条：标题、个数，右边是添加按钮。
func structBar(c *ui.Context, p tokens, title string, n int, addLabel string, add func(), disabled bool) {
	ui.Row(c.Key("bar-"+title)).Gap(6).AlignItems(ui.Center).Padding(0, 6, 1, 12).Height(sizeTab).FillWidth().Shrink(0).Background(p.toolbar).Children(func() {
		ui.Text(c, title).FontWeight(600).FontSize(font2XS)
		ui.Text(c, "("+itoa(n)+")").FontSize(font2XS).TextColor(p.fgMuted)
		ui.Spacer(c)
		if actBtn(c.Key("add"), icPlus, addLabel).Disabled(disabled).Clicked() {
			add()
		}
	})
	hairline(c, p.border)
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
	return !t.w.cfg.ReadOnly && database.IsMySQLCompatible(t.w.cfg.Type)
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
