package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// tabBar 是标题栏里的标签条：选中的标签铺内容区底色、顶上压一道强调色线，
// 和下面的内容连成一体（盖住标题栏的底线），其余的没有底色，标签之间不画分隔线。
// 标签的关闭按钮只在指针停在上面时出现，未保存的修改用橙色小点占着同一个位置；
// 中键点击直接关闭，标签放不下时条里可以横向滚动。
func (w *Workspace) tabBar(c *ui.Context, p tokens) {
	middleDown := false
	// 标题栏的底线由 titleBar 先画；标签条自己没有背景，选中的标签带着内容区的底色盖在那条线上
	ui.Row(c).Grow(1).MinWidth(0).Height(sizeTab).Shrink(1).Children(func() {
		strip := ui.ScrollHorizontal(c.Key("tabstrip")).Grow(1).MinWidth(0).Scrollbars(false)
		strip.Children(func() {
			ui.Row(c).Height(sizeTab).Gap(1).AlignItems(ui.Stretch).Children(func() {
				for i, tb := range w.tabs {
					i, tb := i, tb
					active := i == w.active
					label := tb.title()
					tab := ui.Row(c.Key(fmt.Sprintf("tab-%d-%s", i, label))).Gap(6).AlignItems(ui.Center).Padding(0, spaceSM-2, 2, spaceSM+2).Height(sizeTab).MinWidth(96).MaxWidth(200).Shrink(0) // 下内边距 2px 抵消上边框的 2px，内容才在标签里垂直居中
					hovered := tab.Hovered()
					styleTitleTab(tab, p, active, hovered)
					closeClicked := false
					tab.Children(func() {
						fg := p.fgSecondary
						if active {
							fg = p.fg
						}
						ui.Row(c).Size(16, 16).Shrink(0).Center().Children(func() { // 图标和文字共用 16px 行盒，中心才对得齐
							ui.Icon(c, tb.icon()).FontSize(fontCaption).TextColor(tabIconColor(p, tb, active, fg))
						})
						lbl := ui.Text(c, label).FontSize(font2XS).FixedLineHeight(16).SingleLine().TextColor(fg).Grow(1).TruncateMiddle()
						if isPreviewTab(tb) {
							lbl.Italic()
						}
						// the close slot keeps its width, so tabs never change size
						ui.Box(c.Key("slot")).Size(18, 18).Shrink(0).Radius(4).Children(func() {
							ui.Column(c).Fill().Center().Children(func() {
								switch {
								case hovered:
									x := iconButton(c.Key("x"), icX, locales.CommonClose()).Size(18, 18).Radius(4)
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
		// 双击标签栏的空白处（标签之外的部分）新建一个查询，和编辑器类应用的习惯一致
		if strip.DoubleClicked() {
			defer w.newQuery("")
		}
		// 标签栏右端的「＋」：新建查询，没有标签时也在，空标签栏不再是一片死区
		ui.Row(c.Key("tab-new")).Shrink(0).FillHeight().Padding(0, 4, 1, 4).Center().Children(func() {
			if tipBtn(c.Key("new-q"), icPlus, locales.MenuNewQuery()+w.sh.keyHint("new-query")).Size(sizeBtn, sizeBtn).Radius(radiusBtn).Clicked() {
				defer w.newQuery("")
			}
		})
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
			ui.Box(c).Absolute().Top(5).Right(5).Size(7, 7).Radius(3.5).Background(dot).Border(1.5, p.toolbar)
		}
	})
	if b.Clicked() {
		s.notesOpen = !s.notesOpen
		s.notesSeen = len(s.notes)
	}
	s.notesPopover(c, p, b)
}

// styleTitleTab 给标题栏里的一个标签套上选中 / 悬浮 / 普通三种外观：顶部固定留 2px 边框（普通态透明），
// 三种状态高度一致，切换时内容不会抖。上两角圆角，下两角直，和内容区接上。
func styleTitleTab(tab ui.Element, p tokens, active, hovered bool) ui.Element {
	tab = tab.Radius(radiusPanel-2, radiusPanel-2, 0, 0)
	switch {
	case active:
		return tab.Background(p.surface).BorderWidth(2, 0, 0, 0).BorderColor(p.accent)
	case hovered:
		return tab.Background(p.hover).BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
	}
	return tab.BorderWidth(2, 0, 0, 0).BorderColor(ui.Color{})
}
