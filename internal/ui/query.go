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

// The pages of the result area.
const (
	pageData = iota
	pageChart
	pageMessages
)

// queryResult is the outcome of one statement of a run.
type queryResult struct {
	stmt string
	res  *database.QueryResult
	g    *resultGrid

	name   string // from a "-- name: …" comment above the statement
	pinned bool   // kept when the query runs again
	chart  chartState
	find   string           // narrows the rows shown to those containing it
	all    []map[string]any // every row, while find narrows what g shows

	showPreview bool    // 右侧的行预览栏（空格开关）
	previewW    float32 // 预览栏宽度
	pvq         string  // 预览栏里的字段搜索
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
	fav      favCache // 收藏按钮针对的语句，见 favorites.go

	vars        paramDialog
	page        int  // pageData, pageChart or pageMessages
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
	q.ed.Bindings = w.sh.queryBindings()
	q.ed.OnCompletion = q.complete
	w.tabs = append(w.tabs, q)
	w.active = len(w.tabs) - 1
	return q
}

// queryBindings steps the editor's own keys aside for the ones the tab handles.
func (s *Shell) queryBindings() []editor.Binding {
	var out []editor.Binding
	for _, k := range []chord{
		s.chordOf("run"), s.chordOf("run-all"), s.chordOf("format"),
		{ui.Cmd | ui.Shift, ui.KeyD}, {ui.Cmd | ui.Shift, ui.KeyI},
		{ui.Cmd | ui.Alt, ui.KeyBracketLeft}, {ui.Cmd | ui.Alt, ui.KeyBracketRight},
		{ui.Ctrl, ui.KeyBackquote},
	} {
		if k.valid() {
			out = append(out, editor.Binding{Mods: k.mods, Key: k.key, Command: editor.NoCommand})
		}
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
	p.ed.Bindings = w.sh.queryBindings()
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
		q.w.sh.toastWarn(locales.QueryEmpty())
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
			g := newLocalGrid()
			g.dbType = w.cfg.Type
			stmt := ""
			if i < len(stmts) {
				stmt = stmts[i]
			}
			qr := &queryResult{stmt: stmt, res: r, g: g, name: sqltext.ResultName(stmt), all: r.Rows}
			g.onKeys = func(c *ui.Context, tbl ui.Element) { qr.gridKeys(c, tbl, w.sh) }
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
		// a "-- minidb …chart" comment on the statement shown opens the chart at once
		if r := q.results[q.shown]; r.res.Error == "" && len(r.res.Columns) > 0 {
			if _, ok := sqltext.ParseChartDirective(r.stmt); ok {
				q.page = pageChart
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

// view draws the tab: one editor with its results, or two side by side.
func (q *queryTab) view(c *ui.Context, p tokens) {
	if q.pane2 == nil {
		q.body(c, p)
		return
	}
	// 左栏里可能处理 ⌘⇧D 取消分屏而把 q.pane2 清空，右栏要用这一帧开头取到的那个
	pane2 := q.pane2
	ui.SplitPercent(c, &q.splitH, func() {
		ui.Column(c.Key("pane1")).Fill().Children(func() { q.body(c, p) })
	}, func() {
		ui.Column(c.Key("pane2")).Fill().Children(func() { pane2.body(c, p) })
	}).Grow(1).FillWidth()
}

func (q *queryTab) body(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(4).Padding(0, 8).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
			if q.running {
				if actBtn(c, icStop, locales.QueryStop()).Clicked() {
					q.stop()
				}
				ui.Spinner(c)
			} else if tipOn(c, actPrimary(c, icPlay, locales.QueryRun()), locales.QueryRunCurrent()+q.w.sh.keyHint("run")).Clicked() {
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
			// 收藏当前语句：已收藏时是实心的黄色星标
			if _, faved := q.favoriteItem(); faved {
				if favBtn(c, icStarFill, locales.QueryUnfavorite(), p.warning).Clicked() {
					q.toggleFavorite()
				}
			} else if favBtn(c, icStar, locales.QueryFavorite(), p.fgSecondary).Clicked() {
				q.toggleFavorite()
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

	// 字号、换行、行号、缩进等由设置决定，所有查询编辑器共用
	q.w.sh.applyEditorPrefs(q.ed)
	edView := func() {
		editorBars(c, q.ed)
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
	case q.w.sh.hit(c, "run-all"):
		q.runDefault(true)
	case q.w.sh.hit(c, "run"):
		q.runDefault(false)
	case q.w.sh.hit(c, "format"):
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
		pal := q.w.sh.tokens(c)
		if q.failed != "" {
			q.banner(c, pal, kindError, q.failed, nil)
			return
		}
		if len(q.results) == 0 {
			// 还没运行过：给个提示，而不是一片空白
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icPlay).FontSize(22).TextColor(pal.fgMuted)
				ui.Text(c, locales.QueryEmptyResults()).FontSize(font2XS).TextColor(pal.fgMuted)
				ui.Text(c, strings.Trim(q.w.sh.keyHint("run"), " ()")).Font(mono).FontSize(font2XS).TextColor(pal.fgMuted)
			})
			return
		}
		if len(q.results) > 1 {
			labels := make([]string, len(q.results))
			for i, r := range q.results {
				labels[i] = r.label(i)
			}
			if i := pillTabs(c, pal, "results", labels, q.shown, false); i >= 0 {
				q.shown = i
			}
			ui.Box(c).Height(4).Shrink(0)
		}
		if q.shown >= len(q.results) {
			q.shown = len(q.results) - 1
		}
		r := q.results[q.shown]
		charting := r.res.Error == "" && len(r.res.Columns) > 0
		switch {
		case q.page == pageMessages:
			q.messagesView(c)
		case q.page == pageChart && charting:
			q.chartView(c, r, q.w.sh.tokens(c))
		default:
			switch {
			case r.res.ErrorCode == "cancelled":
				q.banner(c, pal, kindWarning, locales.QueryCancelled(), nil)
			case r.res.ErrorCode == "timeout":
				q.banner(c, pal, kindError, locales.QueryTimedOut(q.w.sh.prefs.QueryTimeout), nil)
			case r.res.ErrorCode == "read_only":
				q.banner(c, pal, kindError, locales.QueryReadOnly(), nil)
			case r.res.Error != "":
				q.banner(c, pal, kindError, r.res.Error, func() {
					if actBtn(c, icSparkle, locales.QueryAIDiagnose()).Clicked() {
						q.askAI(locales.AiPromptDiagnose(r.stmt, r.res.Error))
					}
				})
			case len(r.res.Columns) == 0:
				q.banner(c, pal, kindSuccess, locales.QueryOkAffected(int(r.res.Total)), nil)
			default:
				// 网格始终在树里同一个位置，预览栏只是加在右边，所以打开它不会让网格滚动位置丢失
				ui.Row(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
					ui.Column(c.Key("rgrid")).Grow(1).MinWidth(0).FillHeight().Children(func() {
						r.g.view(c, locales.QueryResultN(q.shown+1), q.w.sh.toast)
					})
					if r.showPreview {
						r.previewHandle(c, q.w.sh.tokens(c))
						ui.Column(c.Key("rpreview")).Width(r.previewW).FillHeight().Shrink(0).Children(func() { r.resultPreview(c, q.w.sh.tokens(c), q.w.sh) })
					}
				})
			}
		}
		ui.Box(c).Height(1).Background(th.Border).FillWidth()
		ui.Row(c).Gap(12).Padding(4, 12).AlignItems(ui.Center).FillWidth().Background(pal.surfaceSecondary).Children(func() {
			// 状态点：成功绿、失败红
			dot := pal.success
			if r.res.Error != "" {
				dot = pal.danger
			}
			ui.Box(c).Size(7, 7).Radius(3.5).Background(dot).Shrink(0)
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
			if r.res.Error == "" && len(r.res.Columns) > 0 && q.page == pageData {
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
			if i := pillTabs(c, pal, "pages", []string{locales.QueryPageData(), locales.QueryPageChart(), locales.QueryMessages()}, q.page, false); i >= 0 {
				q.page = i
			}
			if r.res.Error == "" && len(r.res.Columns) > 0 && r.stmt != "" {
				if actBtn(c, icDownload, locales.QueryExportResult()).Clicked() {
					q.w.sh.exportQuery(q.w, r.stmt)
				}
			}
		})
	})
}

// messagesView lists what each statement of the run did.
func (q *queryTab) messagesView(c *ui.Context) {
	p := q.w.sh.tokens(c)
	scrollbars(c, ui.Scroll(c.Key("msgs-list")).Grow(1).FillWidth()).Children(func() {
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

// banner 在结果区画一条提示：左侧一枚按种类配色的图标，错误信息等宽显示、可选中，
// 右边可以放一个操作（如"让 AI 诊断"）。
func (q *queryTab) banner(c *ui.Context, p tokens, kind, text string, action func()) {
	col := kindColor(p, kind)
	// 外层撑满剩余高度，让底部状态栏始终贴在最下面
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(10, 12).Children(func() {
		ui.Row(c).Gap(10).Padding(10, 12).AlignItems(ui.Start).FillWidth().Radius(radiusBtn+1).
			Background(col.Alpha(0.07)).Border(1, col.Alpha(0.28)).Shrink(0).Children(func() {
			ui.Icon(c, kindIcon(kind)).FontSize(15).TextColor(col).Shrink(0).Margin(1, 0, 0, 0)
			t := ui.Text(c, text).FontSize(font2XS + 1).TextColor(p.fg).Selectable().Grow(1).MinWidth(0)
			if kind == kindError {
				t.Font(mono)
			}
			if action != nil {
				action()
			}
		})
	})
}
