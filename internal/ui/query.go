package ui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/database"
	"minidb/internal/sqltext"
	"minidb/locales"
)

var requestSeq atomic.Int64

// queryResult is the outcome of one statement of a run.
type queryResult struct {
	stmt string
	res  *database.QueryResult
	g    *resultGrid

	name   string           // from a "-- name: …" comment above the statement
	pinned bool             // kept when the query runs again
	find   string           // narrows the rows shown to those containing it
	all    []map[string]any // every row, while find narrows what g shows
}

// label is the result's tab title.
func (r *queryResult) label(i int) string {
	t := r.name
	if t == "" {
		t = locales.QueryResultN(i + 1)
	}
	if r.pinned {
		t = "◆ " + t
	}
	return t
}

// narrow shows only the rows containing r.find (any cell, case-insensitive).
func (r *queryResult) narrow() {
	rows := r.all
	if q := strings.ToLower(strings.TrimSpace(r.find)); q != "" {
		rows = nil
		for _, row := range r.all {
			for _, v := range row {
				if v != nil && strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
					rows = append(rows, row)
					break
				}
			}
		}
	}
	r.g.set(r.res.Columns, rows)
}

// queryTab is a SQL editor over a result area.
type queryTab struct {
	w *Workspace
	n int

	ed *editor.State

	results []*queryResult
	shown   int
	running bool
	reqID   string
	started time.Time
	failed  string

	splitPct float32
	history  historyPanel

	vars        paramDialog
	showMsgs    bool // the Messages page instead of the rows
	hideResults bool // ⌃` folds the result area away

	// A split puts a second editor beside this one (⌘⇧D). The second is a
	// queryTab of its own that is not in the tab bar; parent is set on it.
	pane2  *queryTab
	parent *queryTab
	second bool      // on the root: the second pane has the keyboard
	want   *queryTab // on the root: the pane whose editor takes the focus next frame
	splitH float32
}

func (w *Workspace) newQuery(sql string) *queryTab {
	w.queryN++
	buf := editor.NewBuffer(sql)
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	q := &queryTab{w: w, n: w.queryN, splitPct: 45}
	q.ed = &editor.State{Buffer: buf, SoftWrap: true}
	// Cmd+Enter runs; the editor's default (open a line below) steps aside.
	q.ed.Bindings = queryBindings()
	q.ed.OnCompletion = q.complete
	w.tabs = append(w.tabs, q)
	w.active = len(w.tabs) - 1
	return q
}

// queryBindings steps the editor's own keys aside for the ones the tab handles.
func queryBindings() []editor.Binding {
	var out []editor.Binding
	for _, k := range []struct {
		mods ui.Modifiers
		key  ui.Key
	}{
		{ui.Cmd, ui.KeyEnter}, {ui.Cmd | ui.Shift, ui.KeyEnter},
		{ui.Cmd | ui.Shift, ui.KeyD}, {ui.Cmd | ui.Shift, ui.KeyI},
		{ui.Cmd | ui.Alt, ui.KeyBracketLeft}, {ui.Cmd | ui.Alt, ui.KeyBracketRight},
		{ui.Ctrl, ui.KeyBackquote},
	} {
		out = append(out, editor.Binding{Mods: k.mods, Key: k.key, Command: editor.NoCommand})
	}
	return out
}

// newPane makes the second editor of a split.
func (q *queryTab) newPane(sql string) *queryTab {
	w := q.w
	w.queryN++
	buf := editor.NewBuffer(sql)
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	p := &queryTab{w: w, n: w.queryN, splitPct: 45, parent: q}
	p.ed = &editor.State{Buffer: buf, SoftWrap: true}
	p.ed.Bindings = queryBindings()
	p.ed.OnCompletion = p.complete
	return p
}

// root is the tab in the tab bar: this one, or the one a split pane belongs to.
func (q *queryTab) root() *queryTab {
	if q.parent != nil {
		return q.parent
	}
	return q
}

// focusedPane is the pane with the keyboard: the second one after ⌘⇧D or a
// click in it, else the tab itself.
func (q *queryTab) focusedPane() *queryTab {
	if q.pane2 != nil && q.second {
		return q.pane2
	}
	return q
}

// isActive tells whether this pane takes the keyboard shortcuts.
func (q *queryTab) isActive() bool {
	r := q.root()
	return r.focusedPane() == q
}

// toggleSplit opens the second pane to the right, or closes it.
func (q *queryTab) toggleSplit() {
	if q.pane2 != nil {
		q.pane2, q.second, q.want = nil, false, q
		return
	}
	q.pane2, q.splitH = q.newPane(""), 50
	q.second = true
	q.want = q.pane2
}

// switchPane moves the keyboard to the other pane (⌘⌥[ and ⌘⌥]).
func (q *queryTab) switchPane() {
	if q.pane2 != nil {
		q.second = !q.second
		q.want = q.focusedPane()
	}
}

func (q *queryTab) title() string { return locales.TabQuery(q.n) }
func (q *queryTab) icon() *ui.SVG { return icCode }
func (q *queryTab) pending() int  { return 0 }

// favoriteKeywords completes a bound keyword to its whole statement.
func (q *queryTab) keywordCompletions(prefix string) []editor.Completion {
	var out []editor.Completion
	lp := strings.ToLower(prefix)
	for kw, sql := range q.w.keywords {
		if lp == "" || strings.HasPrefix(strings.ToLower(kw), lp) {
			out = append(out, editor.Completion{Label: kw, Detail: "★ " + firstLines(sql, 1), Kind: editor.CompletionSnippet, Insert: sql})
		}
	}
	return out
}

func (q *queryTab) complete(r editor.CompletionRequest) {
	src := q.ed.Buffer.String()
	var out []editor.Completion
	pf := q.w.sh.prefs
	for _, it := range sqltext.Complete(src, r.Offset, r.Prefix, q.w.schema) {
		kind := editor.CompletionKeyword
		switch it.Kind {
		case sqltext.KindTable:
			kind = editor.CompletionType
		case sqltext.KindColumn:
			kind = editor.CompletionField
		}
		insert := it.Label
		if kind == editor.CompletionKeyword && pf.CompUpper {
			insert = strings.ToUpper(insert)
		}
		if pf.CompSpace {
			insert += " "
		}
		out = append(out, editor.Completion{Label: it.Label, Detail: it.Detail, Kind: kind, Insert: insert})
	}
	out = append(q.keywordCompletions(r.Prefix), out...)
	q.ed.ShowCompletions(r.Offset-len(r.Prefix), out)
}

// statements returns what a run executes: all statements, or the selection's,
// or the statement at the caret.
func (q *queryTab) statements(all bool) []string {
	src := q.ed.Buffer.String()
	if all {
		return sqltext.Texts(src)
	}
	if sel := q.ed.SelectedText(); sel != "" {
		return sqltext.Texts(sel)
	}
	if st, ok := sqltext.At(src, q.ed.Cursor()); ok {
		return []string{st.Text}
	}
	return nil
}

func (q *queryTab) run(all bool) {
	if q.running {
		return
	}
	stmts := q.statements(all)
	if len(stmts) == 0 {
		q.w.sh.toast(locales.QueryEmpty())
		return
	}
	q.withParams(stmts, q.runGated)
}

// runDefault is ⌘↵, which runs the statement at the caret unless Settings say all.
func (q *queryTab) runDefault(flip bool) { q.run(q.w.sh.prefs.RunAll != flip) }

// withParams asks for the values of the statements' placeholders, when query
// parameters are on and there are any, then passes the statements on with
// the values in.
func (q *queryTab) withParams(stmts []string, next func([]string)) {
	pf := q.w.sh.prefs
	if !pf.Params {
		next(stmts)
		return
	}
	f := sqltext.ParamFormatOf(pf.ParamFormat)
	names := sqltext.ParamNames(stmts, f, pf.ParamQuoted)
	if len(names) == 0 {
		next(stmts)
		return
	}
	vals := map[string]string{}
	for _, n := range names {
		vals[n] = pf.ParamValues[n]
	}
	q.vars = paramDialog{open: true, names: names, vals: vals, stmts: stmts, next: next}
}

// runGated runs stmts once the connection's Safe Mode agrees.
func (q *queryTab) runGated(stmts []string) {
	q.w.sh.safeGate(q.w, stmts, func() { q.runStatements(stmts) })
}

func (q *queryTab) runStatements(stmts []string) {
	w := q.w
	q.running, q.failed = true, ""
	q.started = time.Now()
	q.reqID = fmt.Sprintf("q%d", requestSeq.Add(1))
	reqID, db := q.reqID, w.db
	pageSize := database.DefaultPageSize
	if n := w.sh.prefs.RowLimit; n > 0 {
		pageSize = n
	}
	var results []*database.QueryResult
	var err error
	w.sh.up.bg(func() {
		results, err = w.sh.app.Queries.ExecuteSQLBatch(reqID, w.cfg.ID, db, stmts, 1, pageSize)
		for _, s := range stmts {
			_ = w.sh.app.History.Add(w.cfg.ID, db, s)
		}
	}, func() {
		if q.reqID != reqID {
			return
		}
		q.running = false
		if err != nil {
			q.failed = err.Error()
			for _, s := range stmts {
				w.sh.logSQL(db, s, time.Since(q.started), -1, q.failed)
			}
			return
		}
		// pinned results stay, in front of the new ones
		var pinned []*queryResult
		for _, r := range q.results {
			if r.pinned {
				pinned = append(pinned, r)
			}
		}
		q.results = append(q.results[:0], pinned...)
		for i, r := range results {
			g := newResultGrid(false)
			g.dbType = w.cfg.Type
			stmt := ""
			if i < len(stmts) {
				stmt = stmts[i]
			}
			qr := &queryResult{stmt: stmt, res: r, g: g, name: sqltext.ResultName(stmt), all: r.Rows}
			qr.narrow()
			q.results = append(q.results, qr)
			w.sh.logSQL(db, stmt, time.Duration(r.Duration)*time.Millisecond, len(r.Rows), r.Error)
		}
		q.shown = max(0, len(q.results)-1)
		for i := len(pinned); i < len(q.results); i++ {
			if q.results[i].res.Error != "" {
				q.shown = i
				break
			}
		}
		if w.sh.win != nil {
			w.sh.win.Invalidate()
		}
	})
}

// currentSQL is what Run would execute, as one string.
func (q *queryTab) currentSQL() string {
	return strings.Join(q.statements(false), ";\n")
}

// askAI opens the assistant and sends it a prompt.
func (q *queryTab) askAI(prompt string) {
	w := q.w
	w.panel = 2
	w.ai.send(prompt)
}

func (q *queryTab) stop() {
	if q.running {
		q.w.sh.app.Queries.CancelQuery(q.reqID)
	}
}

func (q *queryTab) format() {
	src := q.ed.Buffer.String()
	if sel := q.ed.SelectedText(); sel != "" {
		s := q.ed.Selection()
		q.ed.Buffer.Edit(editor.Edit{Start: s.Start(), End: s.End(), Text: sqltext.Format(sel)})
		return
	}
	var out string
	last := 0
	for _, st := range sqltext.Split(src) {
		out += src[last:st.Start] + sqltext.Format(st.Text)
		last = st.End
	}
	out += src[last:]
	if out != src {
		q.ed.Buffer.Set(out)
	}
}

// uglify squeezes the selection, or the whole script, onto one line.
func (q *queryTab) uglify() {
	if sel := q.ed.SelectedText(); sel != "" {
		s := q.ed.Selection()
		q.ed.Buffer.Edit(editor.Edit{Start: s.Start(), End: s.End(), Text: sqltext.Uglify(sel)})
		return
	}
	src := q.ed.Buffer.String()
	if out := sqltext.Uglify(src); out != src {
		q.ed.Buffer.Set(out)
	}
}

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
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			d.open = false
		}
		if ui.PrimaryButton(c, locales.QueryRun()+" (⌘↵)").Height(sizeBtn).Clicked() {
			q.applyParams()
		}
	})
	if d.open && (d.apply || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
		q.applyParams()
	}
}

// view draws the tab: one editor with its results, or two side by side.
func (q *queryTab) view(c *ui.Context, p tokens) {
	if q.pane2 == nil {
		q.body(c, p)
		return
	}
	ui.SplitPercent(c, &q.splitH, func() {
		ui.Column(c.Key("pane1")).Fill().Children(func() { q.body(c, p) })
	}, func() {
		ui.Column(c.Key("pane2")).Fill().Children(func() { q.pane2.body(c, p) })
	}).Grow(1).FillWidth()
}

func (q *queryTab) body(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(4).Padding(0, 8).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
			if q.running {
				if ui.Button(c, "").Label(locales.QueryStop()).Height(sizeBtnSm).Children(func() {
					ui.Icon(c, icStop).FontSize(12)
					ui.Text(c, locales.QueryStop()).FontSize(font2XS)
				}).Clicked() {
					q.stop()
				}
				ui.Spinner(c)
			} else if ui.PrimaryButton(c, "").Label(locales.QueryRun()).Tooltip(locales.QueryRunCurrent()).Height(sizeBtnSm).Padding(0, 10).Children(func() {
				ui.Icon(c, icPlay).FontSize(11)
				ui.Text(c, locales.QueryRun()).FontSize(font2XS).FontWeight(500)
			}).Clicked() {
				q.run(false)
			}
			if tipBtn(c, icPlayAll, locales.QueryRunAll()).Size(sizeBtnSm, sizeBtnSm).Disabled(q.running).Clicked() {
				q.run(true)
			}
			vline(c, p.border, 12)
			if tipBtn(c, icFormat, locales.QueryFormat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.format()
			}
			if tipBtn(c, icUglify, locales.QueryUglify()+" (⌘⇧I)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.uglify()
			}
			if tipBtn(c, icHistory, locales.QueryHistory()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.history.toggle(q)
			}
			if tipBtn(c, icSparkle, locales.QueryAIExplain()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.askAI(locales.AiPromptExplain(q.currentSQL()))
			}
			ui.Spacer(c)
			if q.running {
				ui.Text(c, locales.QueryRunning()).TextColor(p.fgMuted).FontSize(font2XS)
			}
			if q.parent == nil && tipBtn(c.Key("split"), icColumns, locales.QuerySplit()+" (⌘⇧D)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.toggleSplit()
			}
			if q.parent != nil && tipBtn(c.Key("unsplit"), icX, locales.QueryCloseSplit()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.parent.toggleSplit()
			}
			if tipBtn(c.Key("edopts"), icSettings, locales.EditorOptions()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.w.sh.edOpt = true
			}
		})
	}).Padding(0)
	hairline(c, p.border)

	// 字号由 ⌘= / ⌘- 调整，所有编辑器共用
	if sz := q.w.sh.prefs.EditorFont; sz > 0 {
		q.ed.Font.Size = sz
	}
	q.ed.NoAutoClose = q.w.sh.prefs.NoAutoClose
	edView := func() {
		e := editor.View(c, q.ed).Grow(1).FillWidth()
		if r := q.root(); r.want == q {
			r.want = nil
			e.Focus()
		} else if q.pane2 == nil && q.parent == nil {
			e.AutoFocus()
		}
		if e.Focused() {
			q.root().second = q.parent != nil
		}
	}
	if q.hideResults {
		ui.Column(c).Fill().Children(edView)
	} else {
		ui.SplitVerticalPercent(c, &q.splitPct, edView, func() { q.resultsView(c) }).Grow(1).FillWidth()
	}

	q.history.view(c, q)
	q.paramsView(c)
	if !q.isActive() || q.vars.open {
		return
	}

	switch {
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter):
		q.runDefault(true)
	case c.Shortcut(ui.Cmd, ui.KeyEnter):
		q.runDefault(false)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF):
		q.format()
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyI):
		q.uglify()
	case c.Shortcut(ui.Cmd, ui.KeyY):
		q.history.toggle(q)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyD):
		q.root().toggleSplit()
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyBracketLeft), c.Shortcut(ui.Cmd|ui.Alt, ui.KeyBracketRight):
		q.root().switchPane()
	case c.Shortcut(ui.Ctrl, ui.KeyBackquote):
		q.hideResults = !q.hideResults
	}
}

func (q *queryTab) resultsView(c *ui.Context) {
	th := c.Theme()
	ui.Column(c).Fill().Children(func() {
		if q.failed != "" {
			ui.Text(c, q.failed).TextColor(th.Danger).Padding(12).Selectable()
			return
		}
		if len(q.results) == 0 {
			return
		}
		if len(q.results) > 1 {
			labels := make([]string, len(q.results))
			for i, r := range q.results {
				labels[i] = r.label(i)
			}
			ui.Segmented(c, &q.shown, labels...).Margin(6, 10)
		}
		if q.shown >= len(q.results) {
			q.shown = len(q.results) - 1
		}
		r := q.results[q.shown]
		if q.showMsgs {
			q.messagesView(c)
		} else {
			switch {
			case r.res.ErrorCode == "cancelled":
				ui.Text(c, locales.QueryCancelled()).TextColor(th.Warning).Padding(12)
			case r.res.ErrorCode == "read_only":
				ui.Text(c, locales.QueryReadOnly()).TextColor(th.Danger).Padding(12)
			case r.res.Error != "":
				ui.Row(c).Gap(10).Padding(12).AlignItems(ui.Center).FillWidth().Children(func() {
					ui.Text(c, r.res.Error).TextColor(th.Danger).Selectable().Grow(1)
					if labeledButton(c, icSparkle, locales.QueryAIDiagnose(), false).Clicked() {
						q.askAI(locales.AiPromptDiagnose(r.stmt, r.res.Error))
					}
				})
			case len(r.res.Columns) == 0:
				ui.Text(c, locales.QueryOkAffected(int(r.res.Total))).TextColor(th.Success).Padding(12)
			default:
				r.g.view(c, locales.QueryResultN(q.shown+1), q.w.sh.toast)
			}
		}
		ui.Box(c).Height(1).Background(th.Border).FillWidth()
		ui.Row(c).Gap(12).Padding(5, 12).AlignItems(ui.Center).FillWidth().Children(func() {
			if r.res.Error == "" && len(r.res.Columns) > 0 {
				switch {
				case r.find != "":
					ui.Text(c, locales.QueryFound(len(r.g.rows), len(r.all))).FontSize(12).TextColor(th.TextMuted)
				case int64(len(r.res.Rows)) < r.res.Total:
					ui.Text(c, locales.QueryTruncated(len(r.res.Rows), int(r.res.Total))).FontSize(12).TextColor(th.TextMuted)
				default:
					ui.Text(c, locales.TableRows(len(r.res.Rows))).FontSize(12).TextColor(th.TextMuted)
				}
			}
			ui.Text(c, locales.StatusDuration(int(r.res.Duration))).FontSize(12).TextColor(th.TextMuted)
			ui.Spacer(c)
			if r.res.Error == "" && len(r.res.Columns) > 0 && !q.showMsgs {
				if ui.SearchField(c.Key("rfind"), &r.find).Placeholder(locales.QueryFindInResult()).FontSize(font2XS).Width(180).Height(sizeBtnSm).Changed() {
					r.narrow()
				}
			}
			pin := locales.QueryPin()
			if r.pinned {
				pin = locales.QueryUnpin()
			}
			if tipBtn(c.Key("pin"), icPin, pin).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				r.pinned = !r.pinned
			}
			ui.Toggle(c.Key("msgs"), &q.showMsgs, locales.QueryMessages())
			if r.res.Error == "" && len(r.res.Columns) > 0 && r.stmt != "" {
				if ui.Button(c, locales.QueryExportResult()).Clicked() {
					q.w.sh.exportQuery(q.w, r.stmt)
				}
			}
		})
	})
}

// messagesView lists what each statement of the run did.
func (q *queryTab) messagesView(c *ui.Context) {
	p := q.w.sh.tokens(c)
	ui.Scroll(c.Key("msgs-list")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).Gap(4).Padding(10, 12).FillWidth().Children(func() {
			for i, r := range q.results {
				col, text := p.success, locales.QueryOkAffected(int(r.res.Total))
				if len(r.res.Columns) > 0 {
					text = locales.TableRows(len(r.res.Rows))
				}
				if r.res.Error != "" {
					col, text = p.danger, r.res.Error
				}
				ui.Row(c.Key(fmt.Sprint("m", i))).Gap(8).AlignItems(ui.Start).FillWidth().Children(func() {
					ui.Text(c, "●").FontSize(font2XS).TextColor(col)
					ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, firstLines(r.stmt, 2)).Font(mono).FontSize(font2XS).Selectable()
						ui.Text(c, text+" · "+locales.StatusDuration(int(r.res.Duration))).FontSize(font2XS).TextColor(col).Selectable()
					})
				})
			}
		})
	})
}
