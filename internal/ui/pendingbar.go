package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 待提交悬浮条：表里有没提交的修改时，浮在内容区底部正中，一处收口数量、查看 SQL、丢弃、提交。
// 原来这三样分散在标题栏的小图标组和底栏的「丢弃 / 提交」两处，相距最远；没有修改时完全不占位。

// pendingParts 是数据页待提交修改的分项：改过的行数、新增的行数、待删除的行数。
// 新增行里的单元格不算「改」，待删除行里改过的单元格也不算。
func (t *tableTab) pendingParts() (mod, add, del int) {
	rows := map[int]bool{}
	for k := range t.edits.cells {
		if k.row < len(t.es.orig) && !t.edits.deleted[k.row] {
			rows[k.row] = true
		}
	}
	return len(rows), t.es.added, len(t.edits.deleted)
}

// pendingText 是悬浮条上的分项文字：数据页「改 1 · 增 1 · 删 1」，结构页「加列 1 · 改索引 1」，为零的项省略。
func (t *tableTab) pendingText() string {
	if t.mode == 1 {
		return strings.Join(t.structParts(), " · ")
	}
	var parts []string
	mod, add, del := t.pendingParts()
	if mod > 0 {
		parts = append(parts, locales.PendingBarMod(itoa(mod)))
	}
	if add > 0 {
		parts = append(parts, locales.PendingBarAdd(itoa(add)))
	}
	if del > 0 {
		parts = append(parts, locales.PendingBarDel(itoa(del)))
	}
	return strings.Join(parts, " · ")
}

// structParts 是结构页待提交修改的分项文字：加 / 删 / 改列，加 / 删 / 改索引，其余（外键、触发器、改名、列序）合在「其他」里。
func (t *tableTab) structParts() []string {
	s := &t.st
	var ac, dc, mc, ai, di, mi int
	for _, c := range s.working() {
		switch c.Status {
		case "new":
			ac++
		case "deleted":
			dc++
		case "modified":
			mc++
		}
	}
	for _, ix := range s.workingIndexes() {
		if ix.Status == "new" && len(ix.Columns) == 0 {
			continue
		}
		switch ix.Status {
		case "new":
			ai++
		case "deleted":
			di++
		case "modified":
			mi++
		}
	}
	other := s.changeCount() - ac - dc - mc - ai - di - mi
	var parts []string
	for _, it := range []struct {
		n int
		f func(string) string
	}{{ac, locales.StructPendAddCol}, {dc, locales.StructPendDelCol}, {mc, locales.StructPendModCol}, {ai, locales.StructPendAddIdx}, {di, locales.StructPendDelIdx}, {mi, locales.StructPendModIdx}, {other, locales.StructPendOther}} {
		if it.n > 0 {
			parts = append(parts, it.f(itoa(it.n)))
		}
	}
	return parts
}

// pendingFloat 画悬浮条。要放在内容区那一层里（相对它定位），不能放在含底栏的外层。
func (t *tableTab) pendingFloat(c *ui.Context, p tokens) {
	s := t.w.sh
	_, n := s.pendingTable()
	if n == 0 || (t.mode != 0 && t.mode != 1) {
		return
	}
	// 设计稿 A 的「岛」：深色（文字色做底）的胶囊，浅色主题下是黑底白字，深色主题下反过来
	ink, inkSoft := p.surface, p.surface.Alpha(0.65)
	island := func(key string, ic *ui.SVG, label, tip string) ui.Element {
		b := tipOn(c, actBtn(c.Key(key), ic, label), tip).Radius(btnActH/2).Background(ui.Transparent).Border(1, ui.Transparent).TextColor(p.surface.Alpha(0.75))
		if b.Hovered() {
			b.Background(p.surface.Alpha(0.12)).TextColor(ink)
		}
		return b
	}
	ui.Row(c.Key("pendbar-wrap")).Absolute().Left(0).Right(0).Bottom(spaceMD).Justify(ui.Center).Children(func() {
		bar := ui.Row(c.Key("pendbar")).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceXS+2, spaceXS+2, spaceXS+2, spaceMD+2).Shrink(0).
			Radius((btnActH+spaceXS*2+4)/2).Background(p.fg).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Children(func() {
			ui.Box(c).Size(sizeDot+1, sizeDot+1).Radius(sizeDot).Background(p.warning).Shrink(0)
			title, previewLabel, commitLabel := locales.PendingBarTitle(n), locales.TablePreviewSQL(), locales.TableCommit()
			if t.mode == 1 {
				title, previewLabel, commitLabel = locales.PendingBarStructTitle(n), locales.PendingBarPreviewAlter(), locales.PendingBarApply()
			}
			ui.Text(c, title).FontSize(fontDense).FontWeight(600).SingleLine().TextColor(ink)
			if txt := t.pendingText(); txt != "" {
				ui.Text(c, txt).FontSize(fontCaption).TextColor(inkSoft).SingleLine()
			}
			vline(c, p.surface.Alpha(0.2), 14)
			if island("pb-sql", icCode, previewLabel, previewLabel+s.keyHint("preview")).Clicked() {
				s.previewPending()
			}
			if island("pb-discard", nil, locales.TableDiscard(), locales.TableDiscard()+s.keyHint("discard")).Clicked() {
				s.discardPending()
			}
			if tipOn(c, actPrimary(c.Key("pb-commit"), icCheck, commitLabel+" "+s.cmdKeys("commit")), commitLabel+s.keyHint("commit")).Radius(btnActH / 2).Clicked() {
				s.commitPending()
			}
		})
		enterPose(c, bar, poseSlideUp, popAnim) // 设计稿：悬浮条自下 10px 滑入
	})
}
