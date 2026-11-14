package ui

import (
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"

	"minidb/locales"
)

// tableMenu is the context menu of a table in the sidebar.
func (w *Workspace) tableMenu(m *ui.Menu, name string) {
	view := w.isViewName(name)
	if view && m.Item(locales.ObjEditView()).Chosen() {
		defer w.openDefinition(KindView, name)
	}
	if m.Item(locales.TableOpen()).Chosen() {
		defer w.openTable(name)
	}
	if m.Item(locales.OverviewTitle()).Chosen() {
		defer w.openOverview()
	}
	if m.Item(locales.TableOpenNewTab()).Chosen() {
		defer w.openTableNewTab(name)
	}
	if m.Item(locales.TableStructure()).Chosen() {
		defer w.openTableMode(name, 1)
	}
	if m.Item(locales.TableDDL()).Chosen() {
		defer w.openTableMode(name, 2)
	}
	if m.Item(locales.TableOpenQuery()).Chosen() {
		defer func() {
			sql, _ := w.sh.app.Queries.DefaultSelectTableSQL(w.cfg.ID, name, 100)
			w.newQuery(sql)
		}()
	}
	m.Separator()
	pin := locales.TablePin()
	if w.isPinned(name) {
		pin = locales.TableUnpin()
	}
	if m.Item(pin).Chosen() {
		defer w.togglePin(name)
	}
	if m.Item(locales.TableCopyName()).Chosen() {
		defer func() { w.sh.copyText(name) }()
	}
	if m.Item(locales.TableCopyScript()).Chosen() {
		defer func() { w.copyCreateScript(name) }()
	}
	if !view && m.Item(locales.TableClone()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.cloneTable(name)
	}
	m.Separator()
	if m.Item(locales.TableExport()).Chosen() {
		defer w.sh.exportTable(w, name)
	}
	if view {
		m.Separator()
		if m.Item(locales.ObjDropView()).Disabled(w.cfg.ReadOnly).Chosen() {
			defer w.dropObject(KindView, name)
		}
		return
	}
	if m.Item(locales.TableImport()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.sh.importInto(w, name)
	}
	m.Separator()
	if m.Item(locales.TableTruncate()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.truncateTable(name)
	}
	if m.Item(locales.TableDrop()).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.dropTable(name)
	}
}

// copyCreateScript puts a table's CREATE statement on the clipboard.
func (w *Workspace) copyCreateScript(name string) {
	db := w.db
	var ddl string
	var err error
	w.sh.up.bg(func() { ddl, err = w.sh.app.Databases.GetDDL(w.cfg.ID, db, name) }, func() {
		if err != nil {
			w.sh.toastErr(errMessage(err))
			return
		}
		w.sh.copyText(ddl)
	})
}

func (w *Workspace) openTableMode(name string, mode int) {
	w.openTable(name)
	if tt, ok := w.tabs[w.active].(*tableTab); ok {
		tt.mode = mode
		tt.modeChanged()
	}
}

// tableOpOpts 是清空 / 删除表对话框里的复选项：按数据库能给的来，没有可选项的库返回空。
//
// 参数：truncate 为真是清空表，否则是删除表。
// 返回值：复选项列表（label 已写好，on 为假）。
func (w *Workspace) tableOpOpts(truncate bool) (cascade, restart *confirmOpt, all []*confirmOpt) {
	tc, ri, dc := app.TableOpSupport(w.cfg.Type)
	if truncate {
		if ri {
			restart = &confirmOpt{label: locales.TableOptRestartIdentity()}
			all = append(all, restart)
		}
		if tc {
			cascade = &confirmOpt{label: locales.TableOptCascade()}
			all = append(all, cascade)
		}
		return
	}
	if dc {
		cascade = &confirmOpt{label: locales.TableOptCascade()}
		all = append(all, cascade)
	}
	return
}

// truncateTable 确认后清空表；按数据库给出 Restart identity、Cascade 等选项（PostgreSQL）。
func (w *Workspace) truncateTable(name string) {
	cascade, restart, all := w.tableOpOpts(true)
	run := func() {
		var o app.TableOpOptions
		if cascade != nil {
			o.Cascade = cascade.on
		}
		if restart != nil {
			o.RestartIdentity = restart.on
		}
		w.sh.safeGatePassword(w, []string{app.BuildTruncateTableSQL(w.cfg.Type, name, o)}, func() {
			db := w.db
			var err error
			w.sh.up.bg(func() { err = w.sh.app.Databases.TruncateTableWithOptions(w.cfg.ID, db, name, o) }, func() {
				if err != nil {
					w.sh.toastErr(errMessage(err))
					return
				}
				w.sh.toastOK(locales.TableDone())
				for _, tb := range w.tabs {
					if tt, ok := tb.(*tableTab); ok && tt.table == name {
						tt.load()
					}
				}
			})
		})
	}
	w.sh.askDangerOpts(locales.TableTruncateTitle(), locales.TableTruncateMsg(name), locales.TableTruncate(), all, run)
}

// dropTable 确认后删除表；PostgreSQL 可勾选 Cascade。
func (w *Workspace) dropTable(name string) {
	cascade, _, all := w.tableOpOpts(false)
	run := func() {
		var o app.TableOpOptions
		if cascade != nil {
			o.Cascade = cascade.on
		}
		w.sh.safeGatePassword(w, []string{app.BuildDropTableSQL(w.cfg.Type, name, o)}, func() {
			db := w.db
			var err error
			w.sh.up.bg(func() { err = w.sh.app.Databases.DropTableWithOptions(w.cfg.ID, db, name, o) }, func() {
				if err != nil {
					w.sh.toastErr(errMessage(err))
					return
				}
				for i := len(w.tabs) - 1; i >= 0; i-- {
					if tt, ok := w.tabs[i].(*tableTab); ok && tt.table == name {
						tt.revert()
						w.closeTab(i)
					}
				}
				w.sh.toastOK(locales.TableDone())
				w.loadTables()
			})
		})
	}
	w.sh.askDangerOpts(locales.TableDropTitle(), locales.TableDropMsg(name), locales.TableDrop(), all, run)
}
