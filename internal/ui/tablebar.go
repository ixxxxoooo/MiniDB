package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

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
	t.jumpView(c)
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

func (t *tableTab) toggleFilter() {
	t.showFilter = !t.showFilter
	t.focusFilter = t.showFilter
	if !t.showFilter && t.hasFilter() {
		t.clearFilter()
	}
}
