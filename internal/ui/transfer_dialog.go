package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 导出 / 导入向导的弹窗外壳（设计稿：统一向导弹窗，备份、恢复同一套骨架）：
// 头部 = 图标块 + 标题 + 一句话 + 关闭；下面是横向步骤条（走过的打勾、当前高亮、点一步就跳过去）；
// 中间是当前这一步；底部左边一句概括、右边「上一步 / 下一步」。各步内容在 transfer_export.go / transfer_import.go。

// xferDialog 是向导弹窗的状态：open 开着，t 是向导的数据（表单数据仍在 Shell 的 exp / imp 里）。
type xferDialog struct {
	open bool
	t    *transferTab
}

// xferView 画向导弹窗。
func (s *Shell) xferView(c *ui.Context) {
	d := &s.xf
	if !d.open || d.t == nil {
		return
	}
	t := d.t
	p := s.tokens(c)
	t.prepare()
	names := []string{locales.XferStepEx1(), locales.XferStepEx2(), locales.XferStepEx3(), locales.XferStepEx4()}
	sub := locales.XferNoWrite()
	if t.mode == xferImport {
		names = []string{locales.XferStepIm1(), locales.XferStepIm2(), locales.XferStepIm3(), locales.XferStepIm4()}
		sub = locales.XferWrites()
	}
	s.dialog(c, &d.open, dlgOpts{title: t.title(), sub: t.w.dbLabel() + " · " + sub, width: sizeWiz, height: sizeWizH, lead: dlgTile(c, p, t.icon())}, func() {
		t.stepsRow(c, p, names)
		scrollbars(c, ui.Scroll(c.Key("xf-body")).Grow(1).FillWidth()).Children(func() {
			body := ui.Column(c.Key(fmt.Sprint("xf-step", t.step))).Gap(spaceLG).Padding(spaceMD, spaceXS).FillWidth()
			enterPose(c, body, posePage, ui.Animation{Duration: motionSlow}) // 设计稿 page-in：换一步时自下滑入
			body.Children(func() {
				if t.mode == xferImport {
					t.importBody(c, p)
				} else {
					t.exportBody(c, p)
				}
			})
		})
	}, func() {
		if t.mode == xferImport {
			t.importFoot(c, p)
		} else {
			t.exportFoot(c, p)
		}
	})
}

// stepsRow 画横向步骤条：当前步强调色圆点（外面一圈淡光环），走过的步打勾，步与步之间一条细线（走过的是绿线）。
// 点一步就跳过去（执行步要先开始才能进）。
func (t *transferTab) stepsRow(c *ui.Context, p tokens, names []string) {
	started := t.w.sh.exp.xfer.active
	if t.mode == xferImport {
		started = t.w.sh.imp.xfer.active
	}
	ui.Row(c.Key("xf-steps")).Gap(spaceMD).AlignItems(ui.Center).FillWidth().Shrink(0).Padding(spaceXS, spaceSM, spaceXS, spaceSM).Children(func() {
		for i, name := range names {
			i, name := i, name
			row := ui.Row(c.Key(fmt.Sprint("step", i))).Gap(spaceSM).AlignItems(ui.Center).Shrink(0)
			label, weight, bg := p.fgMuted, 500, p.surfaceTertiary
			switch {
			case i == t.step:
				label, weight, bg = p.fg, 650, p.accent
			case i < t.step:
				label, bg = p.fgSecondary, p.successBg
			}
			row.Children(func() {
				dot := ui.Row(c.Key("dot")).Size(sizeStepDot, sizeStepDot).Shrink(0).Center().Radius(sizeStepDot / 2).Background(bg)
				if i == t.step {
					dot.Shadow(0, 0, 0, spaceXS, p.accentSoft)
				}
				dot.Children(func() {
					switch {
					case i < t.step:
						ui.Icon(c, icCheck).FontSize(fontCaption).TextColor(p.success)
					case i == t.step:
						ui.Text(c, fmt.Sprint(i+1)).FontSize(fontMicro).FontWeight(700).TextColor(p.accentFg)
					default:
						ui.Text(c, fmt.Sprint(i+1)).FontSize(fontMicro).FontWeight(700).TextColor(p.fgMuted)
					}
				})
				ui.Text(c, name).FontSize(fontCtl).FontWeight(weight).TextColor(label).SingleLine()
			})
			if row.Clicked() && (i < len(names)-1 || started) {
				t.step = i
			}
			if i < len(names)-1 {
				line := p.surfaceTertiary
				if i < t.step {
					line = p.success.Alpha(0.5)
				}
				ui.Box(c.Key(fmt.Sprint("line", i))).Grow(1).MinWidth(spaceLG).Height(loadBarH).Radius(loadBarH / 2).Background(line)
			}
		}
	})
}
