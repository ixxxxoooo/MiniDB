package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 动效：设计稿「动效实验室」的三个时长和两条曲线落到 Go 里。
//   - 快（motionFast，悬停 / 按压）、中（motionBase，菜单 / 遮罩淡入）、慢（motionSlow，模态 / 页面 / 抽屉）；
//   - 曲线：默认「快进缓出」用 ui 自带的 EaseOut；弹层、徽标、提示卡用 ui.Snappy 这一档轻微回弹的弹簧。
//
// 系统开启「减弱动态」时 MyGo 会自动把位移、缩放直接跳到终点，只留淡入淡出，这里不用另外判断。

const (
	motionFast = 120 * time.Millisecond
	motionBase = 200 * time.Millisecond
	motionSlow = 360 * time.Millisecond
)

// enterPose 让元素在第一次出现时从 from 姿态过渡到原位（位移 / 缩放 / 淡入）。
// 元素是新建的，MyGo 没有「上一帧」可以比，所以第一帧把它摆在 from，要一帧后才摆回原位。
// 参数：el 要出场的元素（Key 要稳定，换 Key 会重新出场）；from 起始姿态；a 动画曲线。返回 el 本身，方便链式调用。
func enterPose(c *ui.Context, el ui.Element, from ui.Pose, a ui.Animation) ui.Element {
	entered := ui.Local(el, "entered", func() bool { return false })
	p := ui.Pose{}
	if c.Preferences().ReduceMotion { // 系统要求减弱动态：不做出场，也不多画一帧
		*entered = true
		return el
	}
	if !*entered {
		p, *entered = from, true
		c.Invalidate()
	}
	return el.AnimatePose(p, a)
}

// 几种常用的出场姿态，对应设计稿 ui.css 里的 @keyframes。
var (
	poseSlideUp = ui.Pose{Y: 10, Fade: 1}            // slide-up：悬浮条、页面
	posePage    = ui.Pose{Y: 8, Fade: 1}             // 页面切换：自下 8px
	poseSlideR  = ui.Pose{X: 16, Fade: 1}            // slide-r：右侧停靠栏
	posePop     = ui.Pose{Scale: .96, Y: 8, Fade: 1} // pop-c：模态
)

// fadeAnim 是只淡入淡出的动画（减弱动态时仍然生效）。
var fadeAnim = ui.Animation{Duration: motionBase, NonMotion: true}

// 提示卡、弹层用的出场 / 退场：从右侧滑入并轻微缩放，退场时右移淡出（设计稿 toast-in / toast-out）。
var toastMotion = ui.ElementTransition{
	Animation:    &ui.Animation{Spring: ui.Snappy.Spring},
	Enter:        &ui.Motion{X: 24, Scale: .96, Opacity: 0},
	Exit:         &ui.Motion{X: 24, Opacity: 0},
	ExitDuration: motionBase,
}

// blurLayer 画模态背后的「模糊 + 暗化」：先把窗口里已画的内容模糊，再盖一层遮罩色；
// 两层都是不接收指针的绝对定位盒子，放在比模态低一层的浮层里，点击仍由模态自己的背板处理。
// id 区分同时开着的几个模态（各画一层，叠起来更暗）。遮罩随时间淡入（motionBase），模糊半径同步长大，减弱动态时直接到位。
func blurLayer(c *ui.Context, p tokens, id any) {
	ui.Overlay(c, func() {
		back := ui.Box(c.Key(fmt.Sprintf("scrim-blur-%p", id))).Absolute().Left(0).Top(0).Right(0).Bottom(0)
		t := back.AnimateTo("in", 1, fadeAnim)
		back.Material(glass.Blur{Radius: sizeScrimBlur * t})
		ui.Box(c.Key(fmt.Sprintf("scrim-dim-%p", id))).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(p.scrim.Alpha(t))
	})
}

// pressFeel 给按钮加上设计稿的悬停 / 按压反馈：颜色在 motionFast 内渐变，按住时缩到 .96。
// 只改背景色和 transform，不改布局。参数 b 为刚创建的按钮。
func pressFeel(b ui.Element) {
	b.Transition(ui.ElementTransition{Duration: motionFast, Colors: true})
	if b.Pressed() {
		b.Scale(.96, .96)
	}
}
