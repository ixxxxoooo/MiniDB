package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 命令面板（设计稿 10）的各个部件：分类芯片、分组标题、结果行（命中的部分加粗）、底部的按键提示，
// 以及最后一行「问 AI」。条目的收集和匹配在 palette.go。

// paletteKinds 是分类筛选的顺序：下标 0 是全部，其余对应条目的 kind。
func paletteKinds() []string {
	return []string{"", locales.PaletteTable(), locales.PaletteCommand(), locales.PaletteHistory(), locales.PaletteConnection()}
}

// paletteAIKind 标记最后一行「问 AI」，它不属于任何分类，也没有分组标题。
const paletteAIKind = "\x00ai"

// paletteAskAI 在条目末尾补上「问 AI：“查询词”」：有工作区、查询词不为空、也不是「>」命令模式时才有；选中后把查询词交给 AI 助手。
func (s *Shell) paletteAskAI(items []paletteItem) []paletteItem {
	w := s.activeWorkspace()
	q := strings.TrimSpace(s.pal.query)
	if w == nil || q == "" || strings.HasPrefix(q, ">") {
		return items
	}
	return append(items, paletteItem{kind: paletteAIKind, label: locales.PaletteAskAI() + "“" + q + "”", icon: icSparkle, run: func() {
		w.panel = 2
		w.ai.send(q)
	}})
}

// paletteGroup 按分类筛选条目，并让同一类挨在一起（表、命令、历史、连接），组内保持原来的顺序。
func paletteGroup(items []paletteItem, cat int) []paletteItem {
	kinds := paletteKinds()
	if cat > 0 && cat < len(kinds) {
		items = slices.DeleteFunc(items, func(it paletteItem) bool { return it.kind != kinds[cat] })
	}
	rank := func(k string) int {
		if i := slices.Index(kinds, k); i > 0 {
			return i
		}
		return len(kinds)
	}
	slices.SortStableFunc(items, func(a, b paletteItem) int { return rank(a.kind) - rank(b.kind) })
	return items
}

// paletteChips 画分类芯片：全部 / 表 / 命令 / 历史 / 连接，点一枚切换筛选。
func (s *Shell) paletteChips(c *ui.Context, p tokens) {
	ui.Row(c.Key("pal-chips")).Gap(spaceXS).Padding(2, spaceSM, spaceSM, spaceSM).FillWidth().Shrink(0).Children(func() {
		labels := append([]string{locales.PaletteAll()}, paletteKinds()[1:]...)
		for i, l := range labels {
			bg, fg := p.surfaceTertiary, p.fgSecondary
			if i == s.pal.cat {
				bg, fg = p.accentSoft2, p.accentText
			}
			b := ui.Button(c.Key("pal-cat"+itoa(i)), "").Label(l).Height(sizeChip).Padding(0, spaceSM+2).Radius(sizeChip / 2).Background(bg).BorderWidth(0).AlignItems(ui.Center)
			if b.Hovered() && i != s.pal.cat {
				b.Background(p.hover)
			}
			b.Children(func() { ui.Text(c, l).FontSize(fontCaption + 0.5).FontWeight(500).TextColor(fg).SingleLine() })
			if b.Clicked() {
				s.pal.cat, s.pal.sel = i, 0
			}
		}
	})
}

// paletteList 画结果：同一类前有一个小标题，每行是图标、名称（命中处加粗）、说明、快捷键；
// 有查询词且有工作区时，最后一行是「问 AI」。choose 在点击某一行后执行它。
func (s *Shell) paletteList(c *ui.Context, p tokens, items []paletteItem, choose func()) {
	pal := &s.pal
	if len(items) == 0 {
		ui.Text(c, locales.PaletteNone()).TextColor(p.fgMuted).Padding(14).TextAlign(ui.Center).FillWidth()
	}
	query := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pal.query), ">"))
	scrollbars(c, ui.Scroll(c.Key("palette")).MaxHeight(360).FillWidth()).Children(func() {
		ui.Column(c).Gap(1).FillWidth().Children(func() {
			last := ""
			for i, it := range items {
				i, it := i, it
				if it.kind == paletteAIKind {
					hairline(c, p.borderSubtle)
				} else if pal.cat == 0 && it.kind != last {
					last = it.kind
					ui.Text(c, it.kind).FontSize(fontCaption).FontWeight(600).TextColor(p.fgMuted).Padding(spaceSM-2, spaceSM, 2, spaceSM)
				}
				row := ui.Row(c.Key(it.kind+"/"+it.label+"/"+it.hint)).Gap(spaceSM+2).Padding(0, spaceSM+2).Height(30).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
				on := i == pal.sel
				switch {
				case on:
					row.Background(p.accent)
				case row.Hovered():
					row.Background(p.hover)
				}
				row.Children(func() { s.paletteRow(c, p, it, query, on) })
				if row.Clicked() {
					pal.sel = i
					choose()
				}
			}
		})
	})
}

// paletteRow 画一行的内容：图标 + 名称（命中的部分加粗）+ 说明 + 快捷键或「打开 ⏎」。
func (s *Shell) paletteRow(c *ui.Context, p tokens, it paletteItem, query string, on bool) {
	switch {
	case it.dbType != "":
		dbTile(c, it.dbType, 20, ui.Hex(dbColors[it.dbType]))
	case it.icon != nil:
		ic := paletteIconColor(p, it.icon)
		if on {
			ic = p.accentFg
		}
		ui.Icon(c, it.icon).FontSize(15).TextColor(ic).Width(20)
	default:
		ui.Text(c, it.kind).FontSize(fontCaption).TextColor(p.fgMuted).Width(20)
	}
	fg, sub := p.fg, p.fgMuted
	switch {
	case on:
		fg, sub = p.accentFg, p.accentFg.Alpha(0.8)
	case it.kind == paletteAIKind:
		fg = p.accentText
	}
	ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Children(func() {
		pre, hit, post := splitMatch(it.label, query)
		if hit == "" || it.kind == paletteAIKind {
			ui.Text(c, it.label).TextColor(fg).SingleLine()
			return
		}
		ui.Text(c, pre).TextColor(fg).SingleLine().Shrink(0)
		ui.Text(c, hit).FontWeight(700).TextColor(fg).SingleLine().Shrink(0)
		ui.Text(c, post).TextColor(fg).SingleLine()
	})
	if it.hint != "" {
		ui.Text(c, it.hint).FontSize(fontCaption).TextColor(sub).SingleLine().MaxWidth(220).TruncateMiddle()
	}
	switch {
	case it.keys != "":
		kbd(c, it.keys)
	case on:
		ui.Text(c, locales.PaletteOpenEnter()).FontSize(fontCaption).TextColor(p.accentFg).SingleLine().Shrink(0)
	}
}

// splitMatch 把 label 拆成「命中前 / 命中 / 命中后」三段（不区分大小写，只认查询词本身，^ $ 不算）；没命中 hit 为空。
func splitMatch(label, query string) (pre, hit, post string) {
	q := strings.ToLower(strings.Trim(query, "^$ "))
	if q == "" {
		return label, "", ""
	}
	i := strings.Index(strings.ToLower(label), q)
	if i < 0 || i+len(q) > len(label) {
		return label, "", ""
	}
	return label[:i], label[i : i+len(q)], label[i+len(q):]
}

// paletteFooter 是底部一行按键提示。
func (s *Shell) paletteFooter(c *ui.Context, p tokens) {
	ui.Row(c.Key("pal-foot")).Gap(spaceMD).Padding(spaceSM, spaceSM+2, 0, spaceSM+2).FillWidth().AlignItems(ui.Center).Shrink(0).Children(func() {
		hint := func(keys []string, text string) {
			ui.Row(c).Gap(spaceXS).AlignItems(ui.Center).Children(func() {
				for _, k := range keys {
					kbd(c, k)
				}
				ui.Text(c, text).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			})
		}
		hint([]string{"↑", "↓"}, locales.PaletteFootSelect())
		hint([]string{"⏎"}, locales.PaletteFootOpen())
		hint([]string{"⌘⏎"}, locales.PaletteFootNewTab())
		ui.Spacer(c)
		ui.Text(c, locales.PaletteFootCmds()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	})
}

// tableHint 是表行右边的说明：库名，估算行数有的话接在后面（「shop · 12.4k 行」）。
func tableHint(db string, t model.TableInfo) string {
	if t.RowCount <= 0 {
		return db
	}
	return db + " · " + locales.PaletteRows(shortCount(t.RowCount))
}
