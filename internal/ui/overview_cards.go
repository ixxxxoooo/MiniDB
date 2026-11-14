package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 项目概览的上半部分（设计稿「项目概览」）：四张摘要卡（表 / 总大小 / 估算行数 / 最近更新）、
// 全部 / 表 / 视图分段加筛选条，以及数据大小列里的占比条。

// overviewKind 是概览里的类型筛选：0 全部，1 只看表，2 只看视图。
const (
	ovAll = iota
	ovTables
	ovViews
)

// overviewStats 是摘要卡用的汇总。
type overviewStats struct {
	tables, views        int
	data, index          int64
	rows                 int64
	bigName              string
	bigRows              int64
	recentName, recentAt string
	maxData              int64 // 数据大小的最大值，占比条的 100%
}

// statsOf 汇总一批表和视图；大小、行数里的负数（未知）不计。
func statsOf(items []app.TableOverview) overviewStats {
	var s overviewStats
	for _, it := range items {
		if it.Type == "view" {
			s.views++
			continue
		}
		s.tables++
		s.data += max(it.DataSize, 0)
		s.index += max(it.IndexSize, 0)
		s.rows += max(it.Rows, 0)
		s.maxData = max(s.maxData, it.DataSize)
		if it.Rows > s.bigRows {
			s.bigName, s.bigRows = it.Name, it.Rows
		}
		if it.UpdatedAt > s.recentAt {
			s.recentName, s.recentAt = it.Name, it.UpdatedAt
		}
	}
	return s
}

// compactCount 把行数缩成「数值 + 单位」：1204118 → 1.20 M，8214 → 8,214 不缩，超过一万才用 K。
func compactCount(n int64) (num, unit string) {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2f", float64(n)/1e6), "M"
	case n >= 10_000:
		return fmt.Sprintf("%.1f", float64(n)/1e3), "K"
	}
	return itoa(int(n)), ""
}

// splitSize 把 humanBytes 的结果拆成数值和单位（「28.0 KB」→「28.0」「KB」），摘要卡里数值大、单位小。
func splitSize(n int64) (num, unit string) {
	num, unit, _ = strings.Cut(humanBytes(max(n, 0)), " ")
	return
}

// summaryCards 画四张摘要卡。
func (o *overviewTab) summaryCards(c *ui.Context, p tokens) {
	st := statsOf(o.items)
	sizeNum, sizeUnit := splitSize(st.data + st.index)
	rowNum, rowUnit := compactCount(st.rows)
	recent, recentAt := orDash(st.recentName), orDash(st.recentAt)
	card := func(key, title, num, unit, sub string) {
		ui.Column(c.Key(key)).Gap(2).Padding(10, spaceMD+2).Grow(1).Basis(0).MinWidth(0).Radius(radiusPanel).Background(p.surface).Border(1, p.borderSubtle).Children(func() {
			ui.Text(c, title).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine()
			ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
				ui.Text(c, num).FontSize(fontHeading).FontWeight(600).LetterSpacing(-0.3).SingleLine().Shrink(1).MinWidth(0)
				if unit != "" {
					ui.Text(c, unit).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine().Margin(0, 0, 2, 0)
				}
			})
			ui.Text(c, sub).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		})
	}
	ui.Row(c.Key("ov-cards")).Gap(spaceMD).Padding(spaceMD, spaceMD+2, spaceMD, spaceMD+2).FillWidth().Shrink(0).Children(func() {
		card("ov-c1", locales.OverviewCardTables(), itoa(st.tables), "", locales.OverviewCardTablesSub(itoa(st.views)))
		card("ov-c2", locales.OverviewCardSize(), sizeNum, sizeUnit, locales.OverviewCardSizeSub(humanBytes(st.data), humanBytes(st.index)))
		bn, br := compactCount(st.bigRows)
		card("ov-c3", locales.OverviewCardRows(), rowNum, rowUnit, locales.OverviewCardRowsSub(orDash(st.bigName), bn+strings.TrimSpace(" "+br)))
		card("ov-c4", locales.OverviewCardRecent(), recent, "", recentAt)
	})
}

// filterBar 画摘要卡下面的条：类型分段 + 筛选框 + 提示 + 刷新。
func (o *overviewTab) filterBar(c *ui.Context, p tokens) {
	st := statsOf(o.items)
	ui.Row(c.Key("ov-bar")).Gap(spaceSM).Padding(0, spaceSM+2).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Children(func() {
		if segmented(c, &o.kind, locales.OverviewKindAll(itoa(st.tables+st.views)), locales.OverviewKindTables(itoa(st.tables)), locales.OverviewKindViews(itoa(st.views))) {
			o.sel = -1
		}
		ui.SearchField(c.Key("ovq"), &o.q).Placeholder(locales.OverviewFilterPlaceholder()).Height(sizeBtnSm).FontSize(fontCtl).Width(200).Margin(0, 0, 0, spaceXS)
		ui.Spacer(c)
		ui.Text(c, locales.OverviewFilterHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
		if o.loading {
			ui.Spinner(c)
		}
		if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			o.load()
		}
	})
	hairline(c, p.border)
}

// dataSizeBar 是数据大小列里的占比条加数值：条的长度是它占最大那张表的比例，至少留一小截让非零的表看得见。
func dataSizeBar(c *ui.Context, p tokens, size, maxSize int64) {
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		frac := float32(0)
		if size > 0 && maxSize > 0 {
			frac = max(float32(size)/float32(maxSize), 0.015)
		}
		ui.Box(c).Grow(1).Height(sizeDot).Radius(sizeDot / 2).Background(p.surfaceTertiary).Clip().Children(func() {
			if frac > 0 {
				ui.Box(c).WidthPercent(frac * 100).FillHeight().Radius(sizeDot / 2).Background(p.accent.Alpha(0.75))
			}
		})
		ui.Text(c, overviewSize(size)).Font(mono).FontSize(fontCaption + 0.5).SingleLine().TextColor(p.fgSecondary).Width(58).TextAlign(ui.End)
	})
}
