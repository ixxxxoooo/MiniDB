package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 命令面板右边的预览（设计稿）：表 → 字段与类型；历史 → 语法高亮的 SQL、执行时间和库；连接 → 地址、版本、环境；
// 命令 / 问 AI → 名称、说明和快捷键。表的字段在选中它时才去取，取回来缓存在 palette.cols 里。

// palKey 是表字段缓存的键：连接 / 库 / 表。
func (w *Workspace) palKey(table string) string { return w.cfg.ID + "/" + w.db + "/" + table }

// palettePrefetch 在选中一张表时后台取它的字段（已经有或正在取就不再取）。参数：it 是当前选中的条目。
func (s *Shell) palettePrefetch(it paletteItem) {
	w := s.activeWorkspace()
	if w == nil || it.table == "" {
		return
	}
	key := w.palKey(it.table)
	pal := &s.pal
	if _, ok := pal.cols[key]; ok || pal.loading[key] {
		return
	}
	if pal.loading == nil {
		pal.loading, pal.cols = map[string]bool{}, map[string][]model.ColumnInfo{}
	}
	pal.loading[key] = true
	connID, db, table := w.cfg.ID, w.db, it.table
	var cols []model.ColumnInfo
	s.up.bg(func() { cols, _ = s.app.Databases.GetColumns(connID, db, table) }, func() {
		delete(pal.loading, key)
		pal.cols[key] = cols
	})
}

// palettePreview 画右边的预览栏。参数：it 是选中的条目，没有时为 nil。
func (s *Shell) palettePreview(c *ui.Context, p tokens, it *paletteItem) {
	ui.Column(c.Key("pal-prev")).Grow(1).MinWidth(0).FillHeight().Background(p.surfaceSecondary).Children(func() {
		if it == nil {
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
				ui.Icon(c, icSearch).FontSize(fontHeading).TextColor(p.fgFaint)
			})
			return
		}
		ui.Row(c).Gap(spaceMD).AlignItems(ui.Center).Padding(spaceLG, spaceLG+2, spaceMD, spaceLG+2).FillWidth().Shrink(0).Children(func() {
			paletteLead(c, p, *it, sizeAvatarLg, false)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				title := it.label
				if it.hist != nil {
					title = it.hist.ExecutedAt.Local().Format("2006-01-02 15:04")
				}
				ui.Text(c, title).FontSize(fontBody).FontWeight(650).SingleLine().TruncateMiddle()
				if it.hint != "" {
					ui.Text(c, it.hint).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
				}
			})
		})
		hairline(c, p.borderSubtle)
		scrollbars(c, ui.Scroll(c.Key("pal-prev-body")).Grow(1).FillWidth()).Children(func() { // 正文自己滚：字段多、SQL 长都能看全，滚动不带着左边
			ui.Column(c).Padding(spaceMD, spaceLG+2, spaceLG, spaceLG+2).Gap(spaceXS).FillWidth().Children(func() {
				switch {
				case it.table != "":
					s.previewColumns(c, p, *it)
				case it.hist != nil:
					s.previewHistory(c, p, *it.hist)
				case it.cfg != nil:
					s.previewConn(c, p, *it.cfg)
				default:
					if it.keys != "" {
						kbd(c, it.keys)
					}
				}
			})
		})
	})
}

// previewColumns 画表的全部字段（预览区自己滚）：名称（等宽）和类型（灰）。
func (s *Shell) previewColumns(c *ui.Context, p tokens, it paletteItem) {
	ui.Text(c, locales.DocColField()).FontSize(fontCaption).FontWeight(600).TextColor(p.fgMuted)
	w := s.activeWorkspace()
	cols := s.pal.cols[w.palKey(it.table)]
	for _, col := range cols {
		ui.Row(c.Key("pv-" + col.Name)).FillWidth().Height(sizeRow).AlignItems(ui.Center).Children(func() {
			ui.Text(c, col.Name).Font(mono).FontSize(fontCtl).SingleLine().Shrink(1).MinWidth(0)
			ui.Spacer(c)
			ui.Text(c, col.Type).Font(mono).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(0)
		})
		hairline(c, p.borderSubtle)
	}
	ui.Text(c, "⏎ "+locales.PaletteFootOpen()+"　·　⌘⏎ "+locales.PaletteFootNewTab()).FontSize(fontCaption).TextColor(p.fgMuted).Margin(spaceSM, 0, 0, 0)
}

// previewHistory 画一条历史：语法高亮的 SQL（可换行）和所在的库。
func (s *Shell) previewHistory(c *ui.Context, p tokens, h app.HistoryItem) {
	ui.Column(c).FillWidth().Padding(spaceMD, spaceMD+2).Radius(radiusBtn+2).Background(p.surface).Border(1, p.border).Children(func() {
		ui.RichText(c, sqlSpans(p, h.SQL, p.fg)...).FontSize(fontDense).Font(mono).Selectable().FillWidth()
	})
	if h.Database != "" {
		ui.Text(c, h.Database).FontSize(fontCaption).TextColor(p.fgMuted).Margin(spaceSM, 0, 0, 0)
	}
	ui.Text(c, "⏎ "+locales.PaletteFootOpen()).FontSize(fontCaption).TextColor(p.fgMuted).Margin(spaceSM, 0, 0, 0)
}

// previewConn 画一个连接：类型和地址、环境标记。
func (s *Shell) previewConn(c *ui.Context, p tokens, cfg model.ConnectionConfig) {
	ui.Text(c, connSubtitle(cfg)).FontSize(fontCtl).TextColor(p.fgSecondary).SingleLine().TruncateMiddle()
	if cfg.Tag != "" {
		sqChip(c, tagChipKind(cfg.Tag), tagShort(cfg.Tag)).Margin(spaceSM, 0, 0, 0)
	}
	ui.Text(c, "⏎ "+locales.PaletteFootOpen()).FontSize(fontCaption).TextColor(p.fgMuted).Margin(spaceSM, 0, 0, 0)
}
