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
//   - 曲线：默认「快进缓出」用 ui 自带的 EaseOut；弹层、徽标、提示卡用 popAnim 这一档 150ms 的缓出。
//
// 系统开启「减弱动态」或设置里开了「减弱动画效果」时，位移、缩放直接跳到终点（后者靠 theme.go 每帧 DisableAnimations）。

const (
	motionFast = 100 * time.Millisecond
	motionBase = 160 * time.Millisecond
	motionSlow = 260 * time.Millisecond
)

// popAnim 是弹层、模态、命令面板、悬浮条、提示卡入场用的曲线：150ms 缓出。
// 不用弹簧：框架的 ui.Snappy 标称 500ms，收短到 180ms 后实测尾巴仍拖到 320ms 才停，用户反馈弹出太慢；
// 固定时长的缓出到点就停，快而且可预期。
var popAnim = ui.Animation{Duration: 150 * time.Millisecond}

// enterPose 让元素在第一次出现时从 from 姿态过渡到原位（位移 / 缩放 / 淡入）。
// 元素是新建的，MyGo 没有「上一帧」可以比，所以第一帧把它摆在 from，要一帧后才摆回原位。
// 参数：el 要出场的元素（Key 要稳定，换 Key 会重新出场）；from 起始姿态；a 动画曲线。返回 el 本身，方便链式调用。
func enterPose(c *ui.Context, el ui.Element, from ui.Pose, a ui.Animation) ui.Element {
	built := ui.Local(el, "entered", func() int { return 0 })
	p := ui.Pose{}
	if c.Preferences().ReduceMotion || c.Instant() { // 系统或设置要求减弱动态：不做出场，也不多画一帧
		*built = enterHold
		return el
	}
	// 起始姿势多保持几次构建：点击触发的重建里帧号会跳，「上一帧没摆过姿势就直接到位」会把动画吞掉；
	// 多摆几次，放开那一刻引擎一定认得「刚才摆过」，才会真的做过渡。
	if *built < enterHold {
		p = from
		*built++
		c.Invalidate()
	}
	return el.AnimatePose(p, a)
}

// enterHold 是起始姿势保持的构建次数。
const enterHold = 2

// 几种常用的出场姿态，对应设计稿 ui.css 里的 @keyframes。
var (
	poseSlideUp = ui.Pose{Y: 10, Fade: 1}            // slide-up：悬浮条、页面
	posePage    = ui.Pose{Y: 8, Fade: 1}             // 页面切换：自下 8px
	posePop     = ui.Pose{Scale: .96, Y: 8, Fade: 1} // pop-c：模态
)

// fadeAnim 是只淡入淡出的动画（减弱动态时仍然生效）。
var fadeAnim = ui.Animation{Duration: motionBase, NonMotion: true}

// 提示卡、弹层用的出场 / 退场：从右侧滑入并轻微缩放，退场时右移淡出（设计稿 toast-in / toast-out）。
var toastMotion = ui.ElementTransition{
	Animation:    &popAnim,
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

// rowHover 让列表行、菜单行的悬停 / 选中底色在 motionFast 内渐变，而不是一帧跳过去（设计稿：悬停底色过渡）。
var rowHover = ui.ElementTransition{Duration: motionFast, Colors: true}

// popExit 是锚定浮层（连接 / 库 / 标签列表、筛选）的出场：淡出 80ms（设计稿动效规范）。
var popExit = ui.ElementTransition{Exit: &ui.Motion{Opacity: 0}, ExitDuration: 80 * time.Millisecond}

// chipMotion 是筛选芯片的增删：新增时从 .9 倍缩放淡入（160ms），移除时淡出并缩小（120ms），设计稿动效规范「筛选芯片增删」。
var chipMotion = ui.ElementTransition{
	Animation:    &ui.Animation{Duration: 160 * time.Millisecond},
	Enter:        &ui.Motion{Scale: .9, Opacity: 0},
	Exit:         &ui.Motion{Scale: .9, Opacity: 0},
	ExitDuration: 120 * time.Millisecond,
	Colors:       true,
}

// tabMotion 是标签条里一个标签的增删与状态过渡：新开的标签从 .92 倍缩放淡入（160ms），关掉的淡出（120ms）；
// 选中、悬停造成的底色 / 描边色变化在 motionFast 内渐变，而不是一帧跳过去（设计稿动效补全「标签新增 / 关闭、悬停底色过渡」）。
var tabMotion = ui.ElementTransition{
	Duration:     motionFast,
	Animation:    &ui.Animation{Duration: 160 * time.Millisecond},
	Enter:        &ui.Motion{Scale: .92, Opacity: 0},
	Exit:         &ui.Motion{Scale: .92, Opacity: 0},
	ExitDuration: 120 * time.Millisecond,
	Colors:       true,
}

// cellFlash 让被改动的单元格底色（待提交的黄色、新增的绿色、删除的红色）在 400ms 内淡入，
// 而不是一帧跳出来，用户一眼能看到是哪一格变了（设计稿动效补全「单元格修改闪烁」）。
var cellFlash = ui.ElementTransition{Duration: 400 * time.Millisecond, Colors: true}
