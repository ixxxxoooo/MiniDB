package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// 跳到列（⌘L）：弹出一个搜索框，输入列名（模糊匹配），回车或点一下就选中当前行的这一列并滚过去。
// 列很多的宽表里，不用再横向翻找。

// jumpState 是跳到列弹窗的状态。
type jumpState struct {
	jumpOpen bool
	jumpQ    string
	jumpSel  int
}

// openJump 打开跳到列弹窗。
func (t *tableTab) openJump() {
	if len(t.g.vis) == 0 {
		return
	}
	t.jumpState = jumpState{jumpOpen: true}
}

// jumpCandidates 是弹窗里列出的列：显示着的列，按表头从左到右的顺序，再按输入模糊匹配。
func (t *tableTab) jumpCandidates() []string {
	var names []string
	for _, n := range t.g.displayOrder() {
		if !t.g.hidden[n] {
			names = append(names, n)
		}
	}
	return fuzzyColumns(names, t.jumpQ)
}

// jumpView 画弹窗。
func (t *tableTab) jumpView(c *ui.Context) {
	if !t.jumpOpen {
		return
	}
	sh := t.w.sh
	p := sh.tokens(c)
	shown := t.jumpCandidates()
	t.jumpSel = min(t.jumpSel, max(len(shown)-1, 0))
	choose := func(i int) {
		if i < 0 || i >= len(shown) {
			return
		}
		t.jumpOpen = false
		t.g.jumpTo(max(t.previewRow(), 0), shown[i])
	}
	sh.popup(c, &t.jumpOpen, 420, &t.jumpQ, locales.ColJumpSearch(), func() { t.jumpSel = 0 }, func() { choose(t.jumpSel) },
		func(n int) { t.jumpSel = min(max(t.jumpSel+n, 0), max(len(shown)-1, 0)) }, func() {
			if len(shown) == 0 {
				ui.Text(c, locales.ColJumpNone()).TextColor(p.fgMuted).Padding(14).TextAlign(ui.Center).FillWidth()
			}
			scrollbars(c, ui.Scroll(c.Key("jumpcols")).MaxHeight(320).FillWidth()).Children(func() {
				ui.Column(c).Gap(1).FillWidth().Children(func() {
					for i, name := range shown {
						i, name := i, name
						row := ui.Row(c.Key(name)).Gap(10).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
						switch {
						case i == t.jumpSel:
							row.Background(p.accent.Alpha(0.12))
						case row.Hovered():
							row.Background(p.hover)
						}
						row.Children(func() {
							ui.Text(c, name).Font(mono).FontSize(font2XS + 1).SingleLine().Grow(1).MinWidth(0)
							if t.g.pinned[name] {
								ui.Icon(c, icPin).FontSize(11).TextColor(p.fgMuted)
							}
							ui.Text(c, t.g.colMeta(name).Type).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Shrink(0)
						})
						if row.Clicked() {
							choose(i)
						}
					}
				})
			})
		})
}
