package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// 表标签的小部件：子视图切换、延迟显示、加载提示。

// subViewPill is the rounded segmented switch of the bottom bar.
func (t *tableTab) subViewPill(c *ui.Context, p tokens) {
	// The original keeps these four in English in every language.
	if i := pillTabs(c, p, "sv", []string{"Data", "Structure", "DDL", "Doc"}, t.mode, false); i >= 0 && i != t.mode {
		t.mode = i
		t.modeChanged()
	}
}

// pillTabs 是应用统一的圆角分段开关（底部的 Data/Structure/DDL/Doc、侧栏的
// 项目/查询/历史都用它）。grow 为真时各段等分整行宽度。返回被点中的段，没有则 -1。
func pillTabs(c *ui.Context, p tokens, key string, labels []string, cur int, grow bool) (clicked int) {
	clicked = -1
	bh, fs := float32(20), float32(font2XS-1)
	if grow { // 侧栏顶部：更高一些
		bh, fs = 24, float32(font2XS)
	}
	draw := func() {
		row := ui.Row(c.Key(key)).Gap(0).Padding(2).Radius(radiusBtn + 2).Background(p.segTrack).AlignItems(ui.Center)
		if grow {
			row.FillWidth()
		}
		row.Children(func() {
			for i, l := range labels {
				i, l := i, l
				b := ui.Button(c.Key(fmt.Sprint(key, i)), "").Label(l).Padding(1, 10).Radius(radiusBtn).Height(bh)
				if grow {
					b.Grow(1).Basis(0)
				}
				if i == cur {
					b.Background(p.segThumb).Border(1, p.borderSubtle)
				} else {
					// 未选中的段也留着 1px 透明边框、字重也一样，切换时尺寸才不会抖
					b.Background(ui.Color{}).Border(1, ui.Transparent)
				}
				b.Children(func() {
					col := p.fgSecondary
					if i == cur {
						col = p.fg
					}
					ui.Text(c, l).FontSize(fs).TextColor(col).FontWeight(500).SingleLine()
				})
				if b.Clicked() {
					clicked = i
				}
			}
		})
	}
	if grow {
		// 外面套一行带内边距的容器，而不是给 FillWidth 的行加外边距（那样右边会超出一截，
		// 最右一段就被挡住了）；上面只留很小的空隙
		ui.Row(c.Key(key+"-wrap")).FillWidth().Padding(4, 8, 6, 8).Shrink(0).Children(draw)
	} else {
		draw()
	}
	return clicked
}

// delayed reports whether d has passed since start; until then it asks for
// a frame at that moment, so a state that only shows after a delay appears
// on time without anything else redrawing.
func delayed(c *ui.Context, start time.Time, d time.Duration) bool {
	if left := d - time.Since(start); left > 0 {
		c.After(left)
		return false
	}
	return true
}

// quietLoading fills a view that has nothing to show yet. It stays blank, and
// only after a good while shows a plain note: no spinner, no animation, so
// switching views never flashes a loading state.
func quietLoading(c *ui.Context, p tokens, since *time.Time) {
	if since.IsZero() {
		*since = time.Now()
	}
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		if delayed(c, *since, 800*time.Millisecond) {
			ui.Text(c, locales.CommonLoading()).FontSize(font2XS).TextColor(p.fgMuted)
		}
	})
}

// previewHandle is the line between grid and row pane; dragging it resizes the pane.
func (t *tableTab) previewHandle(c *ui.Context, p tokens) {
	dx, reset := vHandle(c, p, "phandle")
	t.previewW = min(max(t.previewW-dx, 220), 720)
	if reset {
		t.previewW = 340
	}
}
