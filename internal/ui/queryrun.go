package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/database"
	"minidb/internal/sqltext"
	"minidb/locales"
)

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
			q.failed = errMessage(err)
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
			qr := &queryResult{stmt: stmt, res: r, g: g, name: sqltext.ResultName(stmt), all: r.Rows, page: 1, pageSize: pageSize}
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

// inline 是这个标签的内联 AI 栏。
func (q *queryTab) inline() *inlineAI {
	q.ia.q = q
	return &q.ia
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

// format 美化 SQL：有选中的就只排选中的；没有选中就排光标所在的那条语句
// （和 TablePlus 一致）；光标不在任何语句里（比如落在空白处）时才排整个脚本。
// 缩进和关键字大小写按编辑器设置。
func (q *queryTab) format() {
	opt := q.w.sh.prefs.formatOptions()
	q.rewrite(func(sql string) string { return sqltext.FormatWith(sql, opt) }, true)
}

// uglify 把选中的、光标所在的那条语句（或整个脚本）压成一行。
func (q *queryTab) uglify() {
	q.rewrite(sqltext.Uglify, false)
}

// rewrite 用 fn 改写要处理的那一段：选中的文字，否则光标所在的语句，再否则整个脚本。
// perStmt 为真时，整个脚本逐条语句处理（美化要保留语句之间的原有分隔）。
func (q *queryTab) rewrite(fn func(string) string, perStmt bool) {
	src := q.ed.Buffer.String()
	if sel := q.ed.SelectedText(); sel != "" {
		s := q.ed.Selection()
		q.ed.Buffer.Edit(editor.Edit{Start: s.Start(), End: s.End(), Text: fn(sel)})
		return
	}
	if st, ok := sqltext.At(src, q.ed.Cursor()); ok {
		if out := fn(st.Text); out != st.Text {
			q.ed.Buffer.Edit(editor.Edit{Start: st.Start, End: st.End, Text: out})
		}
		return
	}
	var out string
	if perStmt {
		last := 0
		for _, st := range sqltext.Split(src) {
			out += src[last:st.Start] + fn(st.Text)
			last = st.End
		}
		out += src[last:]
	} else {
		out = fn(src)
	}
	if out != src {
		q.ed.Buffer.Set(out)
	}
}
