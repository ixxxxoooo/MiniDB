package ui

import (
	"sync/atomic"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 选中指示块的滑动（设计稿第六轮「选中指示滑动」）：
// 分段开关、视图切换、下划线标签、活动栏的「选中」不再是旧的一段灰掉、新的一段亮起，
// 而是同一块指示块从旧位置滑（位置和宽高一起过渡）到新位置。
//
// 做法：指示块是选中那一项按钮里的一个绝对定位的子元素，全都带同一个 Match 标识；
// 切换选中项时，旧按钮里的指示块不再构建，新按钮里的指示块「出现」，
// MyGo 会让它从上一帧同一标识的指示块所在的位置和大小过渡过来。
// 设计稿里用 FLIP 做同样的事（scripts/motion6.js）。

// thumbMotion 是指示块滑动的时长：和设计稿一致 200ms（motionBase）。
const thumbMotion = motionBase

// thumbMatch 是指示块的共享标识：同一个控件实例里的所有指示块用同一个，不同控件互不相干。
type thumbMatch struct{ id any }

// thumbSeq 给每个控件实例发编号。
var thumbSeq atomic.Uint64

// thumbScope 返回控件 el（分段开关的外层行等）自己的指示块标识。
// 编号存在元素的本地状态里：控件跨帧还在，编号就不变，选中项切换时指示块能接上；
// 换标签页之类让控件整个重建时会换新编号，新页面的指示块不会从别的页面「飞」过来。
func thumbScope(el ui.Element) thumbMatch {
	return thumbMatch{*ui.Local(el, "thumb-id", func() uint64 { return thumbSeq.Add(1) })}
}

// slideThumb 在选中的按钮 host 里放一块撑满按钮的指示块，调用方再给它设背景、圆角、描边、阴影。
// 必须在按钮的 Children 里调用。
//
// 层叠：指示块滑过相邻按钮时不能盖住它们的文字。绝对定位的元素默认画在兄弟之上，所以做两件事：
// 指示块用负的 ZIndex 压到自己按钮的文字下面；选中的按钮 host 整个用负的 ZIndex 压到相邻按钮下面
// （其余按钮背景都是透明的，所以不会看到它被「遮住」，只是文字画在指示块上面）。
//
// 参数：m 控件的指示块标识（同一个控件里的各项传同一个）；
// bleed 是向四周多铺的像素，按钮自己有透明描边时填描边宽度，指示块才会盖到按钮的外缘。
// 返回指示块元素。
func slideThumb(c *ui.Context, host ui.Element, m thumbMatch, bleed float32) ui.Element {
	host.ZIndex(-1)
	return ui.Box(c.Key("thumb")).Absolute().Left(-bleed).Top(-bleed).Right(-bleed).Bottom(-bleed).ZIndex(-1).
		Transition(thumbMove(m))
}

// thumbMove 是指示块的过渡：从同一标识的上一块出现的位置和大小滑过来，时长 thumbMotion。
func thumbMove(m thumbMatch) ui.ElementTransition {
	return ui.ElementTransition{Match: m, Animation: &ui.Animation{Duration: thumbMotion}}
}
