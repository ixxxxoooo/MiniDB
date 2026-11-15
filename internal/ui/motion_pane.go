package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 子视图切换的内容淡入（设计稿第六轮「子视图切换」）：
// 同一个页面里切换数据 / 结构 / DDL / 文档、监控与用户的分页、设置分页、侧栏三页之类的并列视图时，
// 新内容自下 6px 淡入 160ms，旧内容立即换掉。
//
// 做法：每个视图的内容各自一个元素，Key 里带上视图标识，切换时旧的消失、新的「出现」；
// 出现时带 Enter 过渡。MyGo 的 Enter 只在「元素出现在上一帧已存在的父元素里」时才播，所以：
//   - 切换子视图会播；
//   - 打开 / 切换标签时，整页是新的父元素，不会再叠一层子视图淡入，只有 tabArea 那一次页面入场。
//
// 不用 AnimatePose（enterPose）：它在「上一帧没摆过姿势」时直接到位，点击触发的重建里帧号会跳，动画就丢了。

// paneMotion 是子视图新内容的入场：自下 6px、全透明，160ms（设计稿 view-up）。
// Colors 必须设：ElementTransition 里 Position / Size / Colors / Shadows 一个都不设就是「全部都动画」，
// 内容区的位置或尺寸一变（开合侧栏、开关行预览、改窗口大小）整块内容，包括里面的行预览，就会跟着滑动、第一帧还会跳。
// 这里只要入场淡入，不要布局动画，所以显式只选 Colors（面板底色随主题渐变，无害）。
var paneMotion = ui.ElementTransition{
	Animation: &ui.Animation{Duration: motionBase},
	Enter:     &ui.Motion{Y: 6, Opacity: 0},
	Colors:    true,
}

// paneIn 给一个子视图的内容元素 el 加上切换淡入，返回 el 方便链式调用。
// el 的 Key 必须带上视图标识，换视图时它才是「新出现」的元素。
func paneIn(el ui.Element) ui.Element { return el.Transition(paneMotion) }
