package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 执行计划整页（设计稿 A「执行计划」）：点结果区的「执行计划」页签后，查询标签的整个页面换成它——
// 顶部条（← 返回查询、树 / 原文、ANALYZE 开关、复制），下面两张摘要卡，再下面是一张圆角卡片里的计划树：
// 操作（缩进 + 图标 + 粗体算子 + 灰色等宽的对象和条件）、行数、代价、耗时占比条；最费时的一行铺红底。

const (
	planRowH     = 34  // 计划树一行的高度
	planColRows  = 110 // 行数列宽
	planColCost  = 90  // 代价列宽
	planColTime  = 200 // 耗时列宽
	planIndent   = 20  // 每一层缩进
	planBarFloor = 0.03
)

// planPage 画执行计划整页。
func (q *queryTab) planPage(c *ui.Context, p tokens) {
	ps := q.plan
	ui.Toolbar(c, func() {
		if ghostBtn(c.Key("plan-back"), icBack, locales.ExplainBack()).Clicked() {
			q.page = pageData
		}
		if ps.res != nil && len(ps.res.Roots) > 0 {
			vline(c, p.border, 14)
			segmented(c.Key("plan-seg"), &ps.tab, locales.ExplainTree(), locales.ExplainRaw())
		}
		if app.CanExplainAnalyze(q.w.cfg.Type) {
			on := q.planAnalyze
			ui.Row(c.Key("plan-analyze")).Gap(spaceSM).AlignItems(ui.Center).Margin(0, 0, 0, spaceSM).Children(func() {
				ui.Switch(c, &on)
				ui.Text(c, locales.ExplainAnalyzeLabel()).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine()
			})
			if on != q.planAnalyze {
				q.planAnalyze = on
				q.explainStmt(ps.stmt, on)
			}
			ui.Text(c, locales.ExplainAnalyzeHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		}
		ui.Spacer(c)
		if ps.loading {
			ui.Spinner(c)
		}
		if ps.res != nil && fieldBtn(c.Key("plan-copy"), icCopy, locales.CommonCopy()).Clicked() {
			c.Services().WriteClipboard(ps.res.Raw)
			q.w.sh.toast(locales.StatusCopied())
		}
	}).Gap(spaceSM).Padding(0, spaceSM+2).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0)
	hairline(c, p.border)
	switch {
	case ps.loading && ps.res == nil:
		ui.Row(c).Grow(1).FillWidth().Center().Gap(spaceSM).Children(func() {
			ui.Spinner(c)
			ui.Text(c, locales.ExplainRunning()).FontSize(fontCtl).TextColor(p.fgMuted)
		})
	case ps.err != "":
		q.banner(c, p, kindError, ps.err, nil)
	case ps.res != nil:
		q.planContent(c, p, ps)
	}
}

// planContent 是顶部条下面的内容：说明、摘要卡、计划树或原文。
func (q *queryTab) planContent(c *ui.Context, p tokens, ps *planState) {
	res := ps.res
	if res.Note != "" {
		ui.Row(c).Gap(spaceSM).Padding(spaceSM, spaceMD+2).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
			ui.Icon(c, icInfo).FontSize(btnActIcon).TextColor(p.warning)
			ui.Text(c, res.Note).FontSize(fontCtl).TextColor(p.fgSecondary)
		})
	}
	if ps.tab == 1 || len(res.Roots) == 0 {
		scrollbars(c, ui.ScrollBoth(c.Key("plan-raw")).Grow(1).FillWidth()).Children(func() {
			ui.Text(c, res.Raw).Font(mono).FontSize(fontDense).Selectable().Padding(spaceMD, spaceMD+2)
		})
		return
	}
	flat := res.Flat()
	total, scan := ps.dur.Seconds()*1000, 0.0
	if res.Analyzed {
		total = 0
		for _, r := range res.Roots {
			total += r.ActualMs
		}
	}
	for _, r := range flat {
		if len(r.Node.Children) == 0 {
			scan += r.Node.Rows
		}
	}
	ui.Row(c.Key("plan-cards")).Gap(spaceMD).Padding(spaceMD, spaceMD+2).FillWidth().Shrink(0).Children(func() {
		tn, tu := planMsParts(total)
		planCard(c, p, locales.ExplainTotal(), tn, tu)
		sn, su := planNumParts(scan)
		planCard(c, p, locales.ExplainScanRows(), sn, su)
		ui.Box(c).Grow(2).Basis(0) // 右边留给优化建议（暂无数据来源，留白）
	})
	scrollbars(c, ui.Scroll(c.Key("plan-tree")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).Padding(0, spaceMD+2, spaceMD+2, spaceMD+2).MinWidth(0).Children(func() {
			ui.Column(c).FillWidth().Radius(radiusPanel).Background(p.surface).Border(1, p.border).Clip().Children(func() {
				q.planRows(c, p, ps, flat, total)
			})
		})
	})
}

// planCard 是一张摘要卡：标题、大号数值加小号单位。
func planCard(c *ui.Context, p tokens, title, num, unit string) {
	ui.Column(c.Key("pc-"+title)).Gap(2).Padding(spaceMD-2, spaceMD+2).Grow(1).Basis(0).MinWidth(0).Radius(radiusPanel).Background(p.surface).Border(1, p.borderSubtle).Children(func() {
		ui.Text(c, title).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		ui.Row(c).Gap(spaceXS).AlignItems(ui.End).Children(func() {
			ui.Text(c, num).FontSize(fontHeading).FontWeight(600).LetterSpacing(-0.3).SingleLine()
			if unit != "" {
				ui.Text(c, unit).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine().Margin(0, 0, 2, 0)
			}
		})
	})
}

// planMsParts 把毫秒数拆成「数值」「单位」。
func planMsParts(ms float64) (string, string) {
	if ms >= 1000 {
		return fmt.Sprintf("%.2f", ms/1000), "s"
	}
	return fmt.Sprintf("%.1f", ms), "ms"
}

// planNumParts 把行数拆成「数值」「单位」（1.2 K / 3.4 M）。
func planNumParts(n float64) (string, string) {
	s := planNum(n)
	if s == "" {
		return "0", ""
	}
	if last := s[len(s)-1]; last < '0' || last > '9' {
		return s[:len(s)-1], string(last)
	}
	return s, ""
}

// planRows 画计划树的表头和各行。
func (q *queryTab) planRows(c *ui.Context, p tokens, ps *planState, flat []model.PlanRow, total float64) {
	res := ps.res
	ui.Row(c.Key("ph")).Height(sizeHead).Padding(0, spaceMD).AlignItems(ui.Center).FillWidth().Background(p.surfaceSecondary).Children(func() {
		head := func(t string) ui.Element {
			return ui.Text(c, t).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		}
		head(locales.ExplainColOp()).Grow(1)
		head(locales.ExplainColRows()).Width(planColRows).TextAlign(ui.End)
		head(locales.ExplainColCost()).Width(planColCost).TextAlign(ui.End)
		if res.Analyzed {
			head(locales.ExplainColTime()).Width(planColTime).Padding(0, 0, 0, spaceLG)
		}
	})
	hairline(c, p.border)
	maxMs := 0.0
	for _, r := range flat {
		maxMs = max(maxMs, r.Node.ActualMs)
	}
	for i, pr := range flat {
		n := pr.Node
		hot := n == ps.hot
		row := ui.Row(c.Key(fmt.Sprint("pr", i))).Height(planRowH).Padding(0, spaceMD).AlignItems(ui.Center).FillWidth()
		if hot {
			row.Background(p.dangerBg)
		} else if row.Hovered() {
			row.Background(p.hover)
		}
		row.Children(func() {
			ui.Row(c.Key("op")).Gap(spaceSM-2).AlignItems(ui.Center).Grow(1).MinWidth(0).Padding(0, 0, 0, float32(pr.Depth)*planIndent).Children(func() {
				if pr.Depth > 0 {
					ui.Text(c, "└").FontSize(fontCaption).TextColor(p.fgFaint)
				}
				ui.Icon(c, planIcon(n.Op)).FontSize(btnActIcon + 2).TextColor(p.fgSecondary).Shrink(0)
				ui.Text(c, n.Op).FontWeight(600).TextColor(p.fg).SingleLine().Shrink(0)
				detail := strings.TrimSpace(n.Object + " " + n.Extra)
				if n.Object != "" && n.Extra != "" {
					detail = n.Object + " · " + n.Extra
				}
				ui.Text(c, detail).Font(mono).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
			})
			planMono(c, planNum(n.Rows), p.fg).Width(planColRows).TextAlign(ui.End)
			planMono(c, planNum(n.Cost), p.fgSecondary).Width(planColCost).TextAlign(ui.End)
			if res.Analyzed {
				q.planTimeCell(c, p, n.ActualMs, maxMs, hot)
			}
		})
		hairline(c, p.borderSubtle)
	}
	_ = total
}

// planMono 是行数、代价这类右对齐的等宽数字。
func planMono(c *ui.Context, s string, col ui.Color) ui.Element {
	return ui.Text(c, s).Font(mono).FontSize(fontCaption + 0.5).TextColor(col).SingleLine().Shrink(0)
}

// planTimeCell 是耗时列：占最大耗时比例的条加数值，最费时的一行条和数字都是红的。
func (q *queryTab) planTimeCell(c *ui.Context, p tokens, ms, maxMs float64, hot bool) {
	ui.Row(c.Key("pt")).Gap(spaceSM).AlignItems(ui.Center).Width(planColTime).Padding(0, 0, 0, spaceLG).Shrink(0).Children(func() {
		frac := float32(planBarFloor)
		if maxMs > 0 {
			frac = max(float32(ms/maxMs), planBarFloor)
		}
		col := p.accent
		txt := p.fg
		if hot {
			col, txt = p.danger, p.danger
		}
		ui.Box(c).Grow(1).Height(sizeDot).Radius(sizeDot / 2).Background(p.surfaceTertiary).Clip().Children(func() {
			ui.Box(c).WidthPercent(frac * 100).FillHeight().Radius(sizeDot / 2).Background(col)
		})
		t := planMono(c, strings.TrimSuffix(planMs(ms), " ms"), txt).Width(planColTime / 4).TextAlign(ui.End)
		if hot {
			t.FontWeight(600)
		}
	})
}

// planIcon 按算子名挑图标：连接是网络图，扫描是表，其余是层叠。
func planIcon(op string) *ui.SVG {
	switch {
	case strings.Contains(op, "Join"):
		return icER
	case strings.Contains(op, "Scan"), strings.Contains(op, "SCAN"), strings.Contains(op, "SEARCH"):
		return objectIcon(KindTable)
	}
	return icGrid
}
