package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

func (s *Shell) importSQLView(c *ui.Context, p tokens) {
	pv := s.imp.sql
	ui.Text(c, locales.ImportSQLCount(pv.Count)).FontWeight(500)
	dlgBox(c, p).FillWidth().Children(func() {
		ui.Column(c).Padding(8).Gap(4).FillWidth().Children(func() {
			for i, st := range pv.Head {
				ui.Text(c.Key(fmt.Sprint(i)), oneLine(st)).SingleLine().FontSize(font2XS).Font(mono).TextColor(p.fgSecondary)
			}
		})
	})
	if len(pv.Risky) > 0 {
		ui.Text(c, locales.ImportSQLRisky(len(pv.Risky))).TextColor(p.danger).FontSize(font2XS)
	}
	ui.Text(c, locales.ImportSQLNote()).TextColor(p.fgMuted).FontSize(font2XS)
}

// confirmImport 过安全模式的关口，再开始。
func (s *Shell) confirmImport() {
	d := &s.imp
	if d.sql != nil {
		stmts := append(append([]string{}, d.sql.Head...), d.sql.Risky...)
		s.safeGate(d.w, stmts, s.startSQLImport)
		return
	}
	table := d.table
	if d.newTable {
		table = strings.TrimSpace(d.newName)
	}
	if table == "" {
		d.err = locales.ImportNeedTable()
		return
	}
	stmts := []string{"INSERT INTO " + table}
	switch {
	case d.newTable:
		stmts = []string{"CREATE TABLE " + table}
	case d.conflict == app.ConflictReplace:
		stmts = []string{"REPLACE INTO " + table}
	case d.conflict == app.ConflictUpdate:
		stmts = []string{"UPDATE " + table}
	}
	s.safeGate(d.w, stmts, s.startImport)
}

func (s *Shell) startSQLImport() {
	d := &s.imp
	req := app.SQLImportRequest{ConnID: d.w.cfg.ID, Database: d.w.db, FilePath: d.path}
	d.busy = true
	var err error
	var id string
	s.up.bg(func() { id, err = s.app.Imports.StartSQLImport(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.xfer.begin(id)
		s.toastOK(locales.ImportStarted())
	})
}

func (s *Shell) startImport() {
	d := &s.imp
	req := app.ImportRequest{
		ConnID: d.w.cfg.ID, Database: d.w.db, Table: d.table, FilePath: d.path, Parse: d.opts,
		Import: app.ImportOptions{Atomic: d.atomic, EmptyAsNull: d.emptyAsNull, Conflict: d.conflict},
	}
	if d.newTable {
		req.Table = strings.TrimSpace(d.newName)
		req.Import.Conflict = app.ConflictStop
		for i, nc := range d.newCols {
			if name := strings.TrimSpace(nc.name); name != "" {
				req.NewColumns = append(req.NewColumns, app.ImportNewColumn{Source: i, Name: name, Kind: nc.kind})
			}
		}
		if len(req.NewColumns) == 0 {
			d.err = locales.ImportNeedMap()
			return
		}
	} else {
		for i, tgt := range d.target {
			if tgt != "" {
				req.Columns = append(req.Columns, app.ImportColumn{Source: i, Target: tgt})
			}
		}
		if len(req.Columns) == 0 {
			d.err = locales.ImportNeedMap()
			return
		}
	}
	d.busy = true
	var err error
	var id string
	s.up.bg(func() { id, err = s.app.Imports.StartImport(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.xfer.begin(id)
		s.toastOK(locales.ImportStarted())
	})
}
