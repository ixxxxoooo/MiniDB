package ui

import (
	"time"

	"minidb/locales"
)

// buildStructurePreview fills in the DDL a commit would run (for tests and logs).
func (t *tableTab) buildStructurePreview() {
	s := &t.st
	var stmts []string
	var err error
	w := t.w
	objs := s.obj.edits(t.table)
	objs.RenameTo = s.renameTo()
	w.sh.up.bg(func() {
		if t.isNew {
			stmts, err = t.newTableStatements()
			return
		}
		stmts, err = w.sh.app.Databases.PreviewTableStructureChanges(w.cfg.ID, t.table, s.working(), s.orig, s.workingIndexes(), objs)
	}, func() {
		s.preview.stmts, s.preview.err = stmts, ""
		if err != nil {
			s.preview.err = errMessage(err)
		}
	})
}

// applyStructure 执行已经算好的语句 stmts（commitStructure 给的）：成功失败都逐条记进控制台日志。
func (t *tableTab) applyStructure(stmts []string) {
	s := &t.st
	w := t.w
	db := w.db
	s.preview.busy, s.preview.err = true, ""
	working, idx, objs := s.working(), s.workingIndexes(), s.obj.edits(t.table)
	objs.RenameTo = s.renameTo()
	started := time.Now()
	var err error
	w.sh.up.bg(func() {
		err = w.sh.app.Databases.ApplyTableStructureChanges(w.cfg.ID, db, t.table, working, s.orig, idx, objs)
	}, func() {
		s.preview.busy = false
		errText := ""
		if err != nil {
			errText = errMessage(err)
		}
		w.sh.logCommit(db, stmts, 0, time.Since(started), errText)
		if err != nil {
			s.preview.err = locales.StructApplyFail(errMessage(err))
			w.sh.toastErr(s.preview.err)
			return
		}
		w.sh.toastOK(locales.StructApplied())
		if objs.RenameTo != "" {
			w.renamePinned(t.table, objs.RenameTo)
			t.table, t.g.table = objs.RenameTo, objs.RenameTo
		}
		t.ddl = ddlState{}
		t.colSigReset()
		s.load(t)
		t.loadStructure()
		t.load()
		w.loadTables()
	})
}

// commitStructure applies the structure changes at once, without a dialog;
// the connection's Safe Mode may ask first.
func (t *tableTab) commitStructure() {
	s := &t.st
	if s.preview.busy || s.changeCount() == 0 || t.w.cfg.ReadOnly {
		return
	}
	if t.isNew {
		t.commitNewTable()
		return
	}
	// 先按当前的改动算出真正要执行的语句：安全模式确认框里显示的、执行后记进控制台日志的，都是它们
	// （以前用的是预览里留下的旧语句，没开过预览就记成「COMMIT 0 changes」，改了什么日志里看不到）
	w := t.w
	objs := s.obj.edits(t.table)
	objs.RenameTo = s.renameTo()
	working, idx := s.working(), s.workingIndexes()
	var stmts []string
	var err error
	w.sh.up.bg(func() {
		stmts, err = w.sh.app.Databases.PreviewTableStructureChanges(w.cfg.ID, t.table, working, s.orig, idx, objs)
	}, func() {
		if err != nil {
			s.preview.err = locales.StructApplyFail(errMessage(err))
			w.sh.toastErr(s.preview.err)
			return
		}
		if len(stmts) == 0 {
			return
		}
		s.preview.stmts = stmts
		w.sh.safeGate(w, stmts, func() { t.applyStructure(stmts) })
	})
}

// colSigReset forces the data grid to rebuild its columns after a structure change.
func (t *tableTab) colSigReset() { t.g.sig = "" }
