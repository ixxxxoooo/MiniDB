package ui

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// 悬浮提示的统一规范：
//   - 提示在元素正下方居中、离它 6px，放不下自动翻到上方（设计稿「悬浮提示」）；
//   - 字号固定为 tipFont（fontCaption），不随所在区域的主题缩放（数据表用的是紧凑主题，
//     框架自带的 Tooltip 会按主题字号减 1，结果各处大小不一）；
//   - 指针在元素上停留满 tipDelay 才显示，不会一碰就弹出。框架自带的"刚看过一个提示，
//     下一个立即出现"的热身逻辑也会让提示立即出现，所以这里自己计时，
//     只有停留够久了才把元素交给框架去显示（此时框架的延迟为 0，见 applyLook）；
//   - 外观按设计稿：反色小气泡（主文字色做底、内容区底色做字），圆角 8、内边距 5×9、11.5px 中等字重，2 级阴影；
//     文字末尾的快捷键拆成半透明键帽；带换行的提示第一行是标题（半粗），后面是说明（淡一点）；出现时淡入 + 缩放 .96 + 上移 2px。
const (
	tipFont  = fontCaption
	tipDelay = 600 * time.Millisecond
	tipGap   = 6 // 提示离触发元素的距离
)

// tipHover 记录指针在某个元素上从什么时候开始停留。
type tipHover struct {
	since time.Time
	armed bool // 已经交给框架显示过了；第一次交出去之后要再排一帧，框架下一帧才认得它
	// waking 为真表示已经有一个"到点重绘"的定时器在等了，避免每帧都起一个 goroutine。
	waking atomic.Bool
}

// wake 在 d 之后触发一次重绘。指针停着不动时界面不会自己刷新，
// 没有这一下，停留时间到了提示也不会出现。同一个元素同时只排一个定时器。
func (h *tipHover) wake(c *ui.Context, d time.Duration) {
	if !h.waking.CompareAndSwap(false, true) {
		return
	}
	svc := c.Services() // 持久的窗口服务，Invalidate 可以在任意 goroutine 里调用
	go func() {
		time.Sleep(d + 10*time.Millisecond)
		h.waking.Store(false)
		svc.Invalidate()
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
		st.wake(c, wait)
		return e
	}
	if !st.armed {
		st.armed = true
		c.Invalidate()
	}
	p := baseTokens(c)
	ui.TooltipBase(c, e, func(tip ui.Element) {
		tip.MaxWidth(sizeTipMax).Padding(5, 9).Radius(radiusMenu-1).Background(p.fg.Alpha(0.94)).
			Shadow(0, shadowY2, shadowBlur2, 0, p.shadow).
			AttachTo(e, ui.AnchorBottom, ui.AnchorTop).Margin(tipGap, 0, 0, 0)
		enterPose(c, tip, poseTip, ui.Animation{Duration: motionFast})
		tip.Children(func() { tipContent(c, p, text, font) })
	})
	return e
}

// poseTip 是提示出现时的姿态：略小、略靠下、透明，然后淡入到位（设计稿：缩放 .96、上移 2px）。
var poseTip = ui.Pose{Y: 2, Scale: .96, Fade: 1}

// tipContent 画提示里的内容：第一行（标题或正文）加快捷键键帽，有换行时后面几行是淡一点的说明。
// 参数：text 是提示文字；font 为空用界面字体。
func tipContent(c *ui.Context, p tokens, text, font string) {
	title, desc, _ := strings.Cut(text, "\n")
	label, keys := splitTipKeys(title)
	ui.Column(c).Gap(2).Children(func() {
		ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Children(func() {
			t := ui.Text(c, label).FontSize(tipFont).FontWeight(500).TextColor(p.surface)
			if desc != "" {
				t.FontWeight(650)
			}
			if font != "" {
				t.Font(font)
			}
			if len(keys) > 0 {
				ui.Row(c).Gap(3).Children(func() {
					for _, k := range keys {
						ui.Row(c).Height(sizeKbd-1).MinWidth(sizeKbd-1).Padding(0, 4).Radius(radiusSm).Background(p.surface.Alpha(0.2)).Center().Children(func() {
							ui.Text(c, k).FontSize(fontMicro).FontWeight(600).TextColor(p.surface).SingleLine()
						})
					}
				})
			}
		})
		if desc != "" {
			ui.Text(c, desc).FontSize(fontCaption).TextColor(p.surface.Alpha(0.72))
		}
	})
}
