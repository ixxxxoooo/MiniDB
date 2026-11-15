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
	code    string        // 说明下面的等宽代码框（安全模式确认发送的语句），空则没有
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

// askWarnCode 同 askWarn，另在说明下面用等宽代码框列出将要执行的语句 code。
func (s *Shell) askWarnCode(title, message, code, label string, action func()) {
	s.confirm = confirmDialog{open: true, title: title, message: message, code: code, action: action, label: label, tone: toneWarn}
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

// confirmView 是应用统一的确认框（设计稿「危险操作」）：头部一枚 28px 按基调配色的圆形图标加标题，
// 下面是说明和复选项，底栏左边是按键提示、右边取消和确认两个按钮。破坏性操作的确认按钮是红色实心，
// 默认焦点在"取消"上，回车不会直接删东西，要确认按 ⌘⏎；其余情况默认焦点在确认键，回车即确认；Esc 取消。
func (s *Shell) confirmView(c *ui.Context) {
	d := &s.confirm
	if !d.open {
		return
	}
	p := s.tokens(c)
	col, bg, icon := p.accentText, p.accentSoft, icQuestion
	switch d.tone {
	case toneWarn:
		col, bg, icon = p.warning, p.warningBg, icAlertTri
	case toneDanger:
		col, bg, icon = p.danger, p.dangerBg, icAlertTri
	}
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBody
	ct.Surface = p.surface
	confirmed, cancelled := false, false
	modalPanel(c, p, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(confirmWidth).Children(func() {
			if d.tone == toneDanger && c.Shortcut(ui.Cmd, ui.KeyEnter) { // ⌘⏎ 确认；回车落在「取消」上，所以不会误删
				confirmed = true
			}
			ui.Row(c).Gap(spaceMD).AlignItems(ui.Center).Padding(spaceLG, spaceLG, spaceXS, spaceLG).FillWidth().Children(func() {
				ui.Row(c).Size(28, 28).Shrink(0).Center().Radius(14).Background(bg).Children(func() {
					ui.Icon(c, icon).FontSize(fontTitle).TextColor(col)
				})
				ui.Text(c, d.title).FontWeight(600).FontSize(fontTitle).Grow(1).MinWidth(0)
			})
			ui.Column(c).Gap(spaceSM).Padding(spaceSM, spaceLG, spaceLG-2, spaceLG).FillWidth().Children(func() {
				ui.Text(c, d.message).FontSize(fontBody).TextColor(p.fgSecondary).Selectable()
				if d.code != "" {
					ui.Text(c, d.code).Font(mono).FontSize(fontDense).TextColor(p.fg).Selectable().FillWidth().Padding(spaceSM+1, spaceMD-1).
						Radius(radiusBtn).Background(p.surfaceTertiary).Border(1, p.border)
				}
				for i, o := range d.opts {
					ui.Checkbox(c.Key("confirm-opt"+itoa(i)), &o.on, o.label).Margin(spaceXS, 0, 0, 0)
				}
			})
			hairline(c, p.borderSubtle)
			ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Justify(ui.End).Padding(spaceMD-2, spaceMD+2).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
				if d.tone == toneDanger {
					ui.Text(c, locales.ConfirmHintDanger(d.label)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Grow(1).MinWidth(0)
				}
				// 默认焦点：破坏性操作落在"取消"上（回车是安全的），其余落在确认键上
				cancel := footBtn(c.Key("confirm-cancel"), locales.CommonCancel())
				var ok ui.Element
				if d.tone == toneDanger {
					ok = dangerFootBtn(c.Key("confirm-ok"), d.label)
					cancel.AutoFocus()
				} else {
					ok = footPrimary(c.Key("confirm-ok"), d.label)
					ok.AutoFocus()
				}
				if cancel.Clicked() {
					cancelled = true
				}
				if ok.Clicked() {
					confirmed = true
				}
			})
		})
	}).Padding(0).Clip().Radius(radiusDialog).Border(1, p.border).Shadow(0, shadowY, shadowBlur, 0, p.shadow)
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

// confirmWidth 是确认框的宽度（设计稿 380）。
const confirmWidth = 380
