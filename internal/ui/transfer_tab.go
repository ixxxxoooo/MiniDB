package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 导出 / 导入向导页（设计稿「导出」「导入」页）：左边是四步的步骤条，右边是当前这一步，底部是上一步 / 下一步。
//   - 导出：选择对象 → 格式与选项 → 目标位置 → 执行（transfer_export.go）；
//   - 导入：选择文件 → 目标表 → 字段映射 → 导入（transfer_import.go）。
//
// 表单状态仍放在 Shell 的 exp / imp 里（导出查询结果的对话框与它共用 exportDialog），
// 所以同一时刻只有一份向导的数据；在另一个连接里再开向导时从头来过。

// 向导的两种模式。
const (
	xferExport = iota
	xferImport
)

// transferTab 是一个导出或导入向导标签；每个工作区每种模式最多一个。
type transferTab struct {
	w        *Workspace
	mode     int  // xferExport / xferImport
	step     int  // 当前是第几步（0 起）
	showCols bool // 导出第一步：展开列挑选
	waiting  bool // 导入：已点「开始」，等任务启动后跳到执行步
}

func (t *transferTab) title() string {
	if t.mode == xferImport {
		return locales.XferImport()
	}
	return locales.XferExport()
}

func (t *transferTab) icon() *ui.SVG {
	if t.mode == xferImport {
		return icUpload
	}
	return icDownload
}

func (t *transferTab) objKind() ObjectKind { return KindTable }
func (t *transferTab) pending() int        { return 0 }

// openTransfer 打开指定模式的向导标签，已经开着就切过去。
func (w *Workspace) openTransfer(mode int) *transferTab {
	for i, tb := range w.tabs {
		if x, ok := tb.(*transferTab); ok && x.mode == mode {
			w.active = i
			return x
		}
	}
	t := &transferTab{w: w, mode: mode}
	w.tabs = append(w.tabs, t)
	w.active = len(w.tabs) - 1
	return t
}

// restart 让向导从第一步重新开始（从侧栏、右键菜单再次进入向导时用，表单数据已由调用方换成新的）。
func (t *transferTab) restart() {
	t.step, t.showCols, t.waiting = 0, false, false
}

// prepare 在每帧开头保证表单状态属于当前工作区，并处理「任务启动后跳到执行步」。
func (t *transferTab) prepare() {
	s := t.w.sh
	if t.mode == xferExport {
		if s.exp.w != t.w || (!s.exp.open && s.exp.query != "") { // 别的连接的数据，或上次导出查询结果留下的
			pf := s.prefs.Export
			s.exp = exportDialog{w: t.w, sel: map[string]bool{}, prefs: pf, format: pf.Format}
		}
		return
	}
	if s.imp.w != t.w {
		s.imp = newImportDialog(t.w, "")
	}
	d := &s.imp
	switch {
	case t.waiting && d.xfer.active:
		t.step, t.waiting = 3, false
	case t.waiting && d.err != "" && !d.busy:
		t.waiting = false
	}
}

// view 画向导页。
func (t *transferTab) view(c *ui.Context, p tokens) {
	t.prepare()
	names := []string{locales.XferStepEx1(), locales.XferStepEx2(), locales.XferStepEx3(), locales.XferStepEx4()}
	if t.mode == xferImport {
		names = []string{locales.XferStepIm1(), locales.XferStepIm2(), locales.XferStepIm3(), locales.XferStepIm4()}
	}
	ui.Column(c.Key(fmt.Sprint("xfer", t.mode))).Fill().MinWidth(0).Children(func() {
		t.toolbar(c, p)
		ui.Row(c.Key("xf-main")).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			t.stepsColumn(c, p, names)
			hairlineV(c, p.border)
			ui.Column(c.Key("xf-right")).Grow(1).MinWidth(0).FillHeight().Children(func() {
				scrollbars(c, ui.Scroll(c.Key("xf-body")).Grow(1).FillWidth()).Children(func() {
					body := ui.Column(c.Key(fmt.Sprint("xf-step", t.step))).Gap(spaceLG).Padding(spaceLG, spaceXL-2).FillWidth()
					enterPose(c, body, posePage, ui.Animation{Duration: motionSlow}) // 设计稿 page-in：换一步时自下滑入
					body.Children(func() {
						if t.mode == xferImport {
							t.importBody(c, p)
						} else {
							t.exportBody(c, p)
						}
					})
				})
				hairline(c, p.border)
				ui.Row(c.Key("xf-foot")).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceSM+2, spaceLG).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
					if t.mode == xferImport {
						t.importFoot(c, p)
					} else {
						t.exportFoot(c, p)
					}
				})
			})
		})
	})
}

// toolbar 是页面顶部的条：左边「导出 | 导入」切换，右边一句提醒。
func (t *transferTab) toolbar(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(spaceSM).Padding(0, spaceSM+2).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Children(func() {
			sel := t.mode
			if segmented(c, &sel, locales.XferExport(), locales.XferImport()) {
				t.w.openTransfer(sel)
			}
			ui.Spacer(c)
			if t.mode == xferImport {
				chip(c, chipWarn, locales.XferWrites(), false)
			} else {
				ui.Text(c, locales.XferNoWrite()).FontSize(fontCaption).TextColor(p.fgMuted)
			}
		})
	}).Padding(0)
	hairline(c, p.border)
}

// stepsColumn 画左侧的步骤条：当前步强调色圆点，走过的步打勾；点一步就跳过去（执行步要先开始才能进）。
func (t *transferTab) stepsColumn(c *ui.Context, p tokens, names []string) {
	started := t.w.sh.exp.xfer.active
	if t.mode == xferImport {
		started = t.w.sh.imp.xfer.active
	}
	ui.Column(c.Key("xf-steps")).Width(sizeWizSteps).FillHeight().Shrink(0).Padding(spaceLG, spaceMD+2).Gap(spaceSM).Background(p.surfaceSecondary).Children(func() {
		for i, name := range names {
			row := ui.Row(c.Key(fmt.Sprint("step", i))).Gap(spaceSM).AlignItems(ui.Center)
			label, weight, bg := p.fgMuted, 500, p.surfaceTertiary
			switch {
			case i == t.step:
				label, weight, bg = p.fg, 600, p.accent
			case i < t.step:
				bg = p.successBg
			}
			row.Children(func() {
				dot := ui.Row(c.Key("dot")).Size(sizeStepDot, sizeStepDot).Shrink(0).Center().Radius(sizeStepDot / 2).Background(bg)
				if i == t.step {
					dot.Shadow(0, 0, 0, spaceXS, p.accentSoft) // 设计稿：当前步外面一圈淡光环
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
				ui.Text(c, name).FontSize(fontBody).FontWeight(weight).TextColor(label).SingleLine()
			})
			if row.Clicked() && (i < 3 || started) {
				t.step = i
			}
		}
	})
}

// navButtons 画页脚右侧的「上一步 / 下一步」：next 为主按钮文字，off 为真时灰着，onNext 是点下一步的动作。
func (t *transferTab) navButtons(c *ui.Context, back bool, next string, off bool, onNext func()) {
	if back && fieldBtn(c.Key("xf-prev"), nil, locales.XferPrev()).Clicked() {
		t.step--
	}
	if fieldPrimary(c.Key("xf-next"), nil, next).Disabled(off).Clicked() {
		onNext()
	}
}

// ---- 向导里反复用到的小部件 ----

// xferCard 是向导里的白底圆角卡片（清单、预览表都放在里面）。
func xferCard(c *ui.Context, p tokens) ui.Element {
	return ui.Column(c).FillWidth().Radius(radiusPanel).Border(1, p.border).Background(p.surface).Clip()
}

// xferSection 是带小标题的一节：标题在上，内容 fn 在下。
func xferSection(c *ui.Context, p tokens, title string, fn func()) {
	ui.Column(c).Gap(spaceSM).FillWidth().Children(func() {
		ui.Text(c, title).FontSize(fontBody).FontWeight(600)
		fn()
	})
}

// switchRow 画一行「说明 …… 开关」，返回开关是否被拨动。key 在同一父元素里要唯一。
func switchRow(c *ui.Context, p tokens, key, label string, v *bool) bool {
	changed := false
	ui.Row(c.Key(key)).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine()
		ui.Spacer(c)
		changed = ui.Switch(c.Key(key+"-sw"), v).Changed()
	})
	return changed
}

// switchNot 同 switchRow，但开关的「开」对应 *v 为假（比如「写入表头」对应 NoHeader）。
func switchNot(c *ui.Context, p tokens, key, label string, v *bool) {
	on := !*v
	if switchRow(c, p, key, label, &on) {
		*v = !on
	}
}
