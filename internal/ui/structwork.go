package ui

import (
	"slices"
	"strconv"
	"strings"

	"minidb/internal/app"
)

// 结构编辑的工作态推导：由原始行与当前行算出每一行的状态（新增 / 删除 / 修改）、变更数和列序变化。

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
