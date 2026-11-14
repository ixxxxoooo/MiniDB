package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

var _ = strings.TrimSpace

// baseTokens 是当前外观的配色，强调色取窗口主题里的（applyLook 已把它设成当前工作区的主题色），
// 这样没拿到 Shell.tokens 的角落（编辑框描边、开关按钮的选中态等）也跟着工作区的颜色走。
func baseTokens(c *ui.Context) tokens {
	p := lightTokens
	if c.Theme().Dark {
		p = darkTokens
	}
	if th := c.Theme(); th.Accent != (ui.Color{}) {
		p.accent, p.accentHover = th.Accent, th.AccentHover
	}
	return p
}

func (t *tableTab) view(c *ui.Context, p tokens) {
	if t.preview && (t.pending() > 0 || t.mode != 0) {
		t.keep() // editing or looking at the structure makes it a real tab
	}
	switch t.mode {
	case 1:
		t.structureView(c)
	case 2:
		t.ddlView(c)
	case 3:
		t.docView(c)
	default:
		t.dataView(c, p)
	}
	t.bottomBar(c, p)
}

func (t *tableTab) modeChanged() {
	switch t.mode {
	case 1:
		if !t.st.loaded && !t.st.loading {
			t.st.load(t)
		}
	case 2:
		if !t.ddl.loaded && !t.ddl.loading {
			t.ddl.load(t)
		}
	case 3:
		if !t.doc.loaded {
			t.doc.load(t)
		}
	}
}

func (t *tableTab) dataView(c *ui.Context, p tokens) {
	if !t.started { // restored tabs load when first shown
		t.load()
	}
	if t.showFilter {
		t.filterBar(c, p)
		hairline(c, p.border)
	}
	switch {
	case t.err != "":
		ui.Text(c, locales.TableLoadFail(t.err)).TextColor(p.danger).Padding(16).Selectable()
	case len(t.g.cols) == 0:
		// First load: keep the area still; nothing flashes for a quick load.
		quietLoading(c, p, &t.loadStart)
	default:
		ui.Column(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
			// The grid keeps its place in the tree whether or not the row pane
			// is open, so its scroll position survives the pane opening: the
			// pane is only added at the right.
			ui.Row(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
				ui.Column(c.Key("grid")).Grow(1).MinWidth(0).FillHeight().Children(func() { t.gridView(c) })
				if t.showPreview {
					t.previewHandle(c, p)
					ui.Column(c.Key("preview")).Width(t.previewW).FillHeight().Shrink(0).Children(func() { t.rowPreview(c, p) })
				}
			})
			// A reload keeps the rows in place; a thin bar over the top edge
			// shows it, after a moment, without moving anything.
			if t.loading && delayed(c, t.loadStart, 500*time.Millisecond) {
				ui.Progress(c, -1).Absolute().Top(0).Left(0).Right(0).Height(2)
			}
		})
	}
	t.columnsDialog(c)
	t.quickDialog(c)
	t.quickLookDialog(c)
	t.sqlPreviewDialog(c)
	if c.Shortcut(0, ui.KeySpace) && t.es.editing == nil {
		t.togglePreview()
	}
}

// togglePreview shows or hides the row pane; opening it with no row chosen
// chooses the first, so the pane always has something to show.
func (t *tableTab) togglePreview() {
	t.showPreview = !t.showPreview
	if t.previewW == 0 {
		t.previewW = 340
	}
	if t.showPreview && t.previewRow() < 0 && len(t.g.rows) > 0 {
		t.g.chosen.Add(0)
	}
}

// filterBar is the filter area of the grid, as TablePlus's: any number of
// conditions, each with a check box; the ticked ones apply together with
// Apply all (⌘↵) and Unset clears them. "Raw SQL" in the column list takes a
// WHERE clause or a full query. Escape closes the bar.
func (t *tableTab) filterBar(c *ui.Context, p tokens) {
	cols := []string{locales.DatagridRawSQL(), locales.TableFilterAny()}
	for _, col := range t.g.cols {
		cols = append(cols, col.Name)
	}
	ft := *c.Theme()
	ft.Spacing, ft.FontSize = 2.25, font2XS
	apply := false
	focused := -1
	ui.Themed(c, &ft).Background(p.surfaceSecondary).FillWidth().Shrink(0).Children(func() {
		ui.Column(c).Gap(4).Padding(5, 10).FillWidth().Children(func() {
			for i := range t.filters {
				i := i
				f := &t.filters[i]
				raw := f.col == locales.DatagridRawSQL()
				k := func(s string) string { return fmt.Sprint("f", i, s) }
				ui.Row(c.Key(k("row"))).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
					ui.Checkbox(c.Key(k("on")), &f.on, "")
					ui.Select(c.Key(k("col")), &f.col, cols).MinWidth(0).Width(118).Height(sizeBtnSm)
					if !raw {
						ui.Select(c.Key(k("op")), &f.op, filterOps).MinWidth(0).Width(86).Height(sizeBtnSm)
					}
					ph := locales.TableFilterValue()
					if raw {
						ph = locales.TableRawPlaceholder()
					}
					in := ui.TextInput(c.Key(k("val")), &f.val).Placeholder(ph).Grow(1).Height(sizeBtnSm).Font(mono)
					if t.focusFilter && i == 0 {
						t.focusFilter = false
						in.Focus()
					}
					if in.Focused() {
						focused = i
					}
					if in.Submitted() {
						f.on = true
						apply = true
					}
					if in.Changed() && strings.TrimSpace(f.val) != "" {
						f.on = true // typing a value ticks the condition
					}
					if len(t.filters) > 1 {
						if tipBtn(c.Key(k("rm")), icX, locales.TableFilterRemove()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
							defer func() { t.filters = append(t.filters[:i], t.filters[i+1:]...) }()
						}
					}
					if i == len(t.filters)-1 {
						if tipBtn(c.Key(k("add")), icPlus, locales.TableFilterAddRow()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
							defer func() { t.filters = append(t.filters, newFilterRow()) }()
						}
					}
					if i == 0 {
						if actPrimary(c.Key("applyall"), icCheck, locales.TableFilterApplyAll()+" (⌘↵)").Clicked() {
							apply = true
						}
						if actBtn(c.Key("unset"), icX, locales.TableFilterUnset()).Disabled(!t.hasFilter()).Clicked() {
							t.clearFilter()
						}
						if len(t.filters) > 1 { // how the ticked conditions join
							or := 0
							if t.filterOr {
								or = 1
							}
							if ui.Segmented(c.Key("join"), &or, "AND", "OR").FontSize(font2XS).Changed() {
								t.filterOr = or == 1
							}
						}
						if tipOn(c, actBtn(c.Key("filtersql"), icCode, "SQL"), locales.TableFilterSQLTip()).Clicked() {
							t.w.newQuery(t.tableSelectSQL())
						}
						tipBtn(c.Key("filterexport"), icDownload, locales.TableExport()).Size(sizeBtnSm, sizeBtnSm).Menu(func(m *ui.Menu) {
							if m.Item(locales.TableExportTable()).Chosen() {
								defer t.w.sh.exportTable(t.w, t.table)
							}
							if m.Item(locales.TableExportPage()).Chosen() {
								defer t.w.sh.exportQuery(t.w, t.tableSelectSQL())
							}
						})
					}
				})
			}
		})
	})
	t.filterFocused = focused >= 0
	if focused >= 0 { // in the bar: ⌘B ticks, ⌘I adds a condition, ⌘⇧I removes this one
		switch {
		case c.Shortcut(ui.Cmd, ui.KeyB):
			t.filters[focused].on = !t.filters[focused].on
		case c.Shortcut(ui.Cmd, ui.KeyI):
			t.filters = append(t.filters, newFilterRow())
		case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyI):
			if len(t.filters) > 1 {
				t.filters = append(t.filters[:focused], t.filters[focused+1:]...)
			}
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyEnter) {
		apply = true
	}
	if apply {
		t.applyFilter()
	}
	if c.Shortcut(0, ui.KeyEscape) && t.es.editing == nil {
		t.toggleFilter()
	}
}

// subViewPill is the rounded segmented switch of the bottom bar.
func (t *tableTab) subViewPill(c *ui.Context, p tokens) {
	// The original keeps these four in English in every language.
	if i := pillTabs(c, p, "sv", []string{"Data", "Structure", "DDL", "Doc"}, t.mode, false); i >= 0 && i != t.mode {
		t.mode = i
		t.modeChanged()
	}
}

// pillTabs 是应用统一的圆角分段开关（底部的 Data/Structure/DDL/Doc、侧栏的
// 项目/查询/历史都用它）。grow 为真时各段等分整行宽度。返回被点中的段，没有则 -1。
func pillTabs(c *ui.Context, p tokens, key string, labels []string, cur int, grow bool) (clicked int) {
	clicked = -1
	bh, fs := float32(20), float32(font2XS-1)
	if grow { // 侧栏顶部：更高一些
		bh, fs = 24, float32(font2XS)
	}
	draw := func() {
		row := ui.Row(c.Key(key)).Gap(0).Padding(2).Radius(radiusBtn + 2).Background(p.hover).AlignItems(ui.Center)
		if grow {
			row.FillWidth()
		}
		row.Children(func() {
			for i, l := range labels {
				i, l := i, l
				b := ui.Button(c.Key(fmt.Sprint(key, i)), "").Label(l).Padding(1, 10).Radius(radiusBtn).Height(bh)
				if grow {
					b.Grow(1).Basis(0)
				}
				if i == cur {
					b.Background(p.surface).Border(1, p.borderSubtle)
				} else {
					// 未选中的段也留着 1px 透明边框、字重也一样，切换时尺寸才不会抖
					b.Background(ui.Color{}).Border(1, ui.Transparent)
				}
				b.Children(func() {
					col := p.fgSecondary
					if i == cur {
						col = p.fg
					}
					ui.Text(c, l).FontSize(fs).TextColor(col).FontWeight(500).SingleLine()
				})
				if b.Clicked() {
					clicked = i
				}
			}
		})
	}
	if grow {
		// 外面套一行带内边距的容器，而不是给 FillWidth 的行加外边距（那样右边会超出一截，
		// 最右一段就被挡住了）；上面只留很小的空隙
		ui.Row(c.Key(key+"-wrap")).FillWidth().Padding(4, 8, 6, 8).Shrink(0).Children(draw)
	} else {
		draw()
	}
	return clicked
}

// bottomBar is the 28px strip under a table: sub-view switch, row actions,
// refresh and commit, row count and pagination.
func (t *tableTab) bottomBar(c *ui.Context, p tokens) {
	hairline(c, p.border)
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(2).Padding(0, 8).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 2).Background(p.surfaceSecondary).Shrink(0).Children(func() {
			if !t.isNew { // 新建表只有结构页
				t.subViewPill(c, p)
			}
			ui.Spacer(c)
			readOnly := t.w.cfg.ReadOnly
			switch t.mode {
			case 0:
				if tipBtn(c, icPlus, locales.EditAddRow()).Size(sizeBtnSm, sizeBtnSm).Disabled(readOnly).Clicked() {
					t.addRow()
				}
				if tipBtn(c, icTrash, locales.EditDeleteRows()).Size(sizeBtnSm, sizeBtnSm).Disabled(readOnly).Clicked() {
					t.deleteSelected()
				}
				if tipBtn(c, icPanel, locales.PreviewToggle()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.togglePreview()
				}
				vline(c, p.border, 12)
				if toggleBtn(c, icSliders, locales.TableColumnsTip()+" (⌘⌥F)", t.hiddenCount() > 0).Clicked() {
					t.colsOpen = true
				}
				// 过滤：只放一个图标；筛选条打开或已有生效的条件时亮起强调色
				if toggleBtn(c, icSearch, locales.TableAddCondition()+" (⌘F)", t.showFilter || t.hasFilter()).Clicked() {
					t.toggleFilter()
				}
				if tipOn(c, actBtn(c, icCode, "SQL"), locales.ToolbarSQL()).Clicked() {
					sql, _ := t.w.sh.app.Queries.DefaultSelectTableSQL(t.w.cfg.ID, t.table, 100)
					t.w.newQuery(sql)
				}
				if tipBtn(c, icRefresh, locales.CommonRefresh()+" (⌘R)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.reload(nil)
				}
				if n := t.pendingCount(); n > 0 {
					if actBtn(c, icUndo, locales.StatusDiscard()).Clicked() {
						t.revert()
					}
					if tipOn(c, actPrimary(c, icCheck, locales.CommonCommit()+" ("+fmt.Sprint(n)+")"), locales.CommonCommit()+" (⌘S)").Clicked() {
						t.commitNow()
					}
				}
				vline(c, p.border, 12)
				// fixed width: the count arrives after the rows and must not shift the bar
				ui.Row(c).Gap(4).Width(124).Shrink(0).AlignItems(ui.Center).Children(func() {
					rows := "…"
					if t.total >= 0 {
						rows = locales.TableRows(int(t.total))
					}
					ui.Text(c, rows).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
					if t.dur > 0 {
						ui.Text(c, "·").FontSize(font2XS).TextColor(p.fgMuted)
						ui.Text(c, fmtDur(t.dur)).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
					}
				})
				t.pagination(c, p)
			case 1:
				if !t.isNew && tipBtn(c, icRefresh, locales.CommonRefresh()+" (⌘R)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
					t.st.load(t)
				}
				if n := t.st.changeCount(); n > 0 {
					if actBtn(c, icUndo, locales.StructDiscard()).Clicked() {
						t.st.reset()
					}
					if tipOn(c, actPrimary(c, icCheck, locales.CommonCommit()+" ("+fmt.Sprint(n)+")"), locales.CommonCommit()+" (⌘S)").Disabled(readOnly).Clicked() {
						t.commitStructure()
					}
				}
			}
		})
	}).Padding(0)
}

func (t *tableTab) pagination(c *ui.Context, p tokens) {
	size := t.size()
	pages := max(int((t.total+int64(size)-1)/int64(size)), 1)
	if t.total < 0 { // the count is still coming
		pages = max(t.page, 1)
	}
	if ui.Select(c, &t.pageSize, pageSizes).MinWidth(0).Width(62).Height(sizeBtnSm).FontSize(font2XS).Changed() {
		t.reload(func() { t.page = 1 })
	}
	if tipBtn(c.Key("prev"), icChevronR, locales.TablePrev()).Size(18, 18).Disabled(t.page <= 1).Rotate(180).Clicked() {
		t.reload(func() { t.page-- })
	}
	ui.Text(c, fmt.Sprintf("%d/%d", t.page, pages)).FontSize(font2XS).TextColor(p.fgSecondary)
	if tipBtn(c.Key("next"), icChevronR, locales.TableNext()).Size(18, 18).Disabled(t.page >= pages).Clicked() {
		t.reload(func() { t.page++ })
	}
}

func (t *tableTab) toggleFilter() {
	t.showFilter = !t.showFilter
	t.focusFilter = t.showFilter
	if !t.showFilter && t.hasFilter() {
		t.clearFilter()
	}
}

var _ = database.Filter{}

// delayed reports whether d has passed since start; until then it asks for
// a frame at that moment, so a state that only shows after a delay appears
// on time without anything else redrawing.
func delayed(c *ui.Context, start time.Time, d time.Duration) bool {
	if left := d - time.Since(start); left > 0 {
		c.After(left)
		return false
	}
	return true
}

// previewHandle is the line between grid and row pane; dragging it resizes the pane.
func (t *tableTab) previewHandle(c *ui.Context, p tokens) {
	dx, reset := vHandle(c, p, "phandle")
	t.previewW = min(max(t.previewW-dx, 220), 720)
	if reset {
		t.previewW = 340
	}
}

// quietLoading fills a view that has nothing to show yet. It stays blank, and
// only after a good while shows a plain note: no spinner, no animation, so
// switching views never flashes a loading state.
func quietLoading(c *ui.Context, p tokens, since *time.Time) {
	if since.IsZero() {
		*since = time.Now()
	}
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		if delayed(c, *since, 800*time.Millisecond) {
			ui.Text(c, locales.CommonLoading()).FontSize(font2XS).TextColor(p.fgMuted)
		}
	})
}
