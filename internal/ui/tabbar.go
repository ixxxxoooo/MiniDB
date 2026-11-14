package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// tabBar is the strip of tabs, in the editor-tab style of VS Code: the active
// tab on the surface color under a line in the connection's color, the others
// recessed and divided by hairlines. A tab's close button shows only while the
// pointer is over it (or it is the active one, dimmed); a dot marks unsaved
// changes in its place. A middle click closes, and the strip scrolls sideways
// when the tabs outgrow it.
func (w *Workspace) tabBar(c *ui.Context, p tokens) {
	accent := connColor(c, cfgColor(w.cfg))
	middleDown := false
	// 底边的分隔线不是整条横贯的：每个没选中的标签、标签右边的空白和「＋」各自带一段，
	// 选中的标签没有这一段，所以它下面没有线，和内容区连成一体。
	// （绝对定位的整条线会盖在选中的标签上，所以不能那样画。）
	ui.Row(c).FillWidth().Height(sizeTab).Shrink(0).Background(p.toolbar).Children(func() {
		strip := ui.ScrollHorizontal(c.Key("tabstrip")).Shrink(1).MinWidth(0).Scrollbars(false) // 只占标签的宽度，剩下的给右边的空白
		strip.Children(func() {
			ui.Row(c).Height(sizeTab).AlignItems(ui.Stretch).Children(func() {
				for i, tb := range w.tabs {
					i, tb := i, tb
					active := i == w.active
					label := tb.title()
					tab := ui.Row(c.Key(fmt.Sprintf("tab-%d-%s", i, label))).Gap(6).AlignItems(ui.Center).Padding(0, 6, 2, 12).Height(sizeTab).MinWidth(96).MaxWidth(200).Shrink(0) // 下内边距 2px 抵消上边框的 2px，内容才在标签里垂直居中
					hovered := tab.Hovered()
					styleTab(tab, p, active, hovered, accent)
					closeClicked := false
					tab.Children(func() {
						fg := p.fgSecondary
						if active {
							fg = p.fg
						} else {
							tabBaseline(c, p)
						}
						ui.Row(c).Size(16, 16).Shrink(0).Center().Children(func() { // 图标和文字共用 16px 行盒，中心才对得齐
							ui.Icon(c, tb.icon()).FontSize(11).TextColor(tabIconColor(p, tb, active, fg))
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
									ui.Box(c).Size(7, 7).Radius(4).Background(p.fgSecondary)
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
					ui.Box(c.Key(fmt.Sprint("sep", i))).Width(1).FillHeight().Background(p.border).Shrink(0)
				}
			})
		})
		// 标签右边的空白：带着底线一路延伸到「＋」；双击它新建一个查询，和编辑器类应用的习惯一致
		rest := ui.Box(c.Key("tab-rest")).Grow(1).FillHeight()
		rest.Children(func() { tabBaseline(c, p) })
		if rest.DoubleClicked() || strip.DoubleClicked() {
			defer w.newQuery("")
		}
		// 标签栏右端的「＋」：新建查询，没有标签时也在，空标签栏不再是一片死区
		ui.Row(c.Key("tab-new")).Shrink(0).FillHeight().Padding(0, 4, 1, 4).Center().Children(func() {
			tabBaseline(c, p)
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
