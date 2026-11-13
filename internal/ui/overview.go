package ui

// @author ixxxxoooo

import (
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// 项目概览：把当前库所有表和视图列成一张表——名称、类型、引擎、行数、数据和索引大小、排序规则、备注、创建和更新时间。
// 点表头排序，双击一行打开那张表，右键和侧栏里的表一样。MySQL、PostgreSQL 的行数是数据库的估算值。

// overviewTab 是项目概览标签；每个工作区最多一个。
type overviewTab struct {
	w       *Workspace
	items   []database.TableOverview
	loading bool
	err     string
	shownAt time.Time
	q       string // 名称 / 备注的筛选词

	list    ui.ListState
	sel     int
	sort    ui.SortOrder // 当前排序，空表示按名称
	libSort ui.SortOrder // 只用来接住表头点击
	shown   []int        // 筛选、排序后要显示的 items 下标
}

func (o *overviewTab) title() string { return locales.OverviewTitle() }
func (o *overviewTab) icon() *ui.SVG { return icTable }
func (o *overviewTab) pending() int  { return 0 }

// openOverview 打开项目概览标签，已经开着就切过去并刷新。
func (w *Workspace) openOverview() {
	for i, t := range w.tabs {
		if o, ok := t.(*overviewTab); ok {
			w.active = i
			o.load()
			return
		}
	}
	o := &overviewTab{w: w, sel: -1}
	w.tabs = append(w.tabs, o)
	w.active = len(w.tabs) - 1
	o.load()
}

// load 在后台读概览。
func (o *overviewTab) load() {
	w, db := o.w, o.w.db
	var items []database.TableOverview
	var err error
	o.loading, o.err = true, ""
	w.sh.up.bg(func() { items, err = w.sh.app.Databases.GetTableOverview(w.cfg.ID, db) }, func() {
		o.loading = false
		if err != nil {
			o.err = err.Error()
			return
		}
		o.items = items
	})
}

// arrange 按筛选词和排序算出要显示的行（items 的下标）。
func (o *overviewTab) arrange() {
	q := strings.ToLower(strings.TrimSpace(o.q))
	o.shown = o.shown[:0]
	for i, it := range o.items {
		if q == "" || strings.Contains(strings.ToLower(it.Name), q) || strings.Contains(strings.ToLower(it.Comment), q) {
			o.shown = append(o.shown, i)
		}
	}
	col, desc := o.sort.Column, o.sort.Descending
	if col == "" {
		return
	}
	sort.SliceStable(o.shown, func(a, b int) bool {
		x, y := o.items[o.shown[a]], o.items[o.shown[b]]
		less := overviewLess(col, x, y)
		if desc {
			return overviewLess(col, y, x)
		}
		return less
	})
}

// overviewLess 按 col 比较两行，col 是列的 ID。
func overviewLess(col string, x, y database.TableOverview) bool {
	switch col {
	case "type":
		return x.Type < y.Type
	case "engine":
		return x.Engine < y.Engine
	case "rows":
		return x.Rows < y.Rows
	case "data":
		return x.DataSize < y.DataSize
	case "index":
		return x.IndexSize < y.IndexSize
	case "coll":
		return x.Collation < y.Collation
	case "comment":
		return x.Comment < y.Comment
	case "created":
		return x.CreatedAt < y.CreatedAt
	case "updated":
		return x.UpdatedAt < y.UpdatedAt
	}
	return strings.ToLower(x.Name) < strings.ToLower(y.Name)
}

// overviewSummary 是底部的汇总：几张表、几个视图、数据和索引加起来多大。
func overviewSummary(items []database.TableOverview) string {
	var tables, views int
	var size int64
	for _, it := range items {
		if it.Type == "view" {
			views++
		} else {
			tables++
		}
		size += max(it.DataSize, 0) + max(it.IndexSize, 0)
	}
	return locales.OverviewSummary(tables, views, humanBytes(size))
}

func overviewNum(n int64) string {
	if n < 0 {
		return "—"
	}
	return itoa(int(n))
}

func overviewSize(n int64) string {
	if n < 0 {
		return "—"
	}
	return humanBytes(n)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func (o *overviewTab) view(c *ui.Context, p tokens) {
	th := c.Theme()
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(8).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
			ui.Icon(c, icTable).FontSize(13).TextColor(iconColor(p, "table"))
			ui.Text(c, locales.OverviewTitle()).FontWeight(600).FontSize(font2XS)
			ui.Text(c, "("+itoa(len(o.items))+")").FontSize(font2XS).TextColor(p.fgMuted)
			ui.Spacer(c)
			if o.loading {
				ui.Spinner(c)
			}
			ui.SearchField(c.Key("ovq"), &o.q).Placeholder(locales.SidebarFilterPlaceholder()).Height(sizeBtnSm).FontSize(font2XS).Width(200)
			if tipBtn(c, icRefresh, locales.CommonRefresh()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				o.load()
			}
		})
	}).Padding(0)
	hairline(c, p.border)
	if o.err != "" {
		ui.Text(c, locales.TableLoadFail(o.err)).TextColor(p.danger).FontSize(font2XS).Padding(12).Selectable()
		return
	}
	if o.loading && len(o.items) == 0 {
		quietLoading(c, p, &o.shownAt)
		return
	}
	o.arrange()

	gt := *th
	gt.Spacing, gt.FontSize, gt.Font = gridSpacing, font2XS, mono
	gt.TextMuted = th.Text
	soft := p.accent.Alpha(0.16)
	gt.Accent, gt.AccentHover, gt.AccentPressed, gt.InactiveAccent, gt.AccentText = soft, soft, soft, soft, th.Text
	title := func(id, label string) string {
		if o.sort.Column == id || (id == "name" && o.sort.Column == "") {
			if o.sort.Descending {
				return label + " ↓"
			}
			return label + " ↑"
		}
		return label
	}
	cols := []ui.TableColumn{
		{ID: "n", Title: "#", Width: 52, Fixed: true},
		{ID: "name", Title: title("name", locales.StructName()), Width: 220, MinWidth: 90, Sortable: true},
		{ID: "type", Title: title("type", locales.StructType()), Width: 70, MinWidth: 56, Sortable: true},
		{ID: "engine", Title: title("engine", locales.OverviewEngine()), Width: 90, MinWidth: 60, Sortable: true},
		{ID: "rows", Title: title("rows", locales.OverviewRows()), Width: 100, MinWidth: 60, Align: ui.End, Sortable: true},
		{ID: "data", Title: title("data", locales.OverviewDataSize()), Width: 100, MinWidth: 60, Align: ui.End, Sortable: true},
		{ID: "index", Title: title("index", locales.OverviewIndexSize()), Width: 100, MinWidth: 60, Align: ui.End, Sortable: true},
		{ID: "coll", Title: title("coll", locales.OverviewCollation()), Width: 160, MinWidth: 70, Sortable: true},
		{ID: "comment", Title: title("comment", locales.StructComment()), Width: 260, MinWidth: 80, Sortable: true},
		{ID: "created", Title: title("created", locales.OverviewCreated()), Width: 160, MinWidth: 80, Sortable: true},
		{ID: "updated", Title: title("updated", locales.OverviewUpdated()), Width: 160, MinWidth: 80, Sortable: true},
	}
	o.list.Selected = &o.sel
	o.list.Sort = &o.libSort
	o.list.Key = func(i int) any { return o.items[o.shown[i]].Name }
	o.list.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(o.shown) {
			o.w.tableMenu(m, o.items[o.shown[row]].Name)
		}
	}
	tint := func(row int) ui.Color { return structRowTint(p, row, o.sel == row, ui.Color{}) }
	structTable(c, p, &gt, &o.list, locales.OverviewTitle(), cols, len(o.shown), tint, func(row, ci int) {
		if row >= len(o.shown) {
			return
		}
		it := o.items[o.shown[row]]
		cell := func(content func()) {
			cellBox(c, tint(row), listColW(&o.list, cols, ci), content)
		}
		txt := func(s string) { ui.Text(c, s).SingleLine().TextColor(p.fg) }
		muted := func(s string) { ui.Text(c, orDash(s)).SingleLine().TextColor(p.fgSecondary) }
		switch cols[ci].ID {
		case "n":
			cell(func() { ui.Text(c, itoa(row+1)).TextColor(p.fgMuted).FontSize(font2XS) })
		case "name":
			cell(func() {
				ic, kind := icTable, "table"
				if it.Type == "view" {
					ic, kind = icView, "view"
				}
				ui.Icon(c, ic).FontSize(12).TextColor(iconColor(p, kind)).Shrink(0).Margin(0, 6, 0, 0)
				txt(it.Name)
			})
		case "type":
			cell(func() { muted(it.Type) })
		case "engine":
			cell(func() { muted(it.Engine) })
		case "rows":
			cell(func() { ui.Spacer(c); muted(overviewNum(it.Rows)) })
		case "data":
			cell(func() { ui.Spacer(c); muted(overviewSize(it.DataSize)) })
		case "index":
			cell(func() { ui.Spacer(c); muted(overviewSize(it.IndexSize)) })
		case "coll":
			cell(func() { muted(it.Collation) })
		case "comment":
			cell(func() { muted(it.Comment) })
		case "created":
			cell(func() { muted(it.CreatedAt) })
		case "updated":
			cell(func() { muted(it.UpdatedAt) })
		}
	}, func(tbl ui.Element) {
		// 双击或回车打开那张表
		if tbl.Submitted() && o.sel >= 0 && o.sel < len(o.shown) {
			o.w.openTable(o.items[o.shown[o.sel]].Name)
		}
	})
	// 表头点击：不排序（按名称）→ 升序 → 降序 → 回到按名称，和数据网格一样三态
	if clicked := o.libSort.Column; clicked != "" {
		o.libSort = ui.SortOrder{}
		switch {
		case o.sort.Column != clicked:
			o.sort = ui.SortOrder{Column: clicked}
		case !o.sort.Descending:
			o.sort = ui.SortOrder{Column: clicked, Descending: true}
		default:
			o.sort = ui.SortOrder{}
		}
	}
	hairline(c, p.border)
	ui.Row(c).Gap(8).Padding(0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 2).Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Text(c, overviewSummary(o.items)).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
		ui.Spacer(c)
		if o.w.cfg.Type != "sqlite" { // SQLite 的行数是精确统计的，其余库给的是估算值
			ui.Text(c, locales.OverviewRowsTip()).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
		}
	})
}
