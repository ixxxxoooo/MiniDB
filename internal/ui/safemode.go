package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/locales"
)

// Safe Mode follows TablePlus: a connection can warn (Alert) or demand the
// database password (Safe) before a statement is sent, for everything or for
// everything but reads. The decision is pure (database.SafeModeRequirement);
// this file asks the user and runs the action once they agree.

// safeModeLabel is the short name of a level.
func safeModeLabel(m model.SafeMode) string {
	switch m {
	case model.SafeModeAlert1:
		return locales.SafeAlert1()
	case model.SafeModeAlert2:
		return locales.SafeAlert2()
	case model.SafeModeSafe1:
		return locales.SafeSafe1()
	case model.SafeModeSafe2:
		return locales.SafeSafe2()
	}
	return locales.SafeSilent()
}

// safeModeDesc says what a level asks for.
func safeModeDesc(m model.SafeMode) string {
	switch m {
	case model.SafeModeAlert1:
		return locales.SafeAlert1Desc()
	case model.SafeModeAlert2:
		return locales.SafeAlert2Desc()
	case model.SafeModeSafe1:
		return locales.SafeSafe1Desc()
	case model.SafeModeSafe2:
		return locales.SafeSafe2Desc()
	}
	return locales.SafeSilentDesc()
}

// safeModeIcon 是这一级的盾牌图标：静默是空盾，提醒类带感叹号，安全类带对勾。
func safeModeIcon(m model.SafeMode) *ui.SVG {
	switch m {
	case model.SafeModeAlert1, model.SafeModeAlert2:
		return icShieldAlert
	case model.SafeModeSafe1, model.SafeModeSafe2:
		return icShieldCheck
	}
	return icShield
}

// safeModeColor 是盾牌的颜色：静默不起眼，提醒类是警告色，安全类是成功色。
func safeModeColor(p tokens, m model.SafeMode) ui.Color {
	switch m {
	case model.SafeModeAlert1, model.SafeModeAlert2:
		return p.warning
	case model.SafeModeSafe1, model.SafeModeSafe2:
		return p.success
	}
	return p.fgSecondary
}

// safeModeTip 是悬浮时显示的完整说明：这一级属于哪一类、什么情况下会拦、没有密码时怎么办。
func safeModeTip(m model.SafeMode) string {
	switch m {
	case model.SafeModeAlert1:
		return locales.SafeAlert1Tip()
	case model.SafeModeAlert2:
		return locales.SafeAlert2Tip()
	case model.SafeModeSafe1:
		return locales.SafeSafe1Tip()
	case model.SafeModeSafe2:
		return locales.SafeSafe2Tip()
	}
	return locales.SafeSilentTip()
}

// safeModeGroup 是这一级属于的类别标题；每类的第一级前面要画一个标题，静默没有类别（返回空）。
func safeModeGroup(m model.SafeMode) string {
	switch m {
	case model.SafeModeAlert1:
		return locales.SafeGroupAlert()
	case model.SafeModeSafe1:
		return locales.SafeGroupSafe()
	}
	return ""
}

// safeModeFromLabel is the reverse of safeModeLabel, for the connection form.
func safeModeFromLabel(label string) model.SafeMode {
	for _, m := range model.SafeModes {
		if safeModeLabel(m) == label {
			return m
		}
	}
	return model.SafeModeSilent
}

// pwPrompt asks for a password. verify (when set) checks it in the background;
// done runs with the accepted password.
type pwPrompt struct {
	open   bool
	title  string
	msg    string
	pw     string
	err    string
	busy   bool
	verify func(pw string) bool
	done   func(pw string)
}

// stmtSummary lists the statements a confirmation is about, cut to a size a
// dialog can show.
func stmtSummary(stmts []string) string {
	const maxStmts, maxLen = 6, 160
	var b strings.Builder
	for i, s := range stmts {
		if i == maxStmts {
			b.WriteString("\n…")
			break
		}
		s = strings.Join(strings.Fields(s), " ")
		if len([]rune(s)) > maxLen {
			s = string([]rune(s)[:maxLen]) + "…"
		}
		b.WriteString("\n" + s)
	}
	return b.String()
}

// safeGate runs action once w's Safe Mode is satisfied: at once, after a
// confirmation, or after the database password was typed.
func (s *Shell) safeGate(w *Workspace, stmts []string, action func()) {
	mode := w.cfg.EffectiveSafeMode()
	switch database.SafeModeRequirement(mode, w.cfg.HasPassword, stmts) {
	case model.SafeConfirm:
		msg := locales.SafeSendMsg(w.cfg.Name, safeModeLabel(mode), len(stmts)) + "\n" + stmtSummary(stmts)
		s.askWarn(locales.SafeSendTitle(), msg, locales.SafeSend(), action)
	case model.SafePassword:
		s.askStoredPassword(w, locales.SafePasswordTitle(), locales.SafePasswordMsg(w.cfg.Name, safeModeLabel(mode)), action)
	default:
		action()
	}
}

// safeGatePassword is safeGate for actions that already ask for their own
// confirmation (drop, truncate): only a password level adds a step.
func (s *Shell) safeGatePassword(w *Workspace, stmts []string, action func()) {
	if database.SafeModeRequirement(w.cfg.EffectiveSafeMode(), w.cfg.HasPassword, stmts) == model.SafePassword {
		s.safeGate(w, stmts, action)
		return
	}
	action()
}

// askStoredPassword asks for w's database password and runs action when it matches.
func (s *Shell) askStoredPassword(w *Workspace, title, msg string, action func()) {
	id := w.cfg.ID
	s.pwp = pwPrompt{open: true, title: title, msg: msg,
		verify: func(pw string) bool { return s.app.Connections.VerifyPassword(id, pw) },
		done:   func(string) { action() }}
}

// setSafeMode changes w's level and saves it with the connection. Leaving a
// password level takes the password.
func (s *Shell) setSafeMode(w *Workspace, mode model.SafeMode) {
	if w.cfg.EffectiveSafeMode() == mode {
		return
	}
	apply := func() {
		for _, o := range s.spaces {
			if o.cfg.ID == w.cfg.ID {
				o.cfg.SafeMode = string(mode)
			}
		}
		cfg := w.cfg
		var err error
		s.up.bg(func() { err = s.app.Connections.SaveConnection(cfg) }, func() {
			if err != nil {
				s.toastErr(err.Error())
				return
			}
			s.loadConnections()
		})
	}
	switch w.cfg.EffectiveSafeMode() {
	case model.SafeModeSafe1, model.SafeModeSafe2:
		if w.cfg.HasPassword {
			s.askStoredPassword(w, locales.SafeChangeTitle(), locales.SafePasswordMsg(w.cfg.Name, safeModeLabel(w.cfg.EffectiveSafeMode())), apply)
			return
		}
	}
	apply()
}

// safeButton is the toolbar's Safe Mode menu; it names the current level.
func (s *Shell) safeButton(c *ui.Context, p tokens, w *Workspace) {
	cur := w.cfg.EffectiveSafeMode()
	btn := fieldMenuBtn(c.Key("safe"), safeModeIcon(cur), safeModeColor(p, cur), locales.SafeTitle()+" · "+safeModeLabel(cur)).Menu(func(m *ui.Menu) {
		for _, mode := range model.SafeModes {
			if safeModeGroup(mode) != "" { // 提醒类、安全类之间画条分隔线
				m.Separator()
			}
			if m.Item(safeModeLabel(mode)).Radio(mode == cur).Subtitle(safeModeDesc(mode)).Tooltip(safeModeTip(mode)).Chosen() {
				defer s.setSafeMode(w, mode)
			}
		}
	})
	tipOn(c, btn, locales.SafeTitle()+": "+safeModeLabel(cur)+"\n"+safeModeTip(cur))
}

// passwordView draws the password prompt.
func (s *Shell) passwordView(c *ui.Context) {
	d := &s.pwp
	if !d.open {
		return
	}
	p := s.tokens(c)
	submit := false
	s.dialog(c, &d.open, dlgOpts{title: d.title, width: 400}, func() {
		ui.Text(c, d.msg).FontSize(font2XS).TextColor(p.fgSecondary).FillWidth()
		in := ui.TextInput(c.Key("pwp"), &d.pw).Password().AutoFocus().FillWidth().Height(sizeBtn).Margin(8, 0, 0, 0)
		if in.Submitted() {
			submit = true
		}
		if d.err != "" {
			ui.Text(c, d.err).FontSize(font2XS).TextColor(p.danger).Margin(6, 0, 0, 0)
		}
	}, func() {
		if footBtn(c, locales.CommonCancel()).Clicked() {
			*d = pwPrompt{}
		}
		if footPrimary(c, locales.CommonConfirm()).Disabled(d.busy).Clicked() {
			submit = true
		}
	})
	if submit && d.open && !d.busy {
		s.submitPassword()
	}
}

func (s *Shell) submitPassword() {
	d := &s.pwp
	pw, verify, done := d.pw, d.verify, d.done
	accept := func() {
		*d = pwPrompt{}
		if done != nil {
			done(pw)
		}
	}
	if verify == nil {
		accept()
		return
	}
	d.busy, d.err = true, ""
	ok := false
	s.up.bg(func() { ok = verify(pw) }, func() {
		d.busy = false
		if !ok {
			d.err, d.pw = locales.SafePasswordWrong(), ""
			return
		}
		accept()
	})
}

// askConnectPassword asks for the password of a connection that does not keep
// one, then opens it.
func (s *Shell) askConnectPassword(cfg model.ConnectionConfig, open func(pw string)) {
	s.pwp = pwPrompt{open: true, title: locales.ConnAskPasswordTitle(), msg: locales.ConnAskPasswordMsg(cfg.Name), done: open}
}
