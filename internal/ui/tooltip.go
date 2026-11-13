package ui

import (
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// 悬浮提示的统一规范：
//   - 字号固定为 tipFont（10），不随所在区域的主题缩放（数据表用的是紧凑主题，
//     框架自带的 Tooltip 会按主题字号减 1，结果各处大小不一）；
//   - 指针在元素上停留满 tipDelay 才显示，不会一碰就弹出。框架自带的"刚看过一个提示，
//     下一个立即出现"的热身逻辑也会让提示立即出现，所以这里自己计时，
//     只有停留够久了才把元素交给框架去显示（此时框架的延迟为 0，见 applyLook）；
//   - 外观与弹出菜单、提示卡同一套：浮层底色、细边框、柔和的阴影。
const (
	tipFont  = 10
	tipDelay = 600 * time.Millisecond
)

// tipHover 记录指针在某个元素上从什么时候开始停留。
type tipHover struct {
	since time.Time
	armed bool // 已经交给框架显示过了；第一次交出去之后要再排一帧，框架下一帧才认得它
}

// tipWakePending 为 1 时表示已经有一个"到点重绘"的定时器在等了，避免每帧都起一个 goroutine。
var tipWakePending atomic.Bool

// tipRedraw 由 Shell 设置：在 UI 线程上请求重绘一帧。
var tipRedraw func()

// wakeAfter 在 d 之后触发一次重绘。指针停着不动时界面不会自己刷新，
// 没有这一下，停留时间到了提示也不会出现。
func wakeAfter(d time.Duration) {
	if tipRedraw == nil || !tipWakePending.CompareAndSwap(false, true) {
		return
	}
	go func() {
		time.Sleep(d + 10*time.Millisecond)
		tipWakePending.Store(false)
		tipRedraw()
	}()
}

// tipOn 给 e 挂上悬浮提示 text：停留 tipDelay 之后显示，字号 tipFont。
// 返回 e 自己，方便接着链式调用。
func tipOn(c *ui.Context, e ui.Element, text string) ui.Element {
	return tipOnFont(c, e, text, "")
}

// tipOnFont 同 tipOn，font 为空用界面字体；单元格内容这类要对齐阅读的提示用等宽字体。
func tipOnFont(c *ui.Context, e ui.Element, text, font string) ui.Element {
	if text == "" {
		return e
	}
	st := ui.Local(e, "tip-hover", func() tipHover { return tipHover{} })
	if st == nil {
		return e
	}
	if !e.Hovered() {
		st.since, st.armed = time.Time{}, false
		return e
	}
	now := c.Now()
	if st.since.IsZero() {
		st.since = now
	}
	if wait := tipDelay - now.Sub(st.since); wait > 0 {
		wakeAfter(wait)
		return e
	}
	if !st.armed {
		st.armed = true
		c.Invalidate()
	}
	p := baseTokens(c)
	ui.TooltipBase(c, e, func(tip ui.Element) {
		tip.MaxWidth(360).Padding(5, 9).Radius(7).Background(p.surfaceElev).Border(1, p.border).
			Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.18)).
			AttachTo(e, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 6, 0)
		tip.Children(func() {
			t := ui.Text(c, text).FontSize(tipFont).TextColor(p.fg)
			if font != "" {
				t.Font(font)
			}
		})
	})
	return e
}
