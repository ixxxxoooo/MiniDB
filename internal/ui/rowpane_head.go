package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 行预览停靠栏的头部和 JSON 视图（设计稿：右侧 300px，头部「行 7  orders · id 10225」+ 字段 / JSON 分段开关）。

// head 画停靠栏头部：左边是行号和来源，右边是字段 / JSON 切换和关闭。
// 参数：rows 选中的行（升序）；src 数据来源。
func (pp *rowPane) head(c *ui.Context, p tokens, src paneSource, rows []int) {
	title := locales.PreviewTitle()
	switch len(rows) {
	case 0:
	case 1:
		title = locales.PreviewRowTitle(itoa(src.g.offset + rows[0] + 1))
	default:
		title = locales.TableRows(len(rows))
	}
	ui.Row(c).Gap(spaceSM).Padding(0, spaceSM, 0, spaceMD).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Background(p.surface).Children(func() {
		ui.Text(c, title).FontSize(fontCtl).FontWeight(600).SingleLine().Shrink(0)
		if src.g.table != "" {
			ui.Text(c, paneSubtitle(src, rows)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(1).MinWidth(0)
		}
		ui.Spacer(c)
		if len(rows) > 0 {
			sel := 0
			if pp.asJSON {
				sel = 1
			}
			if segmented(c, &sel, locales.PreviewTabFields(), locales.PreviewTabJSON()) {
				pp.asJSON = sel == 1
			}
		}
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			src.onClose()
		}
	})
	hairline(c, p.border)
}

// jsonView 把选中的行整个当 JSON 显示（只读，可选中复制）：所有可见列，格式同「将行复制为 JSON」。
func (pp *rowPane) jsonView(c *ui.Context, p tokens, src paneSource, rows []int) {
	b := block{}
	for _, i := range src.g.vis {
		b.cols = append(b.cols, src.g.cols[i].Name)
	}
	for _, r := range rows {
		b.rows = append(b.rows, src.g.rows[r])
	}
	text := strings.TrimRight(b.format(fmtJSON, src.g.dbType, src.g.table, false), "\n")
	scrollbars(c, ui.Scroll(c.Key("rowjson")).Grow(1).FillWidth()).Children(func() {
		ui.Text(c, text).Selectable().Font(mono).FontSize(fontDense).Padding(spaceMD).FillWidth()
	})
}

// paneSubtitle 是头部标题右边的灰字：「表名 · 主键列 值」；多行或没有主键时只有表名。
func paneSubtitle(src paneSource, rows []int) string {
	if len(rows) != 1 {
		return src.g.table
	}
	for _, col := range src.g.cols {
		if src.g.pkOf(col.Name) {
			tx, _ := cellDisplay(col, src.g.rows[rows[0]][col.Name])
			return src.g.table + " · " + col.Name + " " + tx
		}
	}
	return src.g.table
}
