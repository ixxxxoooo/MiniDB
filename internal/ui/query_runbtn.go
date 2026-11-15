package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 查询工具栏最左边的「执行 / 停止」键。
//
// 执行时整条工具栏不能抖：键留在原地、宽度不变，只是内容换成「转圈 + 停止」、底色渐变成红色，
// 右边也不再另外显示「执行中…」。很快就跑完的语句（不到 runShowDelay）根本不变样，免得闪一下红。

// runShowDelay 是执行多久之后才把执行键显示成「停止」；更短的语句不值得变样。
const runShowDelay = 150 * time.Millisecond

// runBtnMotion 让执行键的底色在 motionFast 内渐变（强调色 ↔ 红色），只动颜色，不动布局。
var runBtnMotion = ui.ElementTransition{Duration: motionFast, Colors: true}

// runButton 画执行 / 停止键。参数 hint 是执行的快捷键提示（空则不画键帽）。
// 返回 clicked：点了「执行」要去跑，点了「停止」要去取消；还没变样的前 runShowDelay 里点击被忽略。
func (q *queryTab) runButton(c *ui.Context, p tokens, hint string) (run, stop bool) {
	showStop := q.running && delayed(c, q.started, runShowDelay)
	label := locales.QueryRun()
	if showStop {
		label = locales.QueryStop()
	}
	b := ui.PrimaryButton(c.Key("run-btn"), "").Label(label).Transition(runBtnMotion)
	if showStop {
		skinBtn(b, p, btnDanger)
	}
	if !showStop {
		tipOn(c, b, locales.QueryRunCurrent()+q.w.sh.keyHint("run"))
	}
	// 记下「执行」状态下的宽度，变成「停止」时撑到同样宽，旁边的按钮才不会被挤动
	w := ui.Local(b, "idle-w", func() float32 { return 0 })
	if !q.running {
		*w = max(*w, b.LayoutBounds().W)
	}
	b.Height(btnActH).MinWidth(*w).Padding(0, btnActPad).Gap(5).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if showStop {
			ui.Spinner(c).Size(btnActIcon, btnActIcon).TextColor(p.accentFg)
		} else {
			ui.Icon(c, icPlay).FontSize(btnActIcon)
		}
		ui.Text(c, label).FontSize(btnActFont).FixedLineHeight(16).SingleLine()
		if hint != "" && !showStop {
			keycapOn(c, hint, true).Margin(0, 0, 0, 2)
		}
	})
	if b.Clicked() {
		switch {
		case showStop:
			stop = true
		case !q.running:
			run = true
		}
	}
	return run, stop
}
