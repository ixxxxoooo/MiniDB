package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/locales"
)

// The sidebar follows TablePlus: a segmented control on top (Items, Queries,
// History), a filter, the items in sections (Pinned, Tables, Views) with their
// counts, and a footer with the database and a "+" menu.

const (
	sideItems = iota
	sideQueries
	sideHistory
)

// sideState is the sidebar's own view state.
type sideState struct {
	seg         int
	collapsed   map[string]bool // sections folded away
	pins        []string        // pinned table names, in the order pinned
	picked      string          // the picked item's name
	focusFilter bool            // ⌘⇧F: give the filter the keyboard next frame
	showFilter  bool            // 用户呼出了筛选框（列表不长时默认收起）
	queries     []app.HistoryItem
	queriesAt   time.Time
}

// pinsBucket keeps the pinned tables of a connection's database.
const pinsBucket = "pins"

type pinList struct {
	Names []string `json:"names"`
}

func (w *Workspace) isPinned(name string) bool {
	for _, n := range w.side.pins {
		if n == name {
			return true
		}
	}
	return false
}

// togglePin pins a table to the top of the list, or lets it go back.
func (w *Workspace) togglePin(name string) {
	if w.isPinned(name) {
		out := w.side.pins[:0:0]
		for _, n := range w.side.pins {
			if n != name {
				out = append(out, n)
			}
		}
		w.side.pins = out
	} else {
		w.side.pins = append(w.side.pins, name)
	}
	pins, key := append([]string(nil), w.side.pins...), w.dbKey()
	w.sh.up.bg(func() { _ = w.sh.app.Store.Put(pinsBucket, key, pinList{Names: pins}) }, func() {})
}

// loadPins reads the pinned tables of the current database.
func (w *Workspace) loadPins() {
	var pl pinList
	key := w.dbKey()
	w.sh.up.bg(func() {
		if w.sh.app.Store.Get(pinsBucket, key, &pl) != nil {
			pl = pinList{}
		}
	}, func() {
		if key == w.dbKey() {
			w.side.pins = pl.Names
		}
	})
}

// sideRow is one line of the Items list: a section header or an item.
type sideRow struct {
	header  bool
	section string // pinned, tables, views
	label   string // header text
	count   int
	info    database.TableInfo

	routineName string // set on a function or procedure row
	kind        string // function or procedure
}

func isView(t database.TableInfo) bool { return strings.Contains(strings.ToUpper(t.Type), "VIEW") }

// sideRows lays the filtered items out in sections. Pinned items come first,
// and appear there only.
func (w *Workspace) sideRows() []sideRow {
	var pinned, tables, views []database.TableInfo
	byName := map[string]database.TableInfo{}
	for _, t := range w.visibleTables() {
		byName[t.Name] = t
	}
	for _, n := range w.side.pins {
		if t, ok := byName[n]; ok {
			pinned = append(pinned, t)
			delete(byName, n)
		}
	}
	for _, t := range w.visibleTables() {
		if _, ok := byName[t.Name]; !ok {
			continue
		}
		if isView(t) {
			views = append(views, t)
		} else {
			tables = append(tables, t)
		}
	}
	var rows []sideRow
	add := func(section, label string, items []database.TableInfo) {
		if len(items) == 0 {
			return
		}
		rows = append(rows, sideRow{header: true, section: section, label: label, count: len(items)})
		if w.side.collapsed[section] {
			return
		}
		for _, t := range items {
			rows = append(rows, sideRow{section: section, info: t})
		}
	}
	add("pinned", locales.SidebarPinned(), pinned)
	add("tables", locales.SidebarTables(), tables)
	add("views", locales.SidebarViews(), views)
	addRoutines := func(section, label, kind string) {
		var names []string
		q := strings.ToLower(strings.TrimSpace(w.filter))
		for _, r := range w.routines {
			if r.Kind == kind && (q == "" || strings.Contains(strings.ToLower(r.Name), q)) {
				names = append(names, r.Name)
			}
		}
		if len(names) == 0 {
			return
		}
		rows = append(rows, sideRow{header: true, section: section, label: label, count: len(names)})
		if w.side.collapsed[section] {
			return
		}
		for _, n := range names {
			rows = append(rows, sideRow{section: section, routineName: n, kind: kind})
		}
	}
	addRoutines("functions", locales.SidebarFunctions(), "function")
	addRoutines("procedures", locales.SidebarProcedures(), "procedure")
	return rows
}

func (w *Workspace) sidebar(c *ui.Context, p tokens) {
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Grow(1).FillHeight().Background(p.toolbar).Children(func() {
			// 项目 / 查询 / 历史：三等分，高度和样式与右侧内容区的标签栏一致
			seg := w.side.seg
			if i := tabStrip(c, p, "sideseg", []string{locales.SidebarItems(), locales.SidebarQueries(), locales.SidebarHistory()}, seg, connColor(c, w.cfg.Color), true, nil); i >= 0 {
				w.side.seg = i
			}
			if seg != w.side.seg {
				w.loadSideQueries()
			}
			w.sideFilter(c)
			if w.side.seg == sideItems {
				w.itemsList(c, p)
			} else {
				w.queriesList(c, p)
			}
			w.sidebarFooter(c, p)
		})
	})
}

// sideFilter 画侧栏的筛选框。默认不显示：只有已有筛选词、或用户主动呼出
// （⌘⇧F 或底部的放大镜）时才出现；在框里按 Esc 清空并收起。
func (w *Workspace) sideFilter(c *ui.Context) {
	if w.side.focusFilter {
		w.side.showFilter = true
	}
	if !w.side.showFilter && w.filter == "" {
		return
	}
	filter := ui.SearchField(c, &w.filter).Placeholder(locales.SidebarFilterPlaceholder()).FontSize(font2XS).Height(sizeBtn).Margin(0, 8, 4, 8)
	if w.side.focusFilter {
		w.side.focusFilter = false
		filter.Focus()
	}
	if filter.Changed() && w.side.seg != sideItems {
		w.loadSideQueries()
	}
	if filter.Focused() && c.Shortcut(0, ui.KeyEscape) {
		w.filter, w.side.showFilter = "", false
		if w.side.seg != sideItems {
			w.loadSideQueries()
		}
	}
}

// itemsList is the sectioned list of tables and views.
func (w *Workspace) itemsList(c *ui.Context, p tokens) {
	rows := w.sideRows()
	switch {
	case w.loadErr != "":
		ui.Text(c, locales.TableLoadFail(w.loadErr)).FontSize(font2XS).TextColor(p.danger).Padding(8, 10)
	case w.loading && len(w.tables) == 0:
		if delayed(c, w.loadStart, 300*time.Millisecond) {
			ui.Text(c, locales.CommonLoading()).FontSize(font2XS).TextColor(p.fgMuted).Padding(16).TextAlign(ui.Center).FillWidth()
		}
	case len(rows) == 0:
		msg := locales.SidebarNoTables()
		if w.filter != "" {
			msg = locales.SidebarNoMatch()
		}
		ui.Text(c, msg).FontSize(font2XS).TextColor(p.fgMuted).Padding(16).TextAlign(ui.Center).FillWidth()
	}
	key := func(i int) string {
		if rows[i].header {
			return "§" + rows[i].section
		}
		if rows[i].routineName != "" {
			return rows[i].section + "/" + rows[i].routineName
		}
		return rows[i].section + "/" + rows[i].info.Name
	}
	w.tableSel.Key = func(i int) any { return key(i) }
	listBars(c, ui.List(c.Key("tables"), &w.tableSel, len(rows), func(i int) {
		r := rows[i]
		if r.header {
			w.sectionHeader(c, p, r)
			return
		}
		w.itemRow(c, p, r)
	}).Grow(1).Label(locales.SidebarTables()), &w.tableSel)
	// ⌥⌫ empties the picked table, as TablePlus does
	if w.side.picked != "" && w.tableSel.Focused(c) && c.Shortcut(ui.Alt, ui.KeyBackspace) && !w.cfg.ReadOnly {
		w.truncateTable(w.side.picked)
	}
}

func (w *Workspace) sectionHeader(c *ui.Context, p tokens, r sideRow) {
	open := !w.side.collapsed[r.section]
	row := ui.Row(c.Key("sec-"+r.section)).Gap(4).AlignItems(ui.Center).Padding(8, 12, 2, 12).FillWidth()
	row.Children(func() {
		ic := icChevronD
		if !open {
			ic = icChevronR
		}
		ui.Icon(c, ic).FontSize(11).TextColor(p.fgMuted)
		ui.Text(c, strings.ToUpper(r.label)).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
		ui.Text(c, fmt.Sprintf("(%d)", r.count)).FontSize(font2XS).TextColor(p.fgMuted)
	})
	if row.Clicked() {
		if w.side.collapsed == nil {
			w.side.collapsed = map[string]bool{}
		}
		w.side.collapsed[r.section] = open
	}
}

func (w *Workspace) itemRow(c *ui.Context, p tokens, r sideRow) {
	if r.routineName != "" {
		w.routineRow(c, p, r)
		return
	}
	info := r.info
	picked := w.side.picked == info.Name
	ic, kind := icTable, "table"
	if isView(info) {
		ic, kind = icView, "view"
	}
	row := ui.Row(c.Key(r.section+"/"+info.Name)).Gap(8).AlignItems(ui.Center).Padding(0, 12).Height(24).Radius(6).Margin(0, 4).FillWidth()
	// 悬浮显示完整表名（名字被侧栏截断时也看得全），有备注就附在下面
	tip := info.Name
	if strings.TrimSpace(info.Comment) != "" {
		tip += "\n" + strings.TrimSpace(info.Comment)
	}
	tipOn(c, row, tip)
	if picked {
		row.Background(p.accent.Alpha(0.12))
	}
	row.Children(func() {
		col, txt := iconColor(p, kind), p.fg
		if picked {
			col, txt = p.accent, p.accent
		}
		ui.Icon(c, ic).FontSize(12).TextColor(col)
		t := ui.Text(c, info.Name).FontSize(font2XS).TextColor(txt).SingleLine().Grow(1)
		if picked {
			t.FontWeight(500)
		}
		if r.section == "pinned" {
			ui.Text(c, "●").FontSize(font2XS - 3).TextColor(p.fgMuted)
		}
	})
	switch {
	case row.DoubleClicked():
		w.side.picked = info.Name
		w.openTable(info.Name)
	case row.Clicked():
		w.side.picked = info.Name
		w.previewTable(info.Name)
	}
	row.ContextMenu(func(m *ui.Menu) { w.tableMenu(m, info.Name) })
}

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
			for _, it := range items {
				it := it
				row := tipOn(c, ui.Column(c.Key(it.ID)).Gap(2).Padding(5, 8).Radius(6).FillWidth(), clipText(it.SQL, 600, 16))
				if row.Hovered() {
					row.Background(p.hover)
				}
				row.Children(func() {
					if it.Name != "" {
						ui.Row(c).Gap(5).AlignItems(ui.Center).FillWidth().Children(func() {
							ui.Icon(c, icStarFill).FontSize(10).TextColor(p.warning).Shrink(0)
							ui.Text(c, it.Name).FontSize(font2XS + 1).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
						})
					}
					ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
						// 历史/收藏里的语句和日志一样做语法高亮
						ui.RichText(c, sqlSpans(p, firstLines(it.SQL, 2), p.fg)...).FontSize(font2XS).Font(mono).SingleLine().MaxWidth(w.sidebarW - 40 - float32(len(it.Keyword)*7)).Shrink(1)
						if it.Keyword != "" {
							ui.Text(c, it.Keyword).FontSize(font2XS - 1).TextColor(p.accent).FontWeight(600).SingleLine()
						}
					})
					ui.Text(c, it.ExecutedAt.Format("01-02 15:04")+histDB(it.Database)).FontSize(font2XS - 1).TextColor(p.fgMuted)
				})
				if row.Clicked() {
					w.newQuery(it.SQL)
				}
				row.ContextMenu(func(m *ui.Menu) { w.queryItemMenu(c, m, it) })
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

func isReadOnlySQL(sql string) bool {
	return database.SafeModeRequirement(database.SafeModeAlert2, false, []string{sql}) == database.SafeNone
}

// sidebarFooter has the current database (a click switches it) and the "+" menu.
func (w *Workspace) sidebarFooter(c *ui.Context, p tokens) {
	hairline(c, p.borderSubtle)
	// 与右边的底栏（tablebar.go bottomBar）同高：整条 sizeBtn+2，控件 sizeBtnSm，在条里垂直居中
	ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn + 2).FillWidth().Shrink(0).Children(func() {
		if w.cfg.Type != "sqlite" {
			name := w.db
			if name == "" {
				name = locales.SidebarNoDatabase()
			}
			if tipOn(c, ui.Button(c.Key("sidedb"), "").Label(name), locales.ToolbarSwitchDatabase()+" (⌘K)").Height(sizeBtnSm).Padding(0, 10).Grow(1).MinWidth(0).Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icDatabase).FontSize(12).TextColor(p.fgSecondary)
					ui.Text(c, name).FontSize(font2XS).SingleLine()
				})
			}).Clicked() {
				w.sh.openSwitcher()
			}
		} else {
			ui.Spacer(c)
		}
		// 放大镜：呼出（或收起）筛选框
		if toggleBtn(c, icSearch, locales.SidebarFilterPlaceholder()+w.sh.keyHint("side-filter"), w.side.showFilter || w.filter != "").Clicked() {
			if w.side.showFilter || w.filter != "" {
				w.side.showFilter, w.filter = false, ""
				w.loadSideQueries()
			} else {
				w.side.focusFilter = true
			}
		}
		ui.MenuButton(c.Key("sideplus"), "+", func(m *ui.Menu) {
			if m.Item(locales.SidebarNewQuery()).Shortcut(ui.Cmd, ui.KeyT).Chosen() {
				defer func() { w.newQuery("") }()
			}
			if m.Item(locales.CommonRefresh()).Shortcut(ui.Cmd, ui.KeyR).Chosen() {
				defer w.loadTables()
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
			if database.SupportsUsers(w.cfg.Type) && m.Item(locales.UsersMenu()).Chosen() {
				defer func() { w.sh.openUsers() }()
			}
			if m.Item(locales.DumpBackupMenu()).Chosen() {
				defer func() { w.sh.openBackup() }()
			}
			if w.cfg.Type != "sqlite" && m.Item(locales.DumpRestoreMenu()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer func() { w.sh.openRestore() }()
			}
			m.Separator()
			if w.cfg.Type != "sqlite" && m.Item(locales.DbNew()).Disabled(w.cfg.ReadOnly).Chosen() {
				defer func() { w.sh.newDatabase(w) }()
			}
		}).Height(sizeBtnSm).Padding(0, 8)
	})
}
