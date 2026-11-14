package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// loadSideQueries fills the Queries (favorites) and History lists.
func (w *Workspace) loadSideQueries() {
	if w.side.seg == sideItems {
		return
	}
	favs := w.side.seg == sideQueries
	q := w.filter
	var items []app.HistoryItem
	w.sh.up.bg(func() { items, _ = w.sh.app.History.List(w.cfg.ID, q, 300) }, func() {
		if favs != (w.side.seg == sideQueries) {
			return
		}
		if favs {
			var only []app.HistoryItem
			for _, it := range items {
				if it.Favorite {
					only = append(only, it)
				}
			}
			items = only
		}
		w.side.queries = items
	})
}

// queriesList lists favorites or history; a click opens the statement in a query tab.
func (w *Workspace) queriesList(c *ui.Context, p tokens) {
	items := w.side.queries
	if len(items) == 0 {
		msg := locales.HistoryEmpty()
		if w.side.seg == sideQueries {
			msg = locales.SidebarNoQueries()
		}
		ui.Text(c, msg).FontSize(font2XS).TextColor(p.fgMuted).Padding(16).TextAlign(ui.Center).FillWidth()
	}
	scrollbars(c, ui.Scroll(c.Key("sidequeries")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).Gap(1).FillWidth().Padding(0, 4).Children(func() {
			if w.side.seg == sideHistory {
				w.historyList(c, p, items)
				return
			}
			for _, it := range items {
				w.queryRow(c, p, it, true)
			}
		})
	})
}

func (w *Workspace) queryItemMenu(c *ui.Context, m *ui.Menu, it app.HistoryItem) {
	if m.Item(locales.SidebarOpenInQuery()).Chosen() {
		defer func() { w.newQuery(it.SQL) }()
	}
	if m.Item(locales.SidebarRunQuery()).Disabled(w.cfg.ReadOnly && !isReadOnlySQL(it.SQL)).Chosen() {
		defer func() { w.newQuery(it.SQL).runGated([]string{strings.TrimSpace(it.SQL)}) }()
	}
	if m.Item(locales.CommonCopy()).Chosen() {
		c.Services().WriteClipboard(it.SQL)
		defer w.sh.toastOK(locales.StatusCopied())
	}
	label := locales.HistoryFavorite()
	if it.Favorite {
		label = locales.HistoryUnfavorite()
	}
	if m.Item(label).Chosen() {
		defer func() {
			w.sh.up.bg(func() { _ = w.sh.app.History.ToggleFavorite(it.ID) }, w.loadSideQueries)
		}()
	}
	if m.Item(locales.QueryBindKeyword()).Chosen() {
		defer w.bindKeyword(it)
	}
	if it.Keyword != "" && m.Item(locales.QueryClearKeyword()).Chosen() {
		defer w.setKeyword(it.ID, "")
	}
	m.Separator()
	if m.Item(locales.CommonDelete()).Chosen() {
		defer func() {
			w.sh.up.bg(func() { _ = w.sh.app.History.Remove(it.ID) }, func() { w.loadSideQueries(); w.loadKeywords() })
		}()
	}
}

// clearHistory 在确认后清空当前连接的历史（收藏的语句保留）。
func (w *Workspace) clearHistory() {
	w.sh.askDanger(locales.HistoryClear(), locales.HistoryClearMsg(), locales.HistoryClear(), func() {
		w.sh.up.bg(func() { _ = w.sh.app.History.Clear(w.cfg.ID) }, func() { w.loadSideQueries(); w.loadKeywords() })
	})
}

func isReadOnlySQL(sql string) bool {
	return app.SafeModeRequirement(model.SafeModeAlert2, false, []string{sql}) == model.SafeNone
}

// sidebarFooter has the current database (a click switches it) and the "+" menu.
func (w *Workspace) sidebarFooter(c *ui.Context, p tokens) {
	// 与右边的底栏（tablebar.go bottomBar）同高：整条 sizeBtn+2，控件 sizeBtnSm，在条里垂直居中。
	// 没有分隔线，也没有切换数据库的按钮（顶栏有，⌘K 也行）：左边是搜索，右边是「…」
	ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn + 2).FillWidth().Shrink(0).Children(func() {
		// 放大镜：呼出（或收起）筛选框
		if toggleBtn(c, icSearch, locales.SidebarFilterPlaceholder()+w.sh.keyHint("side-filter"), w.side.showFilter || w.filter != "").Clicked() {
			if w.side.showFilter || w.filter != "" {
				w.side.showFilter, w.filter = false, ""
				w.loadSideQueries()
			} else {
				w.side.focusFilter = true
			}
		}
		ui.Spacer(c)
		tipBtn(c.Key("sideplus"), icMore, locales.ConnMore()).Size(sizeBtnSm, sizeBtnSm).PopoverMenuButton(func(m *ui.Menu) {
			if m.Item(locales.SidebarNewQuery()).Shortcut(ui.Cmd, ui.KeyT).Chosen() {
				defer func() { w.newQuery("") }()
			}
			if m.Item(locales.TableNew()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer w.newTable()
			}
			if m.Item(locales.OverviewTitle()).Chosen() {
				defer w.openOverview()
			}
			if m.Item(locales.CommonRefresh()).Shortcut(ui.Cmd, ui.KeyR).Chosen() {
				defer w.loadTables()
			}
			if w.side.seg == sideHistory && m.Item(locales.HistoryClear()).Chosen() {
				defer w.clearHistory()
			}
			m.Separator()
			if m.Item(locales.SearchDb()).Chosen() {
				defer w.sh.openDatabaseSearch()
			}
			if m.Item(locales.ExportTablesMenu()).Chosen() {
				defer func() { w.sh.exportTables(w) }()
			}
			if m.Item(locales.ImportFileMenu()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer func() { w.sh.importFile(w) }()
			}
			m.Separator()
			if app.SupportsUsers(w.cfg.Type) && m.Item(locales.UsersMenu()).Chosen() {
				defer func() { w.sh.openUsers() }()
			}
			if m.Item(locales.DumpBackupMenu()).Chosen() {
				defer func() { w.sh.openBackup() }()
			}
			if app.CapsOf(w.cfg.Type).Network && m.Item(locales.DumpRestoreMenu()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer func() { w.sh.openRestore() }()
			}
			m.Separator()
			if app.CapsOf(w.cfg.Type).CreateDatabase && m.Item(locales.DbNew()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer func() { w.sh.newDatabase(w) }()
			}
		}, ui.PopoverMenuButtonOptions{})
	})
}
