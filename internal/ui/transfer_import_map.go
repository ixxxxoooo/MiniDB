package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 导入向导的后两步：字段映射（源列 → 目标列）与导入（进度、统计卡片）。

// imMap 是第三步：源文件每一列对到哪一列（新建表时是起什么名、什么类型）。
func (t *transferTab) imMap(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	if t.needFile(c, p) {
		return
	}
	matched := 0
	if d.newTable {
		for _, nc := range d.newCols {
			if strings.TrimSpace(nc.name) != "" {
				matched++
			}
		}
	} else {
		for _, tg := range d.target {
			if tg != "" {
				matched++
			}
		}
	}
	ui.Row(c.Key("xf-maphead")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.XferImportInto()).FontSize(fontBody).TextColor(p.fgSecondary)
		ui.Text(c, d.targetName()).FontSize(fontBody).FontWeight(600)
		ui.Spacer(c)
		if !d.newTable {
			ui.Text(c, locales.ImportMapBy()).FontSize(fontCaption).TextColor(p.fgMuted)
			if labelSelect(c, &d.mapBy, []string{"name", "order"}, []string{locales.ImportMapByName(), locales.ImportMapByOrder()}, 100) {
				s.autoMapImport()
			}
		}
		kind := chipOK
		if matched < len(d.preview.Columns) {
			kind = chipWarn
		}
		chip(c, kind, locales.XferMatched(matched, len(d.preview.Columns)), false)
	})
	opts := append([]string{locales.ImportSkip()}, d.tableCols...)
	xferCard(c, p).Children(func() {
		ui.Row(c.Key("map-h")).Gap(spaceSM).Padding(0, spaceMD).Height(sizeObjRow).AlignItems(ui.Center).Background(p.surfaceSecondary).Children(func() {
			ui.Text(c, locales.XferSrcCol()).Width(sizeWizCol).FontSize(fontCaption).FontWeight(600).TextColor(p.fgSecondary)
			ui.Box(c).Width(spaceXL)
			ui.Text(c, locales.XferDstCol()).Width(sizeWizCol + spaceXL).FontSize(fontCaption).FontWeight(600).TextColor(p.fgSecondary)
			ui.Text(c, locales.XferSample()).Grow(1).FontSize(fontCaption).FontWeight(600).TextColor(p.fgSecondary)
		})
		for i, name := range d.preview.Columns {
			hairline(c, p.borderSubtle)
			sample := ""
			if len(d.preview.Rows) > 0 && i < len(d.preview.Rows[0]) {
				sample = d.preview.Rows[0][i]
			}
			ui.Row(c.Key(fmt.Sprint("map", i))).Gap(spaceSM).Padding(0, spaceMD).Height(sizeObjRow).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, name).Width(sizeWizCol).Font(mono).FontSize(fontDense).SingleLine().TruncateMiddle().Shrink(0)
				ui.Icon(c, icChevronR).FontSize(btnActIcon).TextColor(p.fgMuted).Width(spaceXL).Shrink(0)
				t.mapTarget(c, p, i, opts)
				ui.Text(c, oneLine(sample)).Grow(1).MinWidth(0).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine().TruncateMiddle()
			})
		}
	})
}

// mapTarget 画映射表里「目标列」那一格：已有表是下拉，新建表是列名输入加类型。
func (t *transferTab) mapTarget(c *ui.Context, p tokens, i int, opts []string) {
	d := &t.w.sh.imp
	if d.newTable {
		if i < len(d.newCols) {
			textField(c.Key(fmt.Sprint("nc", i)), &d.newCols[i].name).Placeholder(locales.ImportSkip()).Width(sizeWizCol - spaceXL).Height(btnActH)
			labelSelect(c, &d.newCols[i].kind, importKinds, importKindLabels(), 70)
		}
		return
	}
	cur := d.target[i]
	if cur == "" {
		cur = locales.ImportSkip()
	}
	if ui.Select(c, &cur, opts).Width(sizeWizCol + spaceXL).Height(btnActH).Changed() {
		if cur == locales.ImportSkip() {
			cur = ""
		}
		d.target[i] = cur
	}
}

// imRun 是第四步：进度条、已写入 / 总行数 / 状态三张小卡，运行中可以取消。
func (t *transferTab) imRun(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	r := &d.xfer
	if !r.active {
		ui.Text(c, locales.XferNeedFile()).FontSize(fontBody).TextColor(p.fgMuted)
		return
	}
	ui.Row(c.Key("xf-iprog")).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.XferImporting()).FontSize(fontBody).TextColor(p.fgSecondary)
		ui.Spacer(c)
		if r.total > 0 {
			ui.Text(c, fmt.Sprintf("%d / %d", r.cur, r.total)).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary)
		}
	})
	ui.Progress(c.Key("xf-ibar"), r.bar()).FillWidth()
	total := "—"
	if r.total > 0 {
		total = fmt.Sprint(r.total)
	}
	status, col := locales.XferRunning(), p.accentText
	switch r.status {
	case "done":
		status, col = locales.XferDone(), p.success
	case "error":
		status, col = locales.XferFailed(), p.danger
	case "cancelled":
		status, col = locales.XferCancelled(), p.warning
	}
	ui.Row(c.Key("xf-stats")).Gap(spaceSM + 2).FillWidth().Children(func() {
		statCard(c, p, locales.XferWritten(), fmt.Sprint(r.cur), p.success)
		statCard(c, p, locales.XferTotalRows(), total, p.fg)
		statCard(c, p, locales.XferStatus(), status, col)
	})
	if r.err != "" {
		ui.Text(c, r.err).FontSize(fontBody).TextColor(p.danger).Selectable().FillWidth()
	}
	ui.Row(c.Key("xf-run-act")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		if r.running() && fieldBtn(c.Key("xf-cancel"), icStop, locales.CommonCancel()).Clicked() {
			s.app.Imports.CancelImport(r.id)
		}
		ui.Spacer(c)
		ui.Text(c, locales.XferBackground()).FontSize(fontCaption).TextColor(p.fgMuted)
	})
}

// statCard 是执行步的一张统计小卡：标题加一个大数字。
func statCard(c *ui.Context, p tokens, title, value string, col ui.Color) {
	xferCard(c, p).Grow(1).Basis(0).Padding(spaceSM+2, spaceMD+2).Gap(2).Children(func() {
		ui.Text(c, title).FontSize(fontCaption).TextColor(p.fgMuted)
		ui.Text(c, value).Font(mono).FontSize(fontHeading).FontWeight(700).TextColor(col).SingleLine()
	})
}
