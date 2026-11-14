package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"minidb/locales"
)

// 状态栏里的翻页：每页行数、上一页、第几页 / 共几页、下一页（行范围文字在 table_topbar.go 的 dataStatus）。
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
