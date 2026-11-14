package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// confirmDialog asks before something destructive.
// 确认框的基调：决定图标和确认按钮的颜色。
const (
	toneInfo   = iota // 普通确认：强调色
	toneWarn          // 会丢掉没保存的东西：橙色
	toneDanger        // 不可恢复的破坏性操作：红色
)

type confirmDialog struct {
	open    bool
	title   string
	message string
	action  func()
	label   string
	tone    int
	opts    []*confirmOpt // 说明下面的复选项（比如清空、删除表时的 Cascade），空则没有
}

// confirmOpt 是确认框里的一个复选项；点确认时 on 就是用户的选择。
type confirmOpt struct {
	label string
	on    bool
}

// askDangerOpts 同 askDanger，另在说明下面列出复选项 opts，action 里读 opts 的 on。
func (s *Shell) askDangerOpts(title, message, label string, opts []*confirmOpt, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, action: action, label: label, tone: toneDanger, opts: opts}
}

func (s *Shell) askWarn(title, message, label string, action func()) {
	s.askTone(toneWarn, title, message, label, action)
}

func (s *Shell) askDanger(title, message, label string, action func()) {
	s.askTone(toneDanger, title, message, label, action)
}

func (s *Shell) askTone(tone int, title, message, label string, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, action: action, label: label, tone: tone}
}

// discardGuard runs fn at once when nothing would be lost, else after the user
// agrees to discard n uncommitted changes.
func (s *Shell) discardGuard(n int, fn func()) {
	if n == 0 {
		fn()
		return
	}
	s.askWarn(locales.EditDiscardTitle(), locales.EditDiscardMsg(n), locales.EditDiscard(), fn)
}

// confirmView 是应用统一的确认框：左上一枚按基调配色的图标，标题与说明，
// 右下取消和确认两个按钮。破坏性操作的确认按钮是红色，默认焦点在"取消"上，
// 回车不会直接删东西；其余情况默认焦点在确认键，回车即确认；Esc 取消。
func (s *Shell) confirmView(c *ui.Context) {
	d := &s.confirm
	if !d.open {
		return
	}
	p := s.tokens(c)
	col, icon := p.accent, icQuestion
	switch d.tone {
	case toneWarn:
		col, icon = p.warning, icAlertTri
	case toneDanger:
		col, icon = p.danger, icTrash
	}
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	confirmed, cancelled := false, false
	ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(400).Children(func() {
			ui.Row(c).Gap(14).AlignItems(ui.Start).Padding(20, 22, 18, 20).FillWidth().Children(func() {
				ui.Row(c).Size(40, 40).Shrink(0).Center().Radius(20).Background(col.Alpha(0.13)).Children(func() {
					ui.Icon(c, icon).FontSize(19).TextColor(col)
				})
				ui.Column(c).Gap(6).Grow(1).MinWidth(0).Padding(2, 0, 0, 0).Children(func() {
					ui.Text(c, d.title).FontWeight(600).FontSize(15)
					ui.Text(c, d.message).FontSize(fontXS).TextColor(p.fgSecondary).Selectable()
					for i, o := range d.opts {
						ui.Checkbox(c.Key("confirm-opt"+itoa(i)), &o.on, o.label).Margin(4, 0, 0, 0)
					}
				})
			})
			hairline(c, p.borderSubtle)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Padding(10, 14).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
				// 默认焦点：破坏性操作落在"取消"上（回车是安全的），其余落在确认键上
				cancel := footBtn(c.Key("confirm-cancel"), locales.CommonCancel())
				ok := footBtn(c.Key("confirm-ok"), d.label).FontWeight(600).TextColor(ui.Hex("#ffffff")).Background(col).BorderWidth(0)
				if d.tone == toneDanger {
					cancel.AutoFocus()
				} else {
					ok.AutoFocus()
				}
				if cancel.Clicked() {
					cancelled = true
				}
				if ok.Hovered() {
					ok.Background(col.Alpha(0.88))
				}
				if ok.Clicked() {
					confirmed = true
				}
			})
		})
	}).Padding(0).Clip()
	switch {
	case confirmed:
		act := d.action
		s.confirm = confirmDialog{}
		if act != nil {
			act()
		}
	case cancelled || !d.open:
		s.confirm = confirmDialog{}
	}
}
