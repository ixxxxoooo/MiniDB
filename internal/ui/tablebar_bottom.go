package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"minidb/locales"
)

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
