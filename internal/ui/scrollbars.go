package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 滚动条的显示方式（设置 → 外观）。
const (
	barsOnScroll = 0 // 只在滚动时出现，停下约 1 秒后淡出（默认）
	barsOnHover  = 1 // 指针停在区域上时显示
	barsAlways   = 2 // 内容溢出就一直显示
)

// barsMode 是当前的滚动条显示方式，随设置改变。
var barsMode = barsOnScroll

const (
	barsHold = 800 * time.Millisecond // 滚动停下后保持完全可见的时间
	barsFade = 500 * time.Millisecond // 之后淡出所花的时间
	barsTail = barsHold + barsFade + 50*time.Millisecond
)

// barWatch 记着一个滚动区域上次滚动的位置和时间，用来判断它是否刚刚动过。
type barWatch struct {
	st   ui.ScrollState
	y, x float32
	at   time.Time
}

// activity 返回 0~1：刚滚动过为 1，停下后逐渐变 0。
func (w *barWatch) activity(now time.Time) float32 {
	if w.at.IsZero() {
		return 0
	}
	d := now.Sub(w.at)
	switch {
	case d <= barsHold:
		return 1
	case d >= barsHold+barsFade:
		return 0
	}
	return 1 - float32(d-barsHold)/float32(barsFade)
}

// poll 看一眼滚动位置有没有变，变了就记下时间；淡出期间要求继续重绘。
func (w *barWatch) poll(c *ui.Context, x, y float32) float32 {
	now := time.Now()
	if x != w.x || y != w.y {
		w.x, w.y, w.at = x, y, now
	}
	a := w.activity(now)
	if !w.at.IsZero() && now.Sub(w.at) < barsTail {
		c.Invalidate()
	}
	return a
}

// scrollbars 按设置的显示方式装饰一个滚动区域（Scroll、List、Table）。
// 用法：scrollbars(c, ui.Scroll(c.Key("x")).Grow(1)).Children(...)
func scrollbars(c *ui.Context, el ui.Element) ui.Element {
	return scrollbarsOn(c, el, nil)
}

// scrollbarsOn 同 scrollbars，但滚动位置记在调用方给的 st 里（要自己控制滚动，如跟随末尾时）。
func scrollbarsOn(c *ui.Context, el ui.Element, st *ui.ScrollState) ui.Element {
	switch barsMode {
	case barsAlways:
		return el.KeepScrollbars(true)
	case barsOnHover:
		return el
	}
	w := ui.Local(el, "scrollbars", func() barWatch { return barWatch{} })
	if w == nil {
		return el
	}
	if st == nil {
		st = &w.st
	}
	return barsLook(c, el.TrackScroll(st), w.poll(c, st.X, st.Y))
}

// listBars 同 scrollbars，用于 List 和 Table：它们不走 TrackScroll，
// 靠 ListState 里看得见的第一行来判断有没有在滚动。
func listBars(c *ui.Context, el ui.Element, ls *ui.ListState) ui.Element {
	switch barsMode {
	case barsAlways:
		return el.KeepScrollbars(true)
	case barsOnHover:
		return el
	}
	w := ui.Local(el, "scrollbars", func() barWatch { return barWatch{} })
	if w == nil {
		return el
	}
	first, _ := ls.Visible()
	return barsLook(c, el, w.poll(c, 0, float32(first)))
}

// barsLook 按活跃度 a（0~1）给滚动区域上色：静止时连同点击区域一起收起，
// 滚动一下就会再出现。
func barsLook(c *ui.Context, el ui.Element, a float32) ui.Element {
	if a <= 0 {
		return el.Scrollbars(false)
	}
	// 保持显示时库一律用 Hover 色画滑块，所以三种颜色都设成同一个渐隐色
	th := c.Theme()
	thumb := th.Text.Alpha(0.4 * a)
	return el.KeepScrollbars(true).ScrollbarStyle(ui.ScrollbarStyle{Thumb: thumb, Hover: thumb, Active: th.Text.Alpha(0.65)})
}

// editorBars 把滚动条显示方式应用到一个代码编辑器上。编辑器没有淡出，
// 所以"滚动时显示"就是滚动后约一秒内保持显示，其余时间收起。
func editorBars(c *ui.Context, ed *editor.State) {
	switch barsMode {
	case barsAlways:
		ed.KeepScrollbars, ed.NoScrollbars = true, false
	case barsOnHover:
		ed.KeepScrollbars, ed.NoScrollbars = false, false
	default:
		w := barsWatches[ed]
		if w == nil {
			w = &barWatch{}
			barsWatches[ed] = w
		}
		on := w.poll(c, 0, float32(ed.ScrollY())) > 0
		ed.KeepScrollbars, ed.NoScrollbars = on, !on
	}
}

// barsWatches 记着各个编辑器的滚动观察者。
var barsWatches = map[*editor.State]*barWatch{}
