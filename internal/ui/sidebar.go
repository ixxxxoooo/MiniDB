package ui

import (
	"fmt"
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
	focusFilter bool            // ⌘⇧F: give the filter the keyboard next frame
	showFilter  bool            // 用户呼出了筛选框（列表不长时默认收起）
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
			// 项目 / 查询 / 历史：三等分的圆角分段开关
			seg := w.side.seg
			if i := pillTabs(c, p, "sideseg", []string{locales.SidebarItems(), locales.SidebarQueries(), locales.SidebarHistory()}, seg, true); i >= 0 {
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
}

func (w *Workspace) sectionHeader(c *ui.Context, p tokens, r sideRow) {
	open := !w.side.collapsed[r.section]
	row := ui.Row(c.Key("sec-"+r.section)).Gap(5).AlignItems(ui.Center).Padding(0, 8).Height(24).Radius(6).Margin(6, 4, 1, 4).FillWidth()
	if row.Hovered() {
		row.Background(p.hover)
	}
	// 悬浮提示告诉用户这一栏可以收起 / 展开
	tip := locales.SidebarCollapse()
	if !open {
		tip = locales.SidebarExpand()
	}
	tipOn(c, row, tip)
	hover := row.Hovered()
	overview := false
	row.Children(func() {
		ic := icChevronD
		if !open {
			ic = icChevronR
		}
		ui.Icon(c, ic).FontSize(11).TextColor(p.fgSecondary)
		ui.Text(c, r.label).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
		ui.Text(c, fmt.Sprintf("(%d)", r.count)).FontSize(font2XS).TextColor(p.fgMuted)
		// 表 / 视图这两栏：悬停时右端出现「项目概览」入口（右键标题也有）
		if hover && (r.section == "tables" || r.section == "views" || r.section == "pinned") {
			ui.Spacer(c)
			if tipBtn(c.Key("ov-"+r.section), icTable, locales.OverviewTitle()).Size(18, 18).Clicked() {
				overview = true
			}
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.OverviewTitle()).Chosen() {
			defer w.openOverview()
		}
	})
	if overview {
		w.openOverview()
		return
	}
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
	kind := KindTable
	if isView(info) {
		kind = KindView
	}
	ic := objectIcon(kind)
	row := ui.Row(c.Key(r.section+"/"+info.Name)).Gap(8).AlignItems(ui.Center).Padding(0, 12).Height(24).Radius(6).Margin(0, 4).FillWidth()
	// 悬浮显示完整表名（名字被侧栏截断时也看得全），有备注就附在下面
	tip := info.Name
	if strings.TrimSpace(info.Comment) != "" {
		tip += "\n" + strings.TrimSpace(info.Comment)
	}
	tipOn(c, row, tip)
	row.Focusable() // 点一下选中后键盘归这一行：⌥⌫ 清空、Delete 删除，和 TablePlus 一样
	if picked {
		row.Background(p.accent.Alpha(0.12))
	} else if row.Hovered() {
		row.Background(p.hover) // 鼠标扫过：灰色底，和点选的强调色底区分开
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
	if picked && !w.cfg.ReadOnly {
		w.tableRowKeys(row, info.Name)
	}
}

// tableRowKeys 是选中的表行拿着键盘时的快捷键：⌥⌫ 清空表，单按 Delete 删除表（视图则删除视图），都先确认。
func (w *Workspace) tableRowKeys(row ui.Element, name string) {
	switch {
	case row.Shortcut(ui.Alt, ui.KeyBackspace):
		w.truncateTable(name)
	case row.Shortcut(0, ui.KeyBackspace), row.Shortcut(0, ui.KeyDelete):
		if w.isViewName(name) {
			w.dropObject(KindView, name)
		} else {
			w.dropTable(name)
		}
	}
}
