package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
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
	filterOn    bool            // 筛选框这一帧是否拿着键盘（⌘⇧F 第二次按下时据此清空筛选词）
	focusFilter bool            // ⌘⇧F: give the filter the keyboard next frame
	hist        histOpts        // 历史 / 查询的搜索选项
	queries     []app.HistoryItem
	queriesAt   time.Time
}

// pinList 是某个库里置顶的表，由 UIStateService 存取。
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
	w.sh.up.bg(func() { _ = w.sh.app.UIState.SavePins(key, pinList{Names: pins}) }, func() {})
}

// loadPins reads the pinned tables of the current database.
func (w *Workspace) loadPins() {
	var pl pinList
	key := w.dbKey()
	w.sh.up.bg(func() {
		if w.sh.app.UIState.LoadPins(key, &pl) != nil {
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
	info    model.TableInfo

	routineName string     // set on a function or procedure row
	kind        ObjectKind // 函数或存储过程
}

func isView(t model.TableInfo) bool { return strings.Contains(strings.ToUpper(t.Type), "VIEW") }

// sideRows lays the filtered items out in sections. Pinned items come first,
// and appear there only.
func (w *Workspace) sideRows() []sideRow {
	var pinned, tables, views []model.TableInfo
	byName := map[string]model.TableInfo{}
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
	add := func(section, label string, items []model.TableInfo) {
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
	addRoutines := func(section, label string, kind ObjectKind) {
		var names []string
		q := strings.ToLower(strings.TrimSpace(w.filter))
		for _, r := range w.routines {
			if r.Kind == string(kind) && (q == "" || strings.Contains(strings.ToLower(r.Name), q)) {
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
	addRoutines("functions", locales.SidebarFunctions(), KindFunction)
	addRoutines("procedures", locales.SidebarProcedures(), KindProcedure)
	return rows
}

func (w *Workspace) sidebar(c *ui.Context, p tokens) {
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Grow(1).FillHeight().Background(p.toolbar).Children(func() {
			// 顶部是连接卡、库与安全模式、筛选框；项目 / 查询 / 历史三个页由活动栏切换（railbar.go）
			w.sideHeader(c, p)
			if w.side.seg == sideItems {
				w.itemsList(c, p)
			} else {
				w.queriesList(c, p)
			}
			w.sidebarFooter(c, p)
		})
	})
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
}
