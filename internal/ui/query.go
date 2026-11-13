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
}

func (w *Workspace) newQuery(sql string) *queryTab {
	w.queryN++
	buf := editor.NewBuffer(sql)
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	q := &queryTab{w: w, n: w.queryN, splitPct: 45}
	q.ed = &editor.State{Buffer: buf, SoftWrap: true}
	// Cmd+Enter runs; the editor's default (open a line below) steps aside.
	q.ed.Bindings = []editor.Binding{
		{Mods: ui.Cmd, Key: ui.KeyEnter, Command: editor.NoCommand},
		{Mods: ui.Cmd | ui.Shift, Key: ui.KeyEnter, Command: editor.NoCommand},
	}
	q.ed.OnCompletion = q.complete
	w.tabs = append(w.tabs, q)
	w.active = len(w.tabs) - 1
	return q
}

func (q *queryTab) title() string { return locales.TabQuery(q.n) }
func (q *queryTab) icon() *ui.SVG { return icCode }
func (q *queryTab) pending() int  { return 0 }

func (q *queryTab) complete(r editor.CompletionRequest) {
	src := q.ed.Buffer.String()
	var out []editor.Completion
	for _, it := range sqltext.Complete(src, r.Offset, r.Prefix, q.w.schema) {
		kind := editor.CompletionKeyword
		switch it.Kind {
		case sqltext.KindTable:
			kind = editor.CompletionType
		case sqltext.KindColumn:
			kind = editor.CompletionField
		}
		out = append(out, editor.Completion{Label: it.Label, Detail: it.Detail, Kind: kind})
	}
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
	q.runGated(stmts)
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
	var results []*database.QueryResult
	var err error
	w.sh.up.bg(func() {
		results, err = w.sh.app.Queries.ExecuteSQLBatch(reqID, w.cfg.ID, db, stmts, 1, database.DefaultPageSize)
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
		q.results = q.results[:0]
		for i, r := range results {
			g := newResultGrid(false)
			g.dbType = w.cfg.Type
			g.set(r.Columns, r.Rows)
			stmt := ""
			if i < len(stmts) {
				stmt = stmts[i]
			}
			q.results = append(q.results, &queryResult{stmt: stmt, res: r, g: g})
			w.sh.logSQL(db, stmt, time.Duration(r.Duration)*time.Millisecond, len(r.Rows), r.Error)
		}
		q.shown = max(0, len(q.results)-1)
		for i, r := range q.results {
			if r.res.Error != "" {
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

func (q *queryTab) view(c *ui.Context, p tokens) {
	th := c.Theme()
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
		})
	}).Padding(0)
	hairline(c, p.border)

	// 字号由 ⌘= / ⌘- 调整，所有编辑器共用
	if sz := q.w.sh.prefs.EditorFont; sz > 0 {
		q.ed.Font.Size = sz
	}
	ui.SplitVerticalPercent(c, &q.splitPct, func() {
		editor.View(c, q.ed).Grow(1).FillWidth().AutoFocus()
	}, func() {
		q.resultsView(c)
	}).Grow(1).FillWidth()

	q.history.view(c, q)
	_ = th

	switch {
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter):
		q.run(true)
	case c.Shortcut(ui.Cmd, ui.KeyEnter):
		q.run(false)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF):
		q.format()
	case c.Shortcut(ui.Cmd, ui.KeyY):
		q.history.toggle(q)
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
			for i := range labels {
				labels[i] = locales.QueryResultN(i + 1)
			}
			ui.Segmented(c, &q.shown, labels...).Margin(6, 10)
		}
		r := q.results[min(q.shown, len(q.results)-1)]
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
		if r.res.Error == "" {
			ui.Box(c).Height(1).Background(th.Border).FillWidth()
			ui.Row(c).Gap(12).Padding(5, 12).FillWidth().Children(func() {
				if len(r.res.Columns) > 0 {
					if int64(len(r.res.Rows)) < r.res.Total {
						ui.Text(c, locales.QueryTruncated(len(r.res.Rows), int(r.res.Total))).FontSize(12).TextColor(th.TextMuted)
					} else {
						ui.Text(c, locales.TableRows(len(r.res.Rows))).FontSize(12).TextColor(th.TextMuted)
					}
				}
				ui.Text(c, locales.StatusDuration(int(r.res.Duration))).FontSize(12).TextColor(th.TextMuted)
				ui.Spacer(c)
				if len(r.res.Columns) > 0 && r.stmt != "" {
					if ui.Button(c, locales.QueryExportResult()).Clicked() {
						q.w.sh.exportQuery(q.w, r.stmt)
					}
				}
			})
		}
	})
}
