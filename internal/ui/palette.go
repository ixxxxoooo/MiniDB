package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/locales"
)

type paletteItem struct {
	kind   string
	label  string
	hint   string
	icon   *ui.SVG
	dbType string // a connection shows its database tile instead of the icon
	keys   string // the shortcut, shown at the right
	run    func()
	newTab func() // ⌘↵: the same, in a tab of its own
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
	history []app.HistoryItem
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
	match := func(text string) bool { return paletteMatch(text, s.pal.query) }
	var out []paletteItem
	add := func(it paletteItem) {
		if match(it.label) {
			out = append(out, it)
		}
	}
	cmds := []paletteItem{
		{kind: locales.PaletteCommand(), label: locales.MenuNewConnection(), icon: icPlug, keys: "⌘N", run: s.cmdNewConnection},
		{kind: locales.PaletteCommand(), label: locales.MenuSettings(), icon: icSettings, keys: "⌘,", run: s.cmdSettings},
	}
	if w := s.activeWorkspace(); w != nil {
		cmds = append(cmds,
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuNewQuery(), icon: icCode, keys: "⌘T", run: s.cmdNewQuery},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuToggleAI(), icon: icSparkle, keys: "⌘I", run: s.cmdToggleAI},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuRefresh(), icon: icRefresh, keys: "⌘R", run: s.cmdRefresh},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuProcesses(), icon: icTerminal, keys: "⌘.", run: s.cmdProcesses},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuRefreshTab(), icon: icRefresh, keys: "⌘⌥R", run: s.cmdRefreshTab},
			paletteItem{kind: locales.PaletteCommand(), label: locales.MenuExport(), icon: icDownload, keys: "⌘⇧E", run: s.cmdExport},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ExportTablesMenu(), icon: icDownload, run: func() { s.exportTables(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ImportFileMenu(), icon: icDownload, run: func() { s.importFile(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.DumpBackupMenu(), icon: icDownload, run: s.openBackup},
			paletteItem{kind: locales.PaletteCommand(), label: locales.DumpRestoreMenu(), icon: icDownload, run: s.openRestore},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarConsole(), icon: icConsole, keys: "⌃⌘C", run: func() { s.consoleOpen = !s.consoleOpen }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.SearchDb(), icon: icSearch, run: s.openDatabaseSearch},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarSwitchDatabase(), icon: icDatabase, keys: "⌘K", run: s.openSwitcher},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarReconnect(), icon: icPlug, keys: "⌘⇧R", run: func() { s.reconnect(w) }},
			paletteItem{kind: locales.PaletteCommand(), label: locales.ToolbarDisconnect(), icon: icUnplug, keys: "⌘⌥.", run: func() { s.closeConnection(w.cfg.ID) }},
		)
		for _, mode := range database.SafeModes {
			mode := mode
			cmds = append(cmds, paletteItem{kind: locales.PaletteCommand(), label: locales.SafeTitle() + ": " + safeModeLabel(mode), hint: safeModeDesc(mode), icon: icLock, run: func() { s.setSafeMode(w, mode) }})
		}
		for _, t := range w.tables {
			name := t.Name
			add(paletteItem{kind: locales.PaletteTable(), label: name, hint: w.db, icon: icTable, run: func() { w.openTable(name) }, newTab: func() { w.openTableNewTab(name) }})
		}
		for _, h := range s.pal.history {
			sql := h.SQL
			add(paletteItem{kind: locales.PaletteHistory(), label: firstLines(sql, 1), icon: icHistory, run: func() { w.newQuery(sql) }})
		}
	}
	for _, cfg := range s.conns {
		cfg := cfg
		add(paletteItem{kind: locales.PaletteConnection(), label: cfg.Name, hint: connSubtitle(cfg), icon: icDatabase, dbType: cfg.Type, run: func() { s.openConnection(cfg) }})
	}
	for _, it := range cmds {
		add(it)
	}
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
	items := s.paletteItems()
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
	s.popup(c, &p.open, 600, &p.query, locales.PaletteHint(), func() { p.sel = 0 }, choose, func(d int) { p.sel = min(max(p.sel+d, 0), max(len(items)-1, 0)) }, func() {
		if len(items) == 0 {
			ui.Text(c, locales.PaletteNone()).TextColor(pp.fgMuted).Padding(14).TextAlign(ui.Center).FillWidth()
		}
		ui.Scroll(c.Key("palette")).MaxHeight(360).FillWidth().Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i, it := range items {
					i, it := i, it
					row := ui.Row(c.Key(it.kind+"/"+it.label+"/"+it.hint)).Gap(10).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
					switch {
					case i == p.sel:
						row.Background(pp.accent.Alpha(0.12))
					case row.Hovered():
						row.Background(pp.hover)
					}
					row.Children(func() {
						// an icon for what the row is; the kind in words only where there is none
						switch {
						case it.dbType != "":
							dbTile(c, it.dbType, 20, ui.Hex(dbColors[it.dbType]))
						case it.icon != nil:
							ui.Icon(c, it.icon).FontSize(15).TextColor(paletteIconColor(pp, it.icon)).Width(20)
						default:
							ui.Text(c, it.kind).FontSize(font2XS - 1).TextColor(pp.fgMuted).Width(20)
						}
						ui.Text(c, it.label).SingleLine().Grow(1)
						if it.hint != "" {
							ui.Text(c, it.hint).FontSize(font2XS - 1).TextColor(pp.fgMuted).SingleLine().MaxWidth(220).TruncateMiddle()
						}
						if it.keys != "" {
							ui.Text(c, it.keys).Font(mono).FontSize(font2XS-1).TextColor(pp.fgSecondary).Padding(1, 6).Radius(4).Background(pp.surfaceSecondary).Border(1, pp.borderSubtle)
						}
					})
					if row.Clicked() {
						p.sel = i
						choose()
					}
				}
			})
		})
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
	case icCode:
		return p.iconQuery
	}
	return p.fgSecondary
}
