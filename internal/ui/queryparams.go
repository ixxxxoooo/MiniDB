package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/sqltext"
	"minidb/locales"
)

// paramDialog is the window that asks for the values of the placeholders.
type paramDialog struct {
	open  bool
	names []string
	vals  map[string]string
	stmts []string
	next  func([]string)
	apply bool // ⌘↵ pressed in a field: applied by the next frame
}

// apply fills the placeholders in, remembers the values, and runs on.
func (q *queryTab) applyParams() {
	d := &q.vars
	sh := q.w.sh
	pf := sh.prefs
	f := sqltext.ParamFormatOf(pf.ParamFormat)
	out := make([]string, len(d.stmts))
	for i, st := range d.stmts {
		out[i] = sqltext.ApplyParams(st, f, pf.ParamQuoted, d.vals)
	}
	if sh.prefs.ParamValues == nil {
		sh.prefs.ParamValues = map[string]string{}
	}
	for k, v := range d.vals {
		sh.prefs.ParamValues[k] = v
	}
	sh.savePrefs()
	next := d.next
	*d = paramDialog{}
	if next != nil {
		next(out)
	}
}

func (q *queryTab) paramsView(c *ui.Context) {
	d := &q.vars
	if !d.open {
		return
	}
	sh := q.w.sh
	p := sh.tokens(c)
	sh.dialog(c, &d.open, dlgOpts{title: locales.QueryParamsTitle(), sub: locales.QueryParamsSub(), width: 440}, func() {
		for i, name := range d.names {
			name := name
			dlgField(c, p, name, func() {
				v := d.vals[name]
				in := ui.TextInput(c.Key("pv:"+name), &v).FillWidth().Height(sizeBtn).Font(mono).
					OnTextKey(func(k ui.TextKey) bool { // ⌘↵ applies from inside a field
						if k.Key == ui.KeyEnter && k.Mods == ui.Cmd {
							d.apply = true
							q.w.sh.redraw()
							return true
						}
						return false
					})
				if i == 0 {
					in.AutoFocus()
				}
				d.vals[name] = v
			})
		}
		ui.Text(c, locales.QueryParamsHint()).FontSize(font2XS).TextColor(p.fgMuted)
	}, func() {
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		if footPrimary(c, locales.QueryRun()+q.w.sh.keyHint("run")).Clicked() {
			q.applyParams()
		}
	})
	if d.open && (d.apply || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
		q.applyParams()
	}
}
