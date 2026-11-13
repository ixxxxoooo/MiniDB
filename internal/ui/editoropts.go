package ui

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// ---- 收藏查询的关键字 ----

// loadKeywords reads the keywords bound to favorite queries, for completion.
func (w *Workspace) loadKeywords() {
	var kw map[string]string
	w.sh.up.bg(func() { kw, _ = w.sh.app.History.Keywords(w.cfg.ID) }, func() { w.keywords = kw })
}

// bindKeyword asks for the word that stands for a favorite query.
func (w *Workspace) bindKeyword(it app.HistoryItem) {
	w.sh.prompt(promptDialog{title: locales.QueryBindKeyword(), sub: firstLines(it.SQL, 1), label: locales.QueryKeyword(), value: it.Keyword,
		ok: locales.CommonConfirm(), action: func(kw string, _ bool, _ string) { w.setKeyword(it.ID, kw) }})
}

func (w *Workspace) setKeyword(id, kw string) {
	var err error
	w.sh.up.bg(func() { err = w.sh.app.History.SetKeyword(id, kw) }, func() {
		if err != nil {
			w.sh.toast(err.Error())
			return
		}
		w.loadKeywords()
		w.loadSideQueries()
	})
}

// ---- 编辑器选项 ----

var rowLimits = []int{0, 100, 500, 1000, 5000, 10000}

func rowLimitLabel(n int) string {
	if n == 0 {
		return locales.EditorRowLimitDefault()
	}
	return strconv.Itoa(n)
}

func rowLimitLabels() []string {
	out := make([]string, len(rowLimits))
	for i, n := range rowLimits {
		out[i] = rowLimitLabel(n)
	}
	return out
}

func paramFormatLabels() []string {
	out := make([]string, len(sqltext.ParamFormats))
	for i, f := range sqltext.ParamFormats {
		out[i] = f.Example()
	}
	return out
}

// editorOptions are the query editor's choices. The gear in a query tab and
// Settings both show them.
func (s *Shell) editorOptions(c *ui.Context, p tokens) {
	pf := &s.prefs
	changed := false
	row := func(label string, control func()) {
		ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, label).Grow(1).FontSize(fontBase)
			control()
		})
	}
	section := func(title string) {
		ui.Text(c, title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary).Margin(6, 0, 0, 0)
	}

	section(locales.EditorSectionRun())
	run := locales.EditorRunCurrent()
	if pf.RunAll {
		run = locales.EditorRunAll()
	}
	row(locales.EditorRunDefault(), func() {
		if ui.Select(c.Key("ed-run"), &run, []string{locales.EditorRunCurrent(), locales.EditorRunAll()}).Width(170).Height(sizeBtn).Changed() {
			pf.RunAll, changed = run == locales.EditorRunAll(), true
		}
	})
	limit := rowLimitLabel(pf.RowLimit)
	row(locales.EditorRowLimit(), func() {
		if ui.Select(c.Key("ed-limit"), &limit, rowLimitLabels()).Width(170).Height(sizeBtn).Changed() {
			for _, n := range rowLimits {
				if rowLimitLabel(n) == limit {
					pf.RowLimit, changed = n, true
				}
			}
		}
	})

	section(locales.EditorSectionParams())
	row(locales.EditorParams(), func() {
		if ui.Switch(c.Key("ed-params"), &pf.Params).Changed() {
			changed = true
		}
	})
	if pf.Params {
		format := sqltext.ParamFormatOf(pf.ParamFormat).Example()
		row(locales.EditorParamFormat(), func() {
			if ui.Select(c.Key("ed-pfmt"), &format, paramFormatLabels()).Width(170).Height(sizeBtn).Changed() {
				for _, f := range sqltext.ParamFormats {
					if f.Example() == format {
						pf.ParamFormat, changed = string(f), true
					}
				}
			}
		})
		row(locales.EditorParamQuoted(), func() {
			if ui.Switch(c.Key("ed-pq"), &pf.ParamQuoted).Changed() {
				changed = true
			}
		})
		row(locales.EditorParamValues(len(pf.ParamValues)), func() {
			if ui.Button(c.Key("ed-pclear"), locales.EditorParamClear()).Height(sizeBtnSm).Disabled(len(pf.ParamValues) == 0).Clicked() {
				pf.ParamValues, changed = nil, true
			}
		})
	}

	section(locales.EditorSectionComplete())
	row(locales.EditorCompUpper(), func() {
		if ui.Switch(c.Key("ed-up"), &pf.CompUpper).Changed() {
			changed = true
		}
	})
	row(locales.EditorCompSpace(), func() {
		if ui.Switch(c.Key("ed-sp"), &pf.CompSpace).Changed() {
			changed = true
		}
	})
	autoClose := !pf.NoAutoClose
	row(locales.EditorAutoClose(), func() {
		if ui.Switch(c.Key("ed-ac"), &autoClose).Changed() {
			pf.NoAutoClose, changed = !autoClose, true
		}
	})
	if changed {
		s.savePrefs()
	}
}

func (s *Shell) editorOptionsDialog(c *ui.Context) {
	if !s.edOpt {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &s.edOpt, dlgOpts{title: locales.EditorOptions(), width: 460}, func() {
		s.editorOptions(c, p)
	}, nil)
}
