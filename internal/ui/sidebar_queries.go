package ui

import (
	"errors"
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
	s := w.side.hist.search(w.filter, favs)
	var items []app.HistoryItem
	var err error
	w.sh.up.bg(func() { items, err = w.sh.app.History.Search(w.cfg.ID, s) }, func() {
		if favs != (w.side.seg == sideQueries) {
			return
		}
		w.side.hist.bad = errors.Is(err, app.ErrHistoryPattern)
		if w.side.hist.bad {
			return // 正则写到一半：保留上一次的结果，等写完整
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

// sideMoreMenu 是侧栏当前库一行右边「更多」菜单的主体（设计稿：侧栏底栏取消，新建表、导入导出备份、监控、用户等都收进这里）。
// 参数：m 是菜单。安全级别的单选在后面由 dbMoreMenu 接着画。
func (w *Workspace) sideMoreMenu(m *ui.Menu) {
	if m.Item(locales.SidebarNewQuery()).Shortcut(ui.Cmd, ui.KeyT).Chosen() {
		defer func() { w.newQuery("") }()
	}
	if m.Item(locales.TableNew()).Disabled(w.cfg.ReadOnly || !app.SupportsVisualCreateTable(w.cfg.Type)).Chosen() {
		defer w.newTable()
	}
	if m.Item(locales.OverviewTitle()).Chosen() {
		defer w.openOverview()
	}
	if m.Item(locales.ErOpenMenu()).Disabled(!app.CanER(w.cfg.Type)).Chosen() {
		defer func() { w.openER("") }()
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
	// 监控与进程：和顶栏的监控按钮、⌥⌘M / ⌘. 是同一个窗口的不同页签
	if len(monitorTabs(w)) > 0 {
		if menuKey(m.Item(locales.MonitorTitle()), w.sh.chordOf("monitor")).Chosen() {
			defer w.sh.cmdMonitor()
		}
	}
	if app.SupportsProcessList(w.cfg.Type) {
		if menuKey(m.Item(locales.MenuProcesses()), w.sh.chordOf("processes")).Chosen() {
			defer w.sh.cmdProcesses()
		}
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
}

// menuKey 给弹出菜单项标上命令当前的快捷键（用户改过键也跟着变；没绑定就不标）。
func menuKey(it *ui.MenuItem, ch chord) *ui.MenuItem {
	if ch.valid() {
		return it.Shortcut(ch.mods, ch.key)
	}
	return it
}

// toggleSideFilter 把键盘交给侧栏的筛选框（必要时先展开侧栏）。筛选框一直都在，所以按一次是聚焦；
// 已经聚焦在框里并且有筛选词时再按一次，清掉筛选词。
func (w *Workspace) toggleSideFilter() {
	w.sidebarHidden = false
	if w.side.filterOn && w.filter != "" {
		w.filter = ""
		w.loadSideQueries()
		return
	}
	w.side.focusFilter = true
}
