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

// pendingText 是悬浮条上的分项文字（「改 1 · 增 1 · 删 1」，为零的项省略）；结构页没有分项，返回空串。
func (t *tableTab) pendingText() string {
	if t.mode != 0 {
		return ""
	}
	mod, add, del := t.pendingParts()
	var parts []string
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

// pendingFloat 画悬浮条。要放在内容区那一层里（相对它定位），不能放在含底栏的外层。
func (t *tableTab) pendingFloat(c *ui.Context, p tokens) {
	s := t.w.sh
	_, n := s.pendingTable()
	if n == 0 || (t.mode != 0 && t.mode != 1) {
		return
	}
	ui.Row(c.Key("pendbar-wrap")).Absolute().Left(0).Right(0).Bottom(spaceMD).Justify(ui.Center).Children(func() {
		ui.Row(c.Key("pendbar")).Gap(spaceSM).AlignItems(ui.Center).Padding(6, spaceSM, 6, spaceMD+2).Shrink(0).
			Radius(radiusPanel).Background(p.surfaceElev).Border(1, p.border).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Children(func() {
			ui.Box(c).Size(sizeDot+1, sizeDot+1).Radius(sizeDot).Background(p.warning).Shrink(0)
			ui.Text(c, locales.PendingBarTitle(n)).FontSize(fontDense).FontWeight(600).SingleLine()
			if txt := t.pendingText(); txt != "" {
				ui.Text(c, txt).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			}
			vline(c, p.border, 14)
			if tipOn(c, ghostBtn(c.Key("pb-sql"), icCode, locales.TablePreviewSQL()), locales.TablePreviewSQL()+s.keyHint("preview")).Clicked() {
				s.previewPending()
			}
			if tipOn(c, actBtn(c.Key("pb-discard"), nil, locales.TableDiscard()), locales.TableDiscard()+s.keyHint("discard")).Clicked() {
				s.discardPending()
			}
			if tipOn(c, actPrimary(c.Key("pb-commit"), icCheck, locales.TableCommit()+" "+s.cmdKeys("commit")), locales.TableCommit()+s.keyHint("commit")).Clicked() {
				s.commitPending()
			}
		})
	})
}
