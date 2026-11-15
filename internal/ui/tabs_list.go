package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 标签太多时的「已打开的标签」列表（设计稿）：标签条右端的「⌄ N」，点开一张浮层——标题带总数，下面是可筛选的标签列表
// （图标、名称、改动过的带橙点、悬停出 ×，选中的淡强调底并加粗），底部「关闭其他标签」。再点一次按钮收起。

// tabsMore 画标签条右端的「⌄ N」按钮和它的浮层。参数：p 是令牌。
func (w *Workspace) tabsMore(c *ui.Context, p tokens) {
	tip := locales.TabsMore()
	btn := tipOn(c, ui.Button(c.Key("tabs-more"), "").Label(tip), tip).Height(sizeBtn).Padding(0, spaceSM-2, 0, spaceSM).Gap(spaceXS).
		Radius(radiusBtn).BorderWidth(0).Background(ui.Color{}).Shrink(0).AlignItems(ui.Center)
	open := ui.Local(btn, "open", func() bool { return false })
	filter := ui.Local(btn, "filter", func() string { return "" })
	if *open || btn.Hovered() {
		btn.Background(p.hover)
	}
	btn.Children(func() {
		ui.Icon(c, icChevronD).FontSize(fontDense).TextColor(p.fgMuted)
		ui.Row(c.Key("tabs-n")).Height(sizeChip).MinWidth(sizeChip).Padding(0, spaceXS).Center().Radius(sizeChip / 2).Background(p.segTrack).Children(func() {
			ui.Text(c, itoa(len(w.tabs))).FontSize(fontMicro).FontWeight(600).TextColor(p.fgSecondary).SingleLine()
		})
	})
	if btn.Clicked() {
		*open = !*open
		*filter = ""
	}
	if !*open {
		return
	}
	closing := -1
	ui.PopoverBase(c, btn, open, func(panel ui.Element) {
		panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(spaceXS, 0, 0, 0).Width(w.tabsPopWidth()).MaxHeight(sizeTabsListH*3/2).
			Background(p.surfaceElev).Border(1, p.border).Radius(radiusPanel).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		enterPose(c, panel, posePop, ui.Snappy).Transition(popExit)
		ui.Column(c).FillWidth().Padding(spaceSM - 2).Gap(1).Children(func() {
			popupHeading(c, p, locales.TabsOpenedN(len(w.tabs)))
			ui.SearchField(c.Key("tabs-q"), filter).Placeholder(locales.TabsFilter()).FontSize(fontCtl).Height(sizeBtn).FillWidth().AutoFocus()
			q := strings.ToLower(strings.TrimSpace(*filter))
			shown := 0
			scrollbars(c, ui.Scroll(c.Key("tabs-list")).FillWidth().MaxHeight(sizeTabsListH)).Children(func() {
				ui.Column(c).Gap(1).FillWidth().Children(func() {
					for i, tb := range w.tabs {
						if q != "" && !strings.Contains(strings.ToLower(tb.title()), q) {
							continue
						}
						shown++
						if w.tabsRow(c, p, i, tb) {
							closing = i
						}
					}
				})
			})
			if shown == 0 {
				ui.Text(c, locales.TabsNoMatch()).TextColor(p.fgMuted).Padding(spaceLG).TextAlign(ui.Center).FillWidth()
			}
			hairline(c, p.borderSubtle).Margin(spaceXS, spaceSM)
			row := connPopRow(c, p, "tabs-others", sizeRow+2, false)
			row.Children(func() {
				ui.Icon(c, icX).FontSize(btnFieldIcon).TextColor(p.fgSecondary)
				ui.Text(c, locales.TabCloseOthers()).FontSize(fontCtl).SingleLine().Grow(1)
			})
			if row.Clicked() {
				*open = false
				defer w.closeOthers(w.active)
			}
		})
	})
	if closing >= 0 {
		w.closeTab(closing)
	}
}

// tabsRow 画列表里的一个标签；点它切过去，点 × 关闭（返回 true，由调用方在这一帧画完后关）。
func (w *Workspace) tabsRow(c *ui.Context, p tokens, i int, tb tab) (closed bool) {
	on := i == w.active
	row := connPopRow(c, p, "tabrow-"+itoa(i), sizeRow+2, on)
	hovered := row.Hovered()
	row.Children(func() {
		col := p.fgSecondary
		if on {
			col = p.accentText
		}
		ui.Icon(c, tb.icon()).FontSize(fontDense).TextColor(iconColor(p, tb.objKind())).Shrink(0) // 图标按类型着色，和标签条一致
		name := ui.Text(c, tb.title()).FontSize(fontCtl).TextColor(col).SingleLine().TruncateMiddle().Grow(1)
		if on {
			name.FontWeight(600)
		}
		switch {
		case hovered:
			if tipBtn(c.Key("tabrow-x"+itoa(i)), icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				closed = true
			}
		case tb.pending() > 0:
			ui.Box(c).Size(sizeDot+1, sizeDot+1).Radius(4).Background(p.warning) // 改动过的：橙点
		}
	})
	if !closed && row.Clicked() {
		w.active = i
	}
	return closed
}

// tabsPopWidth 是「已打开的标签」浮层的宽度：随最长的标签名，夹在 sizeTabsPop 和 sizeTabsPopMax 之间。
// 一行是 图标 + 名称 + 关闭位 + 内外边距，除名称外约占 tabsPopChrome。
func (w *Workspace) tabsPopWidth() float32 {
	var widest float32
	for _, tb := range w.tabs {
		widest = max(widest, estimateTextWidth(tb.title(), 6.5))
	}
	return min(max(widest+tabsPopChrome, sizeTabsPop), sizeTabsPopMax)
}

// tabsPopChrome 是浮层一行里名称以外的宽度：浮层内边距、行内边距、图标、间距和关闭位。
const tabsPopChrome = 84
