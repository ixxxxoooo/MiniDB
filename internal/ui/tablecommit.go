package ui

import (
	"sort"

	"time"

	"minidb/internal/app"

	"minidb/locales"
)

// changeSet turns the edits into the arguments of CommitTableDataChanges.
func (t *tableTab) changeSet() (deletes, inserts []map[string]any, updates []app.RowUpdate) {
	pkOf := func(row int) map[string]any {
		pk := make(map[string]any, len(t.es.pk))
		for _, c := range t.es.pk {
			pk[c] = t.es.orig[row][c]
		}
		return pk
	}
	var delRows []int
	for r := range t.edits.deleted {
		delRows = append(delRows, r)
	}
	sort.Ints(delRows)
	for _, r := range delRows {
		deletes = append(deletes, pkOf(r))
	}
	for r := len(t.es.orig); r < len(t.g.rows); r++ {
		row := map[string]any{}
		for _, c := range t.g.cols {
			if v := t.g.rows[r][c.Name]; v != nil {
				row[c.Name] = v
			}
		}
		if len(row) > 0 {
			inserts = append(inserts, row)
		}
	}
	byRow := map[int]map[string]any{}
	for k, v := range t.edits.cells {
		if t.edits.deleted[k.row] {
			continue
		}
		if byRow[k.row] == nil {
			byRow[k.row] = map[string]any{}
		}
		byRow[k.row][k.col] = v
	}
	var rows []int
	for r := range byRow {
		rows = append(rows, r)
	}
	sort.Ints(rows)
	for _, r := range rows {
		updates = append(updates, app.RowUpdate{PrimaryKey: pkOf(r), Changes: byRow[r]})
	}
	return
}

// buildPreview fills in the statements a commit would run (for tests and logs).
func (t *tableTab) buildPreview() {
	deletes, inserts, updates := t.changeSet()
	rc := make([]app.RowChange, len(updates))
	for i, u := range updates {
		rc[i] = app.RowChange{PrimaryKey: u.PrimaryKey, Changes: u.Changes}
	}
	t.es.preview.stmts = app.PreviewChanges(t.w.cfg.Type, t.w.db, t.table, deletes, inserts, rc)
	t.es.preview.err = ""
}

func (t *tableTab) commit() {
	p := &t.es.preview
	deletes, inserts, updates := t.changeSet()
	w := t.w
	db := w.db
	n := len(deletes) + len(inserts) + len(updates)
	p.busy, p.err = true, ""
	stmts := append([]string(nil), p.stmts...)
	started := time.Now()
	var err error
	w.sh.up.bg(func() {
		err = w.sh.app.Queries.CommitTableDataChanges(w.cfg.ID, db, t.table, deletes, inserts, updates)
	}, func() {
		p.busy = false
		errText := ""
		if err != nil {
			errText = errMessage(err)
		}
		w.sh.logCommit(db, stmts, n, time.Since(started), errText)
		if err != nil {
			p.err = locales.EditCommitFail(errMessage(err))
			w.sh.toastErr(p.err)
			return
		}
		w.sh.toastOK(locales.EditCommitted(n))
		t.load()
	})
}

// commitNow writes the pending edits at once, as the Commit button and ⌘S do,
// without a dialog in between; the connection's Safe Mode may ask first.
func (t *tableTab) commitNow() {
	if t.es.editing != nil {
		t.commitEdit() // the cell being typed in counts too
	}
	if t.es.preview.busy || t.pendingCount() == 0 {
		return
	}
	t.buildPreview()
	t.w.sh.safeGate(t.w, t.es.preview.stmts, t.commit)
}
