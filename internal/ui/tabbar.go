package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// tabBar 是标题栏里的标签条（设计稿 A 的药丸标签）：选中的标签是带 1px 描边的卡片底圆角块，
// 悬浮的淡淡铺一层底，其余的没有底色，标签之间不画分隔线。
// 标签的关闭按钮只在指针停在上面时出现，未保存的修改用橙色小点占着同一个位置；
// 中键点击直接关闭，标签放不下时条里可以横向滚动。
func (w *Workspace) tabBar(c *ui.Context, p tokens) {
	middleDown := false
	// 标题栏的底线由 titleBar 先画；标签条自己没有背景，选中的标签带着内容区的底色盖在那条线上
	// 标签之间和右边的空白也是标题栏的一部分：按住可以拖动窗口，双击最大化
	ui.Row(c).Grow(1).MinWidth(0).Height(sizeTab).Shrink(1).DragWindow().Children(func() {
		strip := ui.ScrollHorizontal(c.Key("tabstrip")).Grow(1).MinWidth(0).Scrollbars(false).DragWindow()
		revealed := ui.Local(strip, "revealed", func() string { return "" })
		strip.Children(func() {
			ui.Row(c).Height(sizeTab).Gap(2).AlignItems(ui.Stretch).DragWindow().Children(func() {
				for i, tb := range w.tabs {
					i, tb := i, tb
					active := i == w.active
					label := tb.title()
					tab := ui.Row(c.Key(fmt.Sprintf("tab-%d-%s", i, label))).Gap(spaceSM-2).AlignItems(ui.Center).Padding(0, spaceSM, 0, spaceSM+2).Height(sizeTab).MaxWidth(sizeTabMax).Shrink(0) // 药丸标签：高 28、内边距 0 8 0 10、最宽 170
					hovered := tab.Hovered()
					styleTitleTab(tab, p, active, hovered)
					if active { // 设计稿：选中的标签滚出可视区时吸附在条的左 / 右端，永远看得见
						tab.Sticky().Left(spaceXS).Right(spaceXS)
						if key := fmt.Sprint(i, label); *revealed != key { // 刚切到它：滚进可视区一次（每帧都滚会不让人往别处滚）
							*revealed = key
							tab.ScrollIntoView()
						}
					}
					closeClicked := false
					tab.Children(func() {
						fg := p.fgMuted
						if active {
							fg = p.fg
						}
						ui.Row(c).Size(16, 16).Shrink(0).Center().Children(func() { // 图标和文字共用 16px 行盒，中心才对得齐
							ui.Icon(c, tb.icon()).FontSize(fontDense).TextColor(tabIconColor(p, tb, active, fg))
						})
						lbl := ui.Text(c, label).FontSize(fontCtl).FontWeight(500).FixedLineHeight(16).SingleLine().TextColor(fg).Grow(1).TruncateMiddle()
						if isPreviewTab(tb) {
							lbl.Italic()
						}
						// the close slot keeps its width, so tabs never change size
						ui.Box(c.Key("slot")).Size(16, 16).Shrink(0).Radius(4).Children(func() {
							ui.Column(c).Fill().Center().Children(func() {
								switch {
								case hovered || (active && tb.pending() == 0): // 设计稿：选中的标签始终带关闭按钮
									x := iconButton(c.Key("x"), icX, locales.CommonClose()).Size(16, 16).Radius(4)
									if x.Hovered() {
										x.Background(p.hover)
									}
									if x.Clicked() {
										closeClicked = true
									}
								case tb.pending() > 0:
									ui.Box(c).Size(sizeDot+1, sizeDot+1).Radius(4).Background(p.warning) // 未保存：橙色小点
								}
							})
						})
					})
					if closeClicked {
						defer w.closeTab(i)
					} else if tab.DoubleClicked() {
						w.active = i
						if t, ok := tb.(*tableTab); ok {
							t.keep()
						}
					} else if tab.Clicked() {
						w.active = i
					}
					// A middle click closes the tab at once, on the press.
					if b, down := tab.PressedButton(); down && b == 2 {
						middleDown = true
						if key := fmt.Sprint(i, label); w.midKey != key {
							w.midKey = key
							defer func() { w.closeTab(i); w.sh.redraw() }()
						}
					}
					tab.ContextMenu(func(m *ui.Menu) {
						if t, ok := tb.(*tableTab); ok && t.preview && m.Item(locales.TabKeepOpen()).Chosen() {
							defer t.keep()
						}
						if m.Item(locales.CommonClose()).Chosen() {
							defer w.closeTab(i)
						}
						if m.Item(locales.TabCloseOthers()).Chosen() {
							defer w.closeOthers(i)
						}
					})
				}
			})
		})
		// 空白处是拖动手柄：不能再读 DoubleClicked / Clicked——那会让它变成可点击元素，按下就不再拖动窗口。
		// 双击由窗口接手（最大化 / 还原），所以这里什么都不读；新建查询靠右边的「＋」。
		// 标签栏右端的「＋」：新建查询，没有标签时也在，空标签栏不再是一片死区
		ui.Row(c.Key("tab-new")).Shrink(0).FillHeight().Padding(0, 4, 1, 4).Center().Children(func() {
			if tipBtn(c.Key("new-q"), icPlus, locales.MenuNewQuery()+w.sh.keyHint("new-query")).Size(sizeBtn, sizeBtn).Radius(radiusBtn).Clicked() {
				defer w.newQuery("")
			}
		})
		if len(w.tabs) > 0 {
			w.tabsMore(c, p) // 标签太多时：已打开的标签列表
		}
	})
	if !middleDown {
		w.midKey = ""
	}
}

// tabIconColor colours a tab's icon by what the tab is; inactive tabs keep
// the kind's hue, a little recessed.
func tabIconColor(p tokens, tb tab, active bool, fallback ui.Color) ui.Color {
	col := iconColor(p, tb.objKind())
	if !active {
		col = col.Alpha(0.8)
	}
	return col
}

// bellButton opens the notifications; a dot says there are new ones.
func (s *Shell) bellButton(c *ui.Context, p tokens) {
	fresh := len(s.notes) - s.notesSeen
	tip := locales.ToolbarNotes()
	b := tipOn(c, ui.Button(c.Key(tip), "").Label(tip), tip).Size(sizeBtn, sizeBtn).Padding(0).BorderWidth(0).Background(ui.Color{})
	col := p.fgSecondary
	if b.Hovered() {
		col = p.fg
	}
	b.Children(func() {
		ui.Icon(c, icBell).FontSize(15).TextColor(col)
		if fresh > 0 { // 有没看过的通知：右上角一个红点，有失败的就更醒目
			dot := p.accent
			for _, n := range s.notes[s.notesSeen:] {
				if n.kind == kindError {
					dot = p.danger
				}
			}
			ui.Box(c).Absolute().Top(1).Right(2).Size(sizeDot+2, sizeDot+2).Radius((sizeDot+2)/2).Background(dot).Border(1.5, p.surfaceTertiary)
		}
	})
	if b.Clicked() {
		s.notesOpen = !s.notesOpen
		s.notesSeen = len(s.notes)
	}
	s.notesPopover(c, p, b)
}

// sizeTabMax 是标题栏标签的最大宽度。
const sizeTabMax = 170

// styleTitleTab 给标题栏里的一个标签套上选中 / 悬浮 / 普通三种外观：都留 1px 描边（只有选中的有颜色），
// 三种状态尺寸一致，切换时内容不会抖。选中的是卡片底 + 细描边的圆角块（设计稿 A）。
func styleTitleTab(tab ui.Element, p tokens, active, hovered bool) ui.Element {
	tab = tab.Radius(radiusBtn).BorderWidth(1)
	switch {
	case active:
		return tab.Background(p.surface).BorderColor(p.border)
	case hovered:
		return tab.Background(p.hover).BorderColor(ui.Color{})
	}
	return tab.BorderColor(ui.Color{})
}
