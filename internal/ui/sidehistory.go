package ui

// @author ixxxxoooo

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// 侧栏「历史」按天分组，每天一个可折叠的标题；「查询」（收藏）仍是平铺的列表。

// historyDay 是同一天执行的语句：key 是日期（2006-01-02），label 是标题上显示的文字。
type historyDay struct {
	key, label string
	items      []app.HistoryItem
}

// groupHistoryByDay 把按时间倒序排好的历史按本地日期分组，组的次序和组内次序都不变。
//
// 参数：items 历史（新的在前）；now 当前时间，用来把今天、昨天写成文字。
// 返回值：按天分好的组。
func groupHistoryByDay(items []app.HistoryItem, now time.Time) []historyDay {
	var out []historyDay
	for _, it := range items {
		key := it.ExecutedAt.In(now.Location()).Format("2006-01-02")
		if n := len(out); n == 0 || out[n-1].key != key {
			out = append(out, historyDay{key: key, label: dayLabel(key, now)})
		}
		out[len(out)-1].items = append(out[len(out)-1].items, it)
	}
	return out
}

// dayLabel 是一天的标题：今天、昨天，其余写日期。
func dayLabel(key string, now time.Time) string {
	switch key {
	case now.Format("2006-01-02"):
		return locales.HistoryToday()
	case now.AddDate(0, 0, -1).Format("2006-01-02"):
		return locales.HistoryYesterday()
	}
	return key
}

// historyList 是历史页的列表：每天一个标题（点一下折叠 / 展开），下面是当天的语句。
func (w *Workspace) historyList(c *ui.Context, p tokens, items []app.HistoryItem) {
	for _, day := range groupHistoryByDay(items, time.Now()) {
		section := "day:" + day.key
		w.sectionHeader(c, p, sideRow{section: section, label: day.label, count: len(day.items)})
		if w.side.collapsed[section] {
			continue
		}
		for _, it := range day.items {
			w.queryRow(c, p, it, false)
		}
	}
}

// queryRow 是历史 / 收藏里的一条语句；withDate 为假时只显示时间（按天分组的历史里，日期已在标题上）。
func (w *Workspace) queryRow(c *ui.Context, p tokens, it app.HistoryItem, withDate bool) {
	row := tipOn(c, ui.Column(c.Key(it.ID)).Gap(2).Padding(5, 8).Radius(6).FillWidth(), clipText(it.SQL, 600, 16))
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		if it.Name != "" {
			ui.Row(c).Gap(5).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Icon(c, icStarFill).FontSize(10).TextColor(p.warning).Shrink(0)
				ui.Text(c, it.Name).FontSize(font2XS + 1).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
			})
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
			// 历史/收藏里的语句和日志一样做语法高亮
			ui.RichText(c, sqlSpans(p, firstLines(it.SQL, 2), p.fg)...).FontSize(font2XS).Font(mono).SingleLine().MaxWidth(w.sidebarW - 40 - float32(len(it.Keyword)*7)).Shrink(1)
			if it.Keyword != "" {
				ui.Text(c, it.Keyword).FontSize(font2XS - 1).TextColor(p.accent).FontWeight(600).SingleLine()
			}
		})
		layout := "15:04"
		if withDate {
			layout = "01-02 15:04"
		}
		ui.Text(c, it.ExecutedAt.Format(layout)+histDB(it.Database)).FontSize(font2XS - 1).TextColor(p.fgMuted)
	})
	if row.Clicked() {
		w.newQuery(it.SQL)
	}
	row.ContextMenu(func(m *ui.Menu) { w.queryItemMenu(c, m, it) })
}
