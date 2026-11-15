package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

type paletteItem struct {
	kind   string
	label  string
	hint   string
	icon   *ui.SVG
	dbType model.DBType // a connection shows its database tile instead of the icon
	keys   string       // the shortcut, shown at the right
	run    func()
	newTab func() // ⌘↵: the same, in a tab of its own
	// 右侧预览要用的：历史条目、连接配置、表名（各自只有对应种类才有）
	hist  *app.HistoryItem
	cfg   *model.ConnectionConfig
	table string
}

// paletteMatch is how Open Anything matches: the text anywhere in the label,
// at its start with a leading ^, at its end with a trailing $, or all of it
// with both. Case does not matter.
func paletteMatch(label, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	label = strings.ToLower(label)
	start, end := strings.HasPrefix(q, "^"), strings.HasSuffix(q, "$") && len(q) > 0
	q = strings.TrimSuffix(strings.TrimPrefix(q, "^"), "$")
	switch {
	case start && end:
		return label == q
	case start:
		return strings.HasPrefix(label, q)
	case end:
		return strings.HasSuffix(label, q)
	}
	return strings.Contains(label, q)
}

type palette struct {
	open    bool
	query   string
	sel     int
	cat     int // 分类筛选：0 全部，1 表，2 命令，3 历史，4 连接
	history []app.HistoryItem
	// 右侧预览的表字段：键是 连接/库/表，值取回来的列；loading 记着正在取的，免得每帧重复取
	cols    map[string][]model.ColumnInfo
	loading map[string]bool
}

func (s *Shell) openPalette() {
	s.pal = palette{open: true}
	if w := s.activeWorkspace(); w != nil {
		var items []app.HistoryItem
		s.up.bg(func() { items, _ = s.app.History.List(w.cfg.ID, "", 100) }, func() { s.pal.history = items })
	}
}

// paletteItems lists what the palette can reach, filtered by the query.
func (s *Shell) paletteItems() []paletteItem {
	query, cat := s.pal.query, s.pal.cat
	if strings.HasPrefix(strings.TrimSpace(query), ">") { // 「>」开头只看命令
		query, cat = strings.TrimPrefix(strings.TrimSpace(query), ">"), 2
	}
	match := func(text string) bool { return paletteMatch(text, query) }
	var out []paletteItem
	add := func(it paletteItem) {
		if match(it.label) {
			out = append(out, it)
		}
	}
	cmds := []paletteItem{
		{kind: locales.PaletteCommand(), label: locales.MenuNewConnection(), icon: icPlug, keys: s.cmdKeys("new-conn"), run: s.cmdNewConnection},
		{kind: locales.PaletteCommand(), label: locales.MenuSettings(), icon: icSettings, keys: s.cmdKeys("settings"), run: s.cmdSettings},
	}
	if w := s.activeWorkspace(); w != nil {
		cmds = append(cmds,
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuNewQuery(), icon: icQuery, keys: s.cmdKeys("new-query"), run: s.cmdNewQuery},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuToggleAI(), icon: icSparkle, keys: s.cmdKeys("ai"), run: s.cmdToggleAI},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuRefresh(), icon: icRefresh, keys: s.cmdKeys("refresh"), run: s.cmdRefresh},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuRefreshTab(), icon: icRefresh, keys: s.cmdKeys("refresh-tab"), run: s.cmdRefreshTab},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuExport(), icon: icDownload, keys: s.cmdKeys("export"), run: s.cmdExport},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ExportTablesMenu(), icon: icDownload, run: func() { s.exportTables(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ImportFileMenu(), icon: icDownload, run: func() { s.importFile(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.DumpBackupMenu(), icon: icDownload, run: s.openBackup},
			paletteItem{kind: locales.PaletteCommand(), label: locales.DumpRestoreMenu(), icon: icDownload, run: s.openRestore},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarConsole(), icon: icConsole, keys: s.cmdKeys("console"), run: func() { s.consoleOpen = !s.consoleOpen }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.SearchDb(), icon: icSearch, run: s.openDatabaseSearch},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarSwitchDatabase(), icon: icDatabase, keys: s.cmdKeys("switcher"), run: s.openSwitcher},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarReconnect(), icon: icPlug, keys: s.cmdKeys("reconnect"), run: func() { s.reconnect(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarDisconnect(), icon: icUnplug, keys: s.cmdKeys("disconnect"), run: func() { s.closeConnection(w.cfg.ID) }},
		)
		if w.txSupported() {
			cmds = append(cmds,
				paletteItem{kind: locales.PaletteCommand(), label: locales.TxManual(), icon: icCommit, keys: s.cmdKeys("tx-toggle"), run: s.cmdTxToggle},
				paletteItem{kind: locales.PaletteCommand(), label: locales.TxCommit(), icon: icCheck, keys: s.cmdKeys("tx-commit"), run: s.cmdTxCommit},
				paletteItem{kind: locales.PaletteCommand(), label: locales.TxRollback(), icon: icUndo, keys: s.cmdKeys("tx-rollback"), run: s.cmdTxRollback},
			)
		}
		if app.SupportsUsers(w.cfg.Type) {
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.UsersMenu(), icon: icUser, run: s.openUsers})
		}
		if app.CanER(w.cfg.Type) {
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.ErTitle(), icon: icLink, run: func() { w.openER("") }})
		}
		if app.SupportsMonitor(w.cfg.Type) {
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.MonitorTitle(), icon: icActivity, keys: s.cmdKeys("monitor"), run: s.cmdMonitor})
		}
		if app.SupportsProcessList(w.cfg.Type) {
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.MenuProcesses(), icon: icTerminal, keys: s.cmdKeys("processes"), run: s.cmdProcesses})
		}
		// 以下几项原来只有快捷键或藏在菜单里，命令面板里补上，搜一下就能找到
		cmds = append(cmds,
			paletteItem{kind: locales.PaletteCommand(), label: locales.OverviewTitle(), icon: icDatabase, run: w.openOverview},
			paletteItem{kind: locales.PaletteCommand(), label: locales.SettingsShortcuts(), icon: icKeyboard, run: func() { s.openSettings(setShortcuts) }},
		)
		if len(s.spaces) > 1 {
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.WsAllTitle(), icon: icDatabase, keys: s.cmdKeys("workspaces"), run: s.openWorkspaces})
		}
		if s.activeTable() != nil {
			cmds = append(cmds,
				paletteItem{kind: locales.PaletteCommand(), label: locales.KmShowData(), icon: icTable, keys: s.cmdKeys("show-data"), run: s.commandRun("show-data")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.KmShowStructure(), icon: icTable, keys: s.cmdKeys("show-structure"), run: s.commandRun("show-structure")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.ScJumpColumn(), icon: icSearch, keys: s.cmdKeys("jump-column"), run: s.commandRun("jump-column")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.ScColumns(), icon: icSliders, keys: s.cmdKeys("columns"), run: s.commandRun("columns")},
			)
		}
		if s.activeQuery() != nil {
			cmds = append(cmds,
				paletteItem{kind: locales.PaletteCommand(), label: locales.MenuRun(), icon: icQuery, keys: s.cmdKeys("run"), run: s.commandRun("run")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.MenuFormat(), icon: icQuery, keys: s.cmdKeys("format"), run: s.commandRun("format")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.ScExplain(), icon: icQuery, keys: s.cmdKeys("explain"), run: s.commandRun("explain")},
				paletteItem{kind: locales.PaletteCommand(), label: locales.KmCommentLine(), icon: icQuery, keys: s.cmdKeys("comment-line"), run: s.commandRun("comment-line")},
			)
		}
		for _, mode := range model.SafeModes {
			mode := mode
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.SafeTitle() + ": " + safeModeLabel(mode), hint: safeModeDesc(mode), icon: safeModeIcon(mode), run: func() { s.setSafeMode(w, mode) }})
		}
		for _, t := range w.tables {
			name := t.Name
			icon := objectIcon(KindTable)
			if isView(t) {
				icon = objectIcon(KindView)
			}
			add(paletteItem{kind: locales.PaletteTable(), label: name, hint: tableHint(w.dbLabel(), t), icon: icon, table: name, run: func() { w.openTable(name) }, newTab: func() { w.openTableNewTab(name) }})
		}
		for _, h := range s.pal.history {
			h, sql := h, h.SQL
			add(paletteItem{kind: locales.PaletteHistory(), label: firstLines(sql, 1), icon: icHistory, hist: &h, run: func() { w.newQuery(sql) }})
		}
	}
	for _, cfg := range s.conns {
		cfg := cfg
		add(paletteItem{kind: locales.PaletteConnection(), label: cfg.Name, hint: connSubtitle(cfg), icon: icDatabase, dbType: cfg.Type, cfg: &cfg, run: func() { s.openConnection(cfg) }})
	}
	for _, it := range cmds {
		add(it)
	}
	out = paletteGroup(out, cat)
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func (s *Shell) paletteView(c *ui.Context) {
	p := &s.pal
	if !p.open {
		return
	}
	items := s.paletteAskAI(s.paletteItems())
	if p.sel >= len(items) {
		p.sel = max(0, len(items)-1)
	}
	choose := func() {
		if p.sel < len(items) {
			run := items[p.sel].run
			p.open = false
			run()
		}
	}
	pp := s.tokens(c)
	s.popupKey = func(k ui.TextKey) bool {
		switch {
		case k.Key == ui.KeyTab && k.Mods == 0: // complete to the chosen item
			if p.sel < len(items) {
				p.query = items[p.sel].label
				k.SetText(p.query)
			}
			return true
		case k.Key == ui.KeyEnter && k.Mods == ui.Cmd: // open in a tab of its own
			if p.sel < len(items) {
				run := items[p.sel].run
				if items[p.sel].newTab != nil {
					run = items[p.sel].newTab
				}
				p.open = false
				run()
			}
			return true
		}
		return false
	}
	if p.sel < len(items) {
		s.palettePrefetch(items[p.sel])
	}
	s.popup(c, &p.open, sizePalette, &p.query, locales.PaletteHint(), func() { p.sel = 0 }, choose, func(d int) { p.sel = min(max(p.sel+d, 0), max(len(items)-1, 0)) }, func() {
		s.paletteChips(c, pp)
		ui.Row(c.Key("pal-body")).FillWidth().Height(sizePalBody).Children(func() { // 设计稿：左边分组列表，右边预览当前项
			ui.Column(c.Key("pal-left")).Width(sizePalList).FillHeight().Shrink(0).Children(func() { s.paletteList(c, pp, items, choose) })
			ui.Box(c).Width(1).FillHeight().Background(pp.borderSubtle)
			var cur *paletteItem
			if p.sel < len(items) {
				cur = &items[p.sel]
			}
			s.palettePreview(c, pp, cur)
		})
		s.paletteFooter(c, pp)
	})
	switch {
	case c.Shortcut(0, ui.KeyDown):
		p.sel = min(p.sel+1, len(items)-1)
	case c.Shortcut(0, ui.KeyUp):
		p.sel = max(p.sel-1, 0)
	}
}

// paletteIconColor gives the icons of tables and queries their hue, the rest
// the secondary grey.
func paletteIconColor(p tokens, ic *ui.SVG) ui.Color {
	switch ic {
	case icTable:
		return p.iconTable
	case icQuery:
		return p.iconQuery
	case icView:
		return p.iconView
	}
	return p.fgSecondary
}
