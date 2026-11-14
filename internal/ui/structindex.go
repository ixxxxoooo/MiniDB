package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

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
	if t.isNew { // 还没建出来的表：外键、触发器要等建好之后再加
		labels = labels[:1]
	}
	s.sub = min(s.sub, len(labels)-1)
	add := func() {
		switch s.sub {
		case 0:
			t.addIndex()
		case 1:
			t.addFK()
		default:
			t.addTrigger()
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
	if i := tabStrip(c, p, "structsub", labels, s.sub, connColor(c, cfgColor(t.w.cfg)), false, func() {
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
	s.idx = append(s.idx, app.StructureIndexEdit{UID: uid})
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
	s.idxList.Key = func(i int) any { return safeKey(i, len(s.idx), func() any { return s.idx[i].UID }) }
	s.idxList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(s.idx) && m.Item(locales.CommonDelete()).Disabled(readOnly || s.idx[row].IsPrimary).Chosen() {
			defer t.removeIndex(row)
		}
	}
	structTable(c, p, gt, &s.idxList, locales.StructIndexes(), cols, len(s.idx), func(row int) ui.Color {
		if row < 0 || row >= len(s.idx) {
			return ui.Color{}
		}
		ix := &s.idx[row]
		return structRowTint(p, row, s.selIdx == row, structTint(p, !hasUID(s.origIdx, ix.UID), s.idxDel[ix.UID], s.idxModified(*ix)))
	}, func(row, ci int) {
		if row >= len(s.idx) {
			return
		}
		ix := &s.idx[row]
		existed := hasUID(s.origIdx, ix.UID)
		deleted := s.idxDel[ix.UID]
		edit := !readOnly && !deleted && !ix.IsPrimary // 新索引和已有的索引都能就地改（主键除外），已有的提交时先删后建
		focus := false
		cell := func(content func()) {
			ringCellBox(c, p, structRowTint(p, row, s.selIdx == row, structTint(p, !existed, deleted, s.idxModified(*ix))), listColW(&s.idxList.ListState, cols, ci), &focus, content)
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
				in := cellInput(c, p, "iname", &ix.Name, false).PlaceholderColor(p.fgMuted)
				if !existed {
					in.Placeholder(defaultIndexName(s.table, ix.Columns, ix.IsUnique))
				}
				if s.focusIdx == ix.UID {
					s.focusIdx = ""
					in.Focus()
				}
				focus = in.Focused()
			})
		case 2:
			cell(func() {
				if !edit {
					ui.Text(c, strings.Join(ix.Columns, ", ")).SingleLine()
					return
				}
				tf := ui.TokenField(c.Key("icols"), &ix.Columns, names).FillWidth().Height(gridCellH).Padding(0).BorderWidth(0).Background(ui.Transparent).FocusRing(false)
				focus = editInput(tf, p.accent).Focused()
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
