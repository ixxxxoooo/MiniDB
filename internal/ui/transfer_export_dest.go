package ui

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 导出向导的后两步：目标位置（保存位置、文件名模板、将生成的文件）与执行（进度、逐表状态）。

// exDest 是第三步：保存位置、文件名模板、行数上限，以及将要生成的文件。
func (t *transferTab) exDest(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.exp
	chosen := d.chosenTables()
	multi := len(chosen) != 1
	if d.dest != "" && d.destMulti != multi { // 单表选的是文件、多表选的是目录，选法变了就得重选
		d.dest = ""
	}
	xferSection(c, p, locales.XferSaveTo(), func() {
		ui.Row(c.Key("xf-dest")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
			text, col := locales.ExportSaveHint(), p.fgMuted
			if d.dest != "" {
				text, col = d.dest, p.fg
				if multi {
					text = filepath.Dir(d.dest)
				}
			}
			ui.Text(c, text).Font(mono).FontSize(fontCtl).TextColor(col).SingleLine().TruncateMiddle().Grow(1).MinWidth(0).
				Padding(0, spaceSM).Height(sizeInput).Background(p.surface).Border(1, p.border).Radius(radiusBtn)
			if fieldBtn(c.Key("xf-pick"), icFolder, locales.XferChoose()).Clicked() {
				s.pickExportDest()
			}
		})
	})
	xferSection(c, p, locales.XferFiles(), func() {
		if multi {
			dlgField(c, p, locales.XferNameTpl(), func() {
				textField(c.Key("exp-pat"), &d.prefs.Pattern).Placeholder("{table}_{datetime}").FillWidth().Height(sizeInput)
				ui.Text(c, locales.ExportPatternHint("{table}", "{db}", "{date}", "{time}", "{datetime}")).FontSize(fontCaption).TextColor(p.fgMuted)
			})
		}
		dlgField(c, p, locales.ExportLimit(), func() {
			textField(c.Key("exp-limit"), &d.limit).Placeholder(locales.ExportLimitPlaceholder()).Width(sizeWizCol).Height(sizeInput)
			ui.Text(c, locales.ExportLimitHint()).FontSize(fontCaption).TextColor(p.fgMuted)
		})
	})
	t.exFiles(c, p)
}

// exFiles 是目标位置那步底部的卡片：将生成几个文件、各叫什么。
func (t *transferTab) exFiles(c *ui.Context, p tokens) {
	d := &t.w.sh.exp
	chosen := d.chosenTables()
	pattern := ""
	if len(chosen) > 1 {
		pattern = d.prefs.Pattern
	}
	ext := d.exportOptions().Extension()
	var names []string
	for _, tb := range chosen {
		names = append(names, app.ExpandPattern(pattern, tb, d.w.db, time.Now())+"."+ext)
	}
	more := ""
	if len(names) > 6 {
		names, more = names[:6], " …"
	}
	xferCard(c, p).Padding(spaceSM+2, spaceMD+2).Gap(spaceXS).Background(p.surfaceSecondary).Children(func() {
		ui.Text(c, locales.XferFileCount(len(chosen))).FontSize(fontCaption).TextColor(p.fgMuted)
		ui.Text(c, strings.Join(names, " · ")+more).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary).FillWidth()
	})
}

// exRun 是第四步：总进度、每个文件的状态，运行中可以取消。
func (t *transferTab) exRun(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.exp
	r := &d.xfer
	runChip(c, p, r)
	ui.Progress(c.Key("xf-bar"), r.bar()).FillWidth()
	if r.err != "" {
		ui.Text(c, r.err).FontSize(fontBody).TextColor(p.danger).Selectable()
	}
	xferCard(c, p).Children(func() {
		chosen := d.chosenTables()
		for i, tb := range chosen {
			ui.Row(c.Key("f-"+tb)).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceMD).Height(sizeObjRow).FillWidth().Children(func() {
				stateIcon(c, p, r)
				ui.Icon(c, icTable).FontSize(btnActIcon).TextColor(p.iconTable)
				ui.Text(c, tb).FontSize(fontBody).FontWeight(500).SingleLine().Grow(1)
			})
			if i < len(chosen)-1 {
				hairline(c, p.borderSubtle)
			}
		}
	})
	ui.Row(c.Key("xf-run-act")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		if r.running() && fieldBtn(c.Key("xf-cancel"), icStop, locales.CommonCancel()).Clicked() {
			s.app.Exports.CancelExport(r.id)
		}
		ui.Spacer(c)
		ui.Text(c, locales.XferBackground()).FontSize(fontCaption).TextColor(p.fgMuted)
	})
}

// runChip 画执行步顶部的状态芯片和已处理的行数。
func runChip(c *ui.Context, p tokens, r *xferRun) {
	ui.Row(c.Key("xf-status")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		switch r.status {
		case "done":
			chip(c, chipOK, locales.XferDone(), true)
		case "error":
			chip(c, chipErr, locales.XferFailed(), true)
		case "cancelled":
			chip(c, chipWarn, locales.XferCancelled(), true)
		default:
			chip(c, chipAccent, locales.XferRunning(), true)
		}
		ui.Spacer(c)
		text := locales.XferRowsDone(int(r.cur))
		if r.total > 0 {
			text = locales.XferRowsOf(int(r.cur), int(r.total))
		}
		ui.Text(c, text).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine()
	})
}

// stateIcon 是清单每行前面的状态：转圈、对勾、叉、等待点。
func stateIcon(c *ui.Context, p tokens, r *xferRun) {
	switch r.status {
	case "done":
		ui.Icon(c, icCheckC).FontSize(btnActIcon).TextColor(p.success)
	case "error":
		ui.Icon(c, icXCircle).FontSize(btnActIcon).TextColor(p.danger)
	case "cancelled":
		ui.Icon(c, icXCircle).FontSize(btnActIcon).TextColor(p.warning)
	default:
		ui.Spinner(c).Size(btnActIcon, btnActIcon)
	}
}
