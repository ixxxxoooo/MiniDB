package ui

import (
	"fmt"
	"strings"
	"time"

	"minidb/internal/app"
)

// 数据页的取数：当前页、总行数、预取其他子视图，以及丢弃未保存修改前的确认。

func (t *tableTab) size() int {
	var n int
	fmt.Sscan(t.pageSize, &n)
	if n <= 0 {
		return 200
	}
	return n
}

// load fetches the current page; pending edits are dropped, so callers that
// can lose them go through reload.
func (t *tableTab) load() {
	if t.isNew {
		return
	}
	t.started = true
	t.loadStart = time.Now()
	t.seq++
	seq := t.seq
	t.loading, t.err = true, ""
	w := t.w
	sorts := append([]app.Sort(nil), t.sortList()...)
	filters, raw, page, size := append([]app.Filter(nil), t.applied...), t.appliedRaw, t.page, t.size()
	db := w.db
	logged := t.tableSelectSQL() // 控制台日志里记下这一页对应的 SELECT
	var res *app.QueryResult
	var err error
	w.sh.up.bg(func() {
		if strings.TrimSpace(raw) == "" {
			// rows first: COUNT(*) can take long on big tables, so it follows
			res, err = w.sh.app.Queries.QueryTableRows(w.cfg.ID, db, t.table, page, size, filters, sorts)
			return
		}
		res, err = w.sh.app.Queries.QueryTableDataWithRawInput(w.cfg.ID, db, t.table, page, size, filters, sorts, raw)
	}, func() {
		if seq != t.seq {
			return
		}
		t.loading = false
		switch {
		case err != nil:
			t.err = errMessage(err)
			w.sh.logSQL(db, logged, 0, -1, t.err)
		case res != nil && res.Error != "":
			t.err = res.Error
			w.sh.logSQL(db, logged, resultDur(res), -1, t.err)
		case res != nil:
			w.sh.logSQL(db, logged, resultDur(res), len(res.Rows), "")
			t.total, t.dur = res.Total, resultDur(res)
			t.g.offset = (page - 1) * size
			t.setRows(res.Columns, res.Rows)
			if res.Total < 0 {
				t.countRows(seq, filters)
			}
			t.prefetch()
		}
	})
}

// prefetch loads what the other sub-views show (structure, DDL, doc) once the
// rows are on screen, so switching to them is instant, with nothing to wait for.
func (t *tableTab) prefetch() {
	if !t.st.loaded && !t.st.loading && t.st.err == "" {
		t.st.load(t)
	}
	if !t.ddl.loaded && !t.ddl.loading {
		t.ddl.load(t)
	}
	if !t.doc.loaded && !t.doc.loading {
		t.doc.load(t)
	}
}

// countRows fills in the total after the rows are on screen.
func (t *tableTab) countRows(seq int, filters []app.Filter) {
	w, db := t.w, t.w.db
	var n int64
	var err error
	w.sh.up.bg(func() { n, err = w.sh.app.Queries.CountTableData(w.cfg.ID, db, t.table, filters) }, func() {
		if seq != t.seq {
			return
		}
		if err != nil {
			n = int64(len(t.g.rows)) // no total: show what the page holds
			if t.page > 1 {
				n += int64(t.g.offset)
			}
		}
		t.total = n
	})
}

// reload loads again after asking, when edits would be lost.
func (t *tableTab) reload(change func()) {
	if t.isNew {
		return
	}
	t.w.sh.discardGuard(t.pending(), func() {
		if change != nil {
			change()
		}
		t.load()
	})
}
