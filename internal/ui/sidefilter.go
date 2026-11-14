package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// histOpts 是侧栏「历史 / 查询」搜索的选项：正则、区分大小写、整词。
type histOpts struct {
	regex, cs, word bool
	bad             bool // 当前的正则不合法
}

// search 把搜索词和选项合成一次历史搜索；favs 为真时只看收藏。每页最多取 300 条，但先过滤再截断。
func (o histOpts) search(text string, favs bool) app.HistorySearch {
	return app.HistorySearch{Text: text, Regex: o.regex, Case: o.cs, Word: o.word, Favorites: favs, Limit: 300}
}

// textToggle 是搜索框旁的小开关，用一两个字符当图标（Aa、.*、ab）。
func textToggle(c *ui.Context, glyph, tip string, on bool) ui.Element {
	p := baseTokens(c)
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtnSm, sizeBtnSm).Padding(0).BorderWidth(0).Background(ui.Color{})
	col := p.fgSecondary
	switch {
	case on:
		col = p.accent
		b.Background(p.accent.Alpha(0.10))
	case b.Hovered():
		col = p.fg
	}
	return b.Children(func() { ui.Text(c, glyph).Font(mono).FontSize(font2XS).FontWeight(600).TextColor(col) })
}

// sideFilter 画侧栏的筛选框。默认不显示：只有已有筛选词、或用户主动呼出
// （⌘⇧F 或底部的放大镜）时才出现；在框里按 Esc 清空并收起。
// 在「历史 / 查询」页，框右边有三个开关：区分大小写、整词、正则。
func (w *Workspace) sideFilter(c *ui.Context) {
	if w.side.focusFilter {
		w.side.showFilter = true
	}
	if !w.side.showFilter && w.filter == "" {
		return
	}
	var filter ui.Element
	changed := false
	ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(0, 8, 4, 8).FillWidth().Shrink(0).Children(func() {
		filter = ui.SearchField(c, &w.filter).Placeholder(locales.SidebarFilterPlaceholder()).FontSize(font2XS).Height(sizeBtn).Grow(1).MinWidth(0)
		if w.side.seg == sideItems {
			return
		}
		o := &w.side.hist
		for _, t := range []struct {
			glyph, tip string
			on         *bool
		}{{"Aa", locales.HistoryMatchCase(), &o.cs}, {"ab", locales.HistoryMatchWord(), &o.word}, {".*", locales.HistoryMatchRegex(), &o.regex}} {
			if textToggle(c, t.glyph, t.tip, *t.on).Clicked() {
				*t.on = !*t.on
				changed = true
			}
		}
	})
	if w.side.seg != sideItems && w.side.hist.regex && w.side.hist.bad {
		ui.Text(c, locales.HistoryBadRegex()).FontSize(font2XS-1).TextColor(baseTokens(c).danger).Padding(0, 12, 4, 12)
	}
	if w.side.focusFilter {
		w.side.focusFilter = false
		filter.Focus()
	}
	if (filter.Changed() || changed) && w.side.seg != sideItems {
		w.loadSideQueries()
	}
	if filter.Focused() && c.Shortcut(0, ui.KeyEscape) {
		w.filter, w.side.showFilter = "", false
		if w.side.seg != sideItems {
			w.loadSideQueries()
		}
	}
}
