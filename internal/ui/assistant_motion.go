package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 对话流里的动效。原则：动效只表达状态——有东西在进行就动，到了就停，不为好看而动；
// 全部由状态驱动（正在思考 / 工具运行中 / 展开 / 刚出现），没有任何定时器；
// 桌面要求减少动态效果时，位移类动效由 mygo 自动改成淡入淡出或直接出现。

// 动效的时长与曲线。
const (
	motionEnter  = 220 * time.Millisecond // 一条消息、一张卡片出现
	motionToggle = 180 * time.Millisecond // 展开 / 收起一块内容
	motionPulse  = 1500 * time.Millisecond
)

// enterTransition 是新消息、结果卡出现的样子：从下方 8 个点淡入上来。
var enterTransition = ui.ElementTransition{
	Duration: motionEnter,
	Enter:    &ui.Motion{Y: 8, Opacity: 0},
}

// toggleTransition 是展开 / 收起一块内容：高度从零长出来（收起时缩回去），同时淡入淡出。
var toggleTransition = ui.ElementTransition{
	Duration:     motionToggle,
	Enter:        &ui.Motion{Collapse: true, Opacity: 0},
	Exit:         &ui.Motion{Collapse: true, Opacity: 0},
	ExitDuration: 140 * time.Millisecond,
}

// popTransition 是状态图标（完成的对勾、失败的叉）出现的样子：从小一点放大并淡入。
var popTransition = ui.ElementTransition{
	Duration: 200 * time.Millisecond,
	Enter:    &ui.Motion{Scale: 0.6, Opacity: 0},
}

// stepTransition 是时间线上新的一步出现：从下方 4 个点轻轻淡入，比整条消息的入场更收敛。
var stepTransition = ui.ElementTransition{
	Duration: 180 * time.Millisecond,
	Enter:    &ui.Motion{Y: 4, Opacity: 0},
}

// staggerTransition 是一排建议依次出现：第 i 个比前一个晚 40 ms。
func staggerTransition(i int) ui.ElementTransition {
	return ui.ElementTransition{
		Animation: &ui.Animation{Duration: motionEnter, Delay: time.Duration(i) * 40 * time.Millisecond},
		Enter:     &ui.Motion{Y: 6, Opacity: 0},
	}
}

// pulseValue 是 0 到 1 来回的脉动值，周期 motionPulse；用在"正在进行"的东西上让它呼吸。
// e 是要脉动的元素（脉动值存在它的状态里），key 区分同一个元素上的多个脉动。
func pulseValue(e ui.Element, key string) float32 {
	return e.Loop(key, motionPulse, ui.Bounce(ui.EaseInOut))
}

// breathe 让一个元素在 live 时透明度在 0.45～1 之间呼吸；不在进行时保持不透明。
func breathe(e ui.Element, key string, live bool) ui.Element {
	if !live {
		return e
	}
	return e.Opacity(0.45 + 0.55*pulseValue(e, key))
}

// typingDots 是"在等模型"的三个点：依次亮起，一个接一个。
func typingDots(c *ui.Context, p tokens) {
	ui.Row(c.Key("dots")).Gap(3).AlignItems(ui.Center).Height(14).Children(func() {
		for i := 0; i < 3; i++ {
			dot := ui.Box(c.Key(string(rune('a'+i)))).Size(4, 4).Radius(2).Background(p.fgSecondary).Shrink(0)
			// 三个点共用一个周期，各自错开三分之一：亮的那个依次右移
			dot.Opacity(dotOpacity(dot.Loop("d", 1200*time.Millisecond, ui.Linear), i))
		}
	})
}

// dotOpacity 是第 i 个点在周期位置 phase（0～1，匀速）的不透明度：
// 轮到自己的那三分之一周期里先最亮再渐暗，其余时间暗淡。
func dotOpacity(phase float32, i int) float32 {
	at := phase - float32(i)/3
	if at < 0 {
		at += 1
	}
	if at > 1.0/3 {
		return 0.25
	}
	return 0.25 + 0.75*(1-at*3)
}

// streamCursor 是回答还在流出来时末尾的一个小圆点：呼吸，回答结束就不画。
func streamCursor(c *ui.Context, p tokens) {
	dot := ui.Box(c.Key("cursor")).Size(7, 7).Radius(4).Background(p.accent).Margin(2, 0, 0, 2)
	dot.Opacity(0.3 + 0.7*pulseValue(dot, "cur"))
}
