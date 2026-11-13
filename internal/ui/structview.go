package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
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
	info    []database.ColumnInfo // read-only facts: primary key, auto increment

	origIdx []database.StructureIndexEdit
	idx     []database.StructureIndexEdit
	idxDel  map[string]bool

	types   []string
	table   string // 表名，给索引起默认名用
	shownAt time.Time
	nextUID int

	obj objState // foreign keys and triggers

	// 四张网格的滚动 / 选中状态：列、索引、外键、触发器。选中的是行号，-1 表示没有。
	colList, idxList, fkList, trigList ui.ListState
	selCol, selIdx, selFK, selTrig     int
	sub                                int      // 下半区当前的标签：0 索引、1 外键、2 触发器
	split                              float32  // 上下两块之间分隔条的位置（上半区占的百分比）
	focusCol, focusIdx                 string   // 刚新增、等着拿到输入焦点的行（UID）
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
	var info []database.ColumnInfo
	var idx []database.IndexInfo
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
			s.err = err.Error()
			return
		}
		s.loaded, s.info, s.types, s.table = true, info, types, t.table
		if s.split == 0 {
			s.split = 62
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
	s.deleted, s.idxDel = map[string]bool{}, map[string]bool{}
	s.selCol, s.selIdx, s.selFK, s.selTrig = -1, -1, -1, -1
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
		}
		out = append(out, ix)
	}
	return out
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

// changeCount counts edited columns and indexes, and 1 more for a new column order.
func (s *structState) changeCount() int {
	n := 0
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
func structTable(c *ui.Context, p tokens, gt *ui.Theme, ls *ui.ListState, label string, cols []ui.TableColumn, n int,
	cell func(row, col int), keys func(tbl ui.Element)) {
	ui.Themed(c, gt).Background(ui.Transparent).FixedLineHeight(14).Grow(1).FillWidth().MinWidth(0).Children(func() {
		tbl := ui.Table(c, ls, cols, n, cell).Grow(1).MinWidth(0).Label(label)
		tbl = listBars(c, tbl, ls)
		paintGridBg(tbl, n, p.surfaceSecondary, c.Theme().Space(8)+1, showZebra)
		if keys != nil {
			keys(tbl)
		}
	})
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

// structTint 是新增行（绿）和待删除行（红）的底色，没有则返回零值。
func structTint(p tokens, isNew, deleted bool) ui.Color {
	switch {
	case deleted:
		return p.danger.Alpha(0.12)
	case isNew:
		return p.success.Alpha(0.14)
	}
	return ui.Color{}
}

// cellInput is a text field that looks like a grid cell until it is used.
func cellInput(c *ui.Context, key string, v *string, ro bool) ui.Element {
	return ui.TextInput(c.Key(key), v).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false).ReadOnly(ro)
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
	t.structDialogs(c)
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

// ---- 列 ----

func (t *tableTab) structColumns(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	canMove := t.canReorderColumns()
	structBar(c, p, locales.StructColumns(), len(s.cols), locales.StructAddColumn(), s.addColumn, readOnly)

	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: locales.StructName(), Width: 200, MinWidth: 90},
		{ID: "type", Title: locales.StructType(), Width: 170, MinWidth: 90},
		{ID: "null", Title: locales.StructNullable(), Width: 70, MinWidth: 56},
		{ID: "def", Title: locales.StructDefault(), Width: 170, MinWidth: 70},
		{ID: "comment", Title: locales.StructComment(), Width: 360, MinWidth: 80},
		{ID: "x", Width: 34, Fixed: true},
	}
	s.colList.Selected = &s.selCol
	s.colList.Key = func(i int) any { return s.cols[i].UID }
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
	structTable(c, p, gt, &s.colList, locales.StructColumns(), cols, len(s.cols), func(row, ci int) {
		if row >= len(s.cols) {
			return
		}
		col := &s.cols[row]
		_, existed := s.origCol(col.UID)
		deleted := s.deleted[col.UID]
		ro := readOnly || deleted
		cell := func(content func()) {
			cellBox(c, structRowTint(p, row, s.selCol == row, structTint(p, !existed, deleted)), listColW(&s.colList, cols, ci), content)
		}
		switch ci {
		case 0:
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
		case 1:
			cell(func() {
				in := cellInput(c, "n", &col.Name, ro)
				if s.focusCol == col.UID {
					s.focusCol = ""
					in.Focus()
				}
			})
		case 2:
			cell(func() {
				ui.Combobox(c.Key("t"), &col.Type, s.types).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).Disabled(ro)
			})
		case 3:
			cell(func() {
				ui.Themed(c, &s.ctl).Children(func() { ui.Checkbox(c.Key("nl"), &col.Nullable, "").Disabled(ro) })
			})
		case 4:
			cell(func() {
				def := ""
				if col.DefaultValue != nil {
					def = *col.DefaultValue
				}
				in := cellInput(c, "d", &def, ro).Placeholder("NULL")
				if in.Changed() {
					if def == "" {
						col.DefaultValue = nil
					} else {
						col.DefaultValue = &def
					}
				}
			})
		case 5:
			cell(func() { cellInput(c, "c", &col.Comment, ro) })
		case 6:
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

// ---- 索引 / 外键 / 触发器 ----

// structLower 是下半区：一排和主标签栏同款的标签切换索引、外键、触发器，
// 标签栏右端是当前这一项的添加按钮。
func (t *tableTab) structLower(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	dbType := t.w.cfg.Type
	canFK, trig := canEditFK(dbType), canTriggers(dbType)
	labels := []string{
		locales.StructIndexes() + " (" + itoa(len(s.idx)) + ")",
		locales.StructForeignKeys() + " (" + itoa(len(s.obj.fks)) + ")",
	}
	if trig {
		labels = append(labels, locales.StructTriggers()+" ("+itoa(len(s.obj.trigs))+")")
	}
	s.sub = min(s.sub, len(labels)-1)
	add := func() {
		switch s.sub {
		case 0:
			t.addIndex()
		case 1:
			s.obj.newFK = fkForm{open: true, onDelete: "NO ACTION", onUpdate: "NO ACTION"}
		default:
			s.obj.newTrig = trigForm{open: true, timing: "BEFORE", event: "INSERT"}
		}
	}
	addLabel, tip, off := locales.StructAddIndex(), "", readOnly
	switch s.sub {
	case 1:
		addLabel, off = locales.StructAddForeignKey(), readOnly || !canFK
		if !canFK {
			tip = locales.StructFKUnsupported()
		}
	case 2:
		addLabel = locales.StructAddTrigger()
	}
	if i := tabStrip(c, p, "structsub", labels, s.sub, connColor(c, t.w.cfg.Color), false, func() {
		b := actBtn(c.Key("sub-add"), icPlus, addLabel).Disabled(off)
		if tip != "" {
			tipOn(c, b, tip)
		}
		if b.Clicked() {
			add()
		}
	}); i >= 0 {
		s.sub = i
	}
	switch s.sub {
	case 0:
		t.structIndexes(c, p, gt, readOnly)
	case 1:
		t.structFKs(c, p, gt, readOnly, canFK)
	default:
		t.structTriggers(c, p, gt, readOnly)
	}
}

// addIndex 在索引表末尾加一行空白，名字格直接拿到输入焦点，就地填写——
// 和 TablePlus 一样，不弹对话框。
func (t *tableTab) addIndex() {
	s := &t.st
	s.nextUID++
	uid := "new:" + itoa(s.nextUID)
	s.idx = append(s.idx, database.StructureIndexEdit{UID: uid})
	s.sub, s.selIdx, s.focusIdx = 0, len(s.idx)-1, uid
	s.idxList.ScrollIntoView(len(s.idx) - 1)
}

func (t *tableTab) removeIndex(i int) {
	s := &t.st
	if i < 0 || i >= len(s.idx) {
		return
	}
	ix := s.idx[i]
	switch {
	case ix.IsPrimary:
		t.w.sh.toastWarn(locales.StructPKLocked())
	case hasUID(s.origIdx, ix.UID):
		s.idxDel[ix.UID] = !s.idxDel[ix.UID]
	default:
		s.idx = append(s.idx[:i], s.idx[i+1:]...)
		s.selIdx = -1
	}
}

func (t *tableTab) structIndexes(c *ui.Context, p tokens, gt *ui.Theme, readOnly bool) {
	s := &t.st
	var names []string
	for _, col := range s.cols {
		names = append(names, col.Name)
	}
	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: locales.StructIndexName(), Width: 240, MinWidth: 90},
		{ID: "cols", Title: locales.StructIndexColumns(), Width: 320, MinWidth: 120},
		{ID: "type", Title: locales.StructType(), Width: 110, MinWidth: 60},
		{ID: "unique", Title: locales.StructUnique(), Width: 80, MinWidth: 56},
		{ID: "x", Width: 34, Fixed: true},
	}
	s.idxList.Selected = &s.selIdx
	s.idxList.Key = func(i int) any { return s.idx[i].UID }
	s.idxList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(s.idx) && m.Item(locales.CommonDelete()).Disabled(readOnly || s.idx[row].IsPrimary).Chosen() {
			defer t.removeIndex(row)
		}
	}
	structTable(c, p, gt, &s.idxList, locales.StructIndexes(), cols, len(s.idx), func(row, ci int) {
		if row >= len(s.idx) {
			return
		}
		ix := &s.idx[row]
		existed := hasUID(s.origIdx, ix.UID)
		deleted := s.idxDel[ix.UID]
		edit := !existed && !readOnly // 只有还没提交的新索引能就地编辑，已有的索引要改请删了重建
		cell := func(content func()) {
			cellBox(c, structRowTint(p, row, s.selIdx == row, structTint(p, !existed, deleted)), listColW(&s.idxList, cols, ci), content)
		}
		switch ci {
		case 0:
			cell(func() {
				if ix.IsPrimary {
					ui.Icon(c, icKey).FontSize(11).TextColor(p.warning)
				} else {
					ui.Text(c, itoa(row+1)).TextColor(p.fgMuted).FontSize(font2XS)
				}
			})
		case 1:
			cell(func() {
				if !edit {
					ui.Text(c, ix.Name).SingleLine()
					return
				}
				in := cellInput(c, "iname", &ix.Name, false).Placeholder(defaultIndexName(s.table, ix.Columns, ix.IsUnique)).PlaceholderColor(p.fgMuted)
				if s.focusIdx == ix.UID {
					s.focusIdx = ""
					in.Focus()
				}
			})
		case 2:
			cell(func() {
				if !edit {
					ui.Text(c, strings.Join(ix.Columns, ", ")).SingleLine()
					return
				}
				tf := ui.TokenField(c.Key("icols"), &ix.Columns, names).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent)
				if len(ix.Columns) == 0 {
					tf.Placeholder(locales.StructPickColumns())
				}
			})
		case 3:
			cell(func() { ui.Text(c, ix.Type).SingleLine().TextColor(p.fgSecondary) })
		case 4:
			cell(func() {
				if edit {
					ui.Themed(c, &s.ctl).Children(func() { ui.Checkbox(c.Key("iuniq"), &ix.IsUnique, "") })
					return
				}
				txt := "NO"
				if ix.IsUnique || ix.IsPrimary {
					txt = "YES"
				}
				ui.Text(c, txt).TextColor(p.fgSecondary)
			})
		case 5:
			cell(func() {
				if s.selIdx != row || readOnly || ix.IsPrimary {
					return
				}
				ic, tip := icTrash, locales.CommonDelete()
				if deleted {
					ic, tip = icUndo, locales.StructDiscard()
				}
				if tipBtn(c.Key("idel"), ic, tip).Size(20, 20).Clicked() {
					defer t.removeIndex(row)
				}
			})
		}
	}, func(tbl ui.Element) {
		if !readOnly && (tbl.Shortcut(0, ui.KeyBackspace) || tbl.Shortcut(0, ui.KeyDelete)) {
			t.removeIndex(s.selIdx)
		}
	})
}

func (t *tableTab) structDialogs(c *ui.Context) {
	t.objectDialogs(c)
}

// buildStructurePreview fills in the DDL a commit would run (for tests and logs).
func (t *tableTab) buildStructurePreview() {
	s := &t.st
	var stmts []string
	var err error
	w := t.w
	w.sh.up.bg(func() {
		stmts, err = w.sh.app.Databases.PreviewTableStructureChanges(w.cfg.ID, t.table, s.working(), s.orig, s.workingIndexes())
	}, func() {
		s.preview.stmts, s.preview.err = stmts, ""
		if err != nil {
			s.preview.err = err.Error()
		}
	})
}

func (t *tableTab) applyStructure() {
	s := &t.st
	w := t.w
	db := w.db
	s.preview.busy, s.preview.err = true, ""
	working, idx := s.working(), s.workingIndexes()
	stmts := append([]string(nil), s.preview.stmts...)
	started := time.Now()
	var err error
	w.sh.up.bg(func() {
		err = w.sh.app.Databases.ApplyTableStructureChanges(w.cfg.ID, db, t.table, working, s.orig, idx)
	}, func() {
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
		t.ddl = ddlState{}
		t.colSigReset()
		s.load(t)
		t.loadStructure()
		t.load()
		w.loadTables()
	})
}

// commitStructure applies the structure changes at once, without a dialog;
// the connection's Safe Mode may ask first.
func (t *tableTab) commitStructure() {
	s := &t.st
	if s.preview.busy || s.changeCount() == 0 || t.w.cfg.ReadOnly {
		return
	}
	stmts := s.preview.stmts
	if len(stmts) == 0 {
		stmts = []string{"ALTER TABLE " + t.table}
	}
	t.w.sh.safeGate(t.w, stmts, t.applyStructure)
}

// colSigReset forces the data grid to rebuild its columns after a structure change.
func (t *tableTab) colSigReset() { t.g.sig = "" }
