package ui

import (
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"

	"minidb/locales"
)

// tableMenu is the context menu of a table in the sidebar.
func (w *Workspace) tableMenu(m *ui.Menu, name string) {
	view := w.isViewName(name)
	if view && menuItem(m, locales.ObjEditView(), icPencil).Chosen() {
		defer w.openDefinition(KindView, name)
	}
	if menuItem(m, locales.TableOpen(), icTable).Chosen() {
		defer w.openTable(name)
	}
	if menuItem(m, locales.OverviewTitle(), icView).Chosen() {
		defer w.openOverview()
	}
	if app.CanER(w.cfg.Type) && !view && menuItem(m, locales.ErOpenMenu(), icLink).Chosen() {
		defer w.openER(name)
	}
	if menuItem(m, locales.TableOpenNewTab(), icPlus).Chosen() {
		defer w.openTableNewTab(name)
	}
	if menuItem(m, locales.TableStructure(), icColumns).Chosen() {
		defer w.openTableMode(name, 1)
	}
	if menuItem(m, locales.TableDDL(), icCode).Chosen() {
		defer w.openTableMode(name, 2)
	}
	if menuItem(m, locales.TableOpenQuery(), icTerminal).Chosen() {
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
	if menuItem(m, pin, icPin).Chosen() {
		defer w.togglePin(name)
	}
	if menuItem(m, locales.TableCopyName(), icCopy).Chosen() {
		defer func() { w.sh.copyText(name) }()
	}
	if menuItem(m, locales.TableCopyScript(), icCode).Chosen() {
		defer func() { w.copyCreateScript(name) }()
	}
	if !view && menuItem(m, locales.TableClone(), icCopy).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.cloneTable(name)
	}
	m.Separator()
	if menuItem(m, locales.TableExport(), icDownload).Chosen() {
		defer w.sh.exportTable(w, name)
	}
	if view {
		m.Separator()
		if dangerItem(m, locales.ObjDropView(), icTrash).Disabled(w.cfg.ReadOnly).Chosen() {
			defer w.dropObject(KindView, name)
		}
		return
	}
	if menuItem(m, locales.TableImport(), icUpload).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.sh.importInto(w, name)
	}
	m.Separator()
	if dangerItem(m, locales.TableTruncate(), icTrash).Disabled(w.cfg.ReadOnly).Chosen() {
		defer w.truncateTable(name)
	}
	if dangerItem(m, locales.TableDrop(), icTrash).Disabled(w.cfg.ReadOnly).Chosen() {
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
	w.sh.askDangerOpts(locales.TableDropTitle(name), locales.TableDropMsg(), locales.TableDrop(), all, run)
	if cascade == nil { // 有 Cascade 勾选时语句随勾选变化，不预先摆出
		w.sh.confirm.code = app.BuildDropTableSQL(w.cfg.Type, name, app.TableOpOptions{}) // 把将要执行的语句摆出来（设计稿）
	}
}
