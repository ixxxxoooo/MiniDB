package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// Safe Mode follows TablePlus: a connection can warn (Alert) or demand the
// database password (Safe) before a statement is sent, for everything or for
// everything but reads. The decision is pure (database.SafeModeRequirement);
// this file asks the user and runs the action once they agree.

// safeModeLabel is the short name of a level.
func safeModeLabel(m database.SafeMode) string {
	switch m {
	case database.SafeModeAlert1:
		return locales.SafeAlert1()
	case database.SafeModeAlert2:
		return locales.SafeAlert2()
	case database.SafeModeSafe1:
		return locales.SafeSafe1()
	case database.SafeModeSafe2:
		return locales.SafeSafe2()
	}
	return locales.SafeSilent()
}

// safeModeDesc says what a level asks for.
func safeModeDesc(m database.SafeMode) string {
	switch m {
	case database.SafeModeAlert1:
		return locales.SafeAlert1Desc()
	case database.SafeModeAlert2:
		return locales.SafeAlert2Desc()
	case database.SafeModeSafe1:
		return locales.SafeSafe1Desc()
	case database.SafeModeSafe2:
		return locales.SafeSafe2Desc()
	}
	return locales.SafeSilentDesc()
}

// safeModeFromLabel is the reverse of safeModeLabel, for the connection form.
func safeModeFromLabel(label string) database.SafeMode {
	for _, m := range database.SafeModes {
		if safeModeLabel(m) == label {
			return m
		}
	}
	return database.SafeModeSilent
}

func safeModeLabels() []string {
	out := make([]string, len(database.SafeModes))
	for i, m := range database.SafeModes {
		out[i] = safeModeLabel(m)
	}
	return out
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
	case database.SafeConfirm:
		msg := locales.SafeSendMsg(w.cfg.Name, safeModeLabel(mode), len(stmts)) + "\n" + stmtSummary(stmts)
		s.ask(locales.SafeSendTitle(), msg, locales.SafeSend(), action)
	case database.SafePassword:
		s.askStoredPassword(w, locales.SafePasswordTitle(), locales.SafePasswordMsg(w.cfg.Name, safeModeLabel(mode)), action)
	default:
		action()
	}
}

// safeGatePassword is safeGate for actions that already ask for their own
// confirmation (drop, truncate): only a password level adds a step.
func (s *Shell) safeGatePassword(w *Workspace, stmts []string, action func()) {
	if database.SafeModeRequirement(w.cfg.EffectiveSafeMode(), w.cfg.HasPassword, stmts) == database.SafePassword {
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
func (s *Shell) setSafeMode(w *Workspace, mode database.SafeMode) {
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
	case database.SafeModeSafe1, database.SafeModeSafe2:
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
	ui.MenuButton(c.Key("safe"), locales.SafeTitle()+" · "+safeModeLabel(cur), func(m *ui.Menu) {
		for _, mode := range database.SafeModes {
			if m.Item(safeModeLabel(mode)).Radio(mode == cur).Subtitle(safeModeDesc(mode)).Chosen() {
				defer s.setSafeMode(w, mode)
			}
		}
	}).Height(sizeBtn).FontSize(font2XS)
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
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			*d = pwPrompt{}
		}
		if ui.PrimaryButton(c, locales.CommonConfirm()).Height(sizeBtn).Disabled(d.busy).Clicked() {
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
func (s *Shell) askConnectPassword(cfg database.ConnectionConfig, open func(pw string)) {
	s.pwp = pwPrompt{open: true, title: locales.ConnAskPasswordTitle(), msg: locales.ConnAskPasswordMsg(cfg.Name), done: open}
}
