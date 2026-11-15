package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 导入向导的四步：选择文件 → 目标表 → 字段映射 → 导入。数据放在 Shell.imp（importDialog）里，
// 解析预览、列映射、安全模式关口和真正的导入都沿用 import.go / importview.go，这里只负责画页面。

// importBody 画当前这一步；有错误时先在顶部显示。
func (t *transferTab) importBody(c *ui.Context, p tokens) {
	d := &t.w.sh.imp
	if d.err != "" {
		ui.Text(c, d.err).FontSize(fontBody).TextColor(p.danger).Selectable().FillWidth()
	}
	switch t.step {
	case 0:
		t.imFile(c, p)
	case 1:
		t.imTarget(c, p)
	case 2:
		t.imMap(c, p)
	default:
		t.imRun(c, p)
	}
}

// ready 说明文件已经解析好（数据文件有预览，或是能直接执行的 .sql）。
func (d *importDialog) ready() bool { return d.preview != nil || d.sql != nil }

// targetName 是导入的目标表名：新建表时是输入的名字，否则是选中的现有表。
func (d *importDialog) targetName() string {
	if d.newTable {
		return strings.TrimSpace(d.newName)
	}
	return d.table
}

// importFoot 画页脚：左边一句概括，右边上一步 / 下一步（映射那步的主按钮是「开始导入」）。
func (t *transferTab) importFoot(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	hint := ""
	switch {
	case t.step == 0 && d.preview != nil:
		hint = locales.ImportRows(int(d.preview.TotalRows)) + " · " + locales.XferColumnsCount(len(d.preview.Columns))
	case t.step == 1 && d.preview != nil:
		hint = locales.XferTargetSummary(d.targetName(), importConflictLabel(d))
	case t.step == 2:
		hint = locales.ImportAtomic()
	}
	ui.Text(c, hint).FontSize(fontBody).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
	ui.Spacer(c)
	switch t.step {
	case 0:
		t.navButtons(c, false, locales.XferNext(), !d.ready(), func() { t.step++ })
	case 1:
		t.navButtons(c, true, locales.XferNext(), !d.ready(), func() { t.step++ })
	case 2:
		label := locales.ImportStart()
		if d.sql != nil {
			label = locales.ImportSQLRun()
		}
		t.navButtons(c, true, label, !d.ready() || d.busy, func() {
			d.xfer, t.waiting = xferRun{}, true
			s.confirmImport()
		})
	default:
		if d.xfer.running() {
			return
		}
		if fieldBtn(c.Key("xf-again"), nil, locales.XferAgain()).Clicked() {
			d.xfer, t.step = xferRun{}, 0
		}
	}
}

// importConflictLabel 是当前「写入方式」的显示名；新建表时没有这个概念，返回空。
func importConflictLabel(d *importDialog) string {
	if d.newTable {
		return ""
	}
	values, labels := importConflictValues(), importConflictLabels()
	for i, v := range values {
		if v == d.conflict {
			return labels[i]
		}
	}
	return labels[0]
}

// imFile 是第一步：选文件，设解析方式，看前几行预览。
func (t *transferTab) imFile(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	if d.path == "" {
		zone := ui.Column(c.Key("xf-drop")).FillWidth().Height(sizeDropZone).Center().Gap(spaceXS+2).Radius(radiusDialog).Border(1.5, p.accent.Alpha(0.45)).Background(p.accentSoft)
		if zone.Hovered() {
			zone.Background(p.accentSoft2)
		}
		zone.Children(func() {
			ui.Icon(c, icUpload).FontSize(sizeDropIcon).TextColor(p.accentText)
			ui.Text(c, locales.XferDrop()).FontSize(fontBody).FontWeight(600).TextColor(p.accentText)
			ui.Text(c, locales.XferDropSub()).FontSize(fontCaption).TextColor(p.fgMuted)
		})
		if zone.Clicked() {
			s.pickImportFile()
		}
		return
	}
	xferCard(c, p).Padding(spaceMD, spaceLG-2).Children(func() {
		ui.Row(c.Key("xf-file")).Gap(spaceMD).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Icon(c, icFileSQL).FontSize(fontHeading).TextColor(p.fgSecondary)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, filepath.Base(d.path)).FontSize(fontBody).FontWeight(600).SingleLine().TruncateMiddle()
				ui.Text(c, importMeta(d)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
			})
			if actBtn(c.Key("xf-change"), nil, locales.XferChange()).Clicked() {
				s.pickImportFile()
			}
		})
	})
	switch {
	case d.sql != nil:
		s.importSQLView(c, p)
	case d.preview != nil:
		t.imParse(c, p)
		t.imPreview(c, p)
	}
}

// importMeta 是文件卡片第二行：行数、编码、分隔符等已识别出来的信息。
func importMeta(d *importDialog) string {
	switch {
	case d.sql != nil:
		return locales.ImportSQLCount(d.sql.Count)
	case d.preview == nil:
		return ""
	}
	parts := []string{locales.ImportRows(int(d.preview.TotalRows))}
	if d.preview.Format != app.ImportFormatXLSX {
		parts = append(parts, d.encoding, d.delimiter)
	}
	return strings.Join(parts, " · ")
}

// imParse 是解析方式一行：编码、分隔符、引号（或 Excel 的工作表），右边「首行为列名」开关。
func (t *transferTab) imParse(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	ui.Row(c.Key("xf-parse")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		if d.preview.Format == app.ImportFormatXLSX {
			s.importSheetPick(c, p)
		} else {
			s.importCSVOpts(c, p)
		}
		ui.Spacer(c)
		ui.Text(c, locales.ImportHasHeader()).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine()
		if ui.Switch(c.Key("xf-hdr"), &d.opts.HasHeader).Changed() {
			s.refreshImportPreview()
		}
	})
}

// imPreview 是文件前几行：表头一行加几行数据，宽了可以横向滚。
func (t *transferTab) imPreview(c *ui.Context, p tokens) {
	d := &t.w.sh.imp
	xferCard(c, p).Children(func() {
		scrollbars(c, ui.ScrollBoth(c.Key("xf-pv")).FillWidth()).Children(func() {
			ui.Column(c).Children(func() {
				ui.Row(c.Key("pv-h")).Padding(0, spaceMD).Height(sizeObjRow).AlignItems(ui.Center).Background(p.surfaceSecondary).Children(func() {
					for i, name := range d.preview.Columns {
						ui.Text(c.Key(fmt.Sprint(i)), name).Width(sizeWizCol).Font(mono).FontSize(fontDense).FontWeight(600).SingleLine().Shrink(0)
					}
				})
				for r, row := range d.preview.Rows[:min(5, len(d.preview.Rows))] {
					ui.Row(c.Key(fmt.Sprint("pv", r))).Padding(0, spaceMD).Height(sizeObjRow).AlignItems(ui.Center).Children(func() {
						for i, cell := range row {
							ui.Text(c.Key(fmt.Sprint(i)), oneLine(cell)).Width(sizeWizCol).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine().Shrink(0)
						}
					})
				}
			})
		})
	})
}

// needFile 在还没选文件时代替后面几步的内容。
func (t *transferTab) needFile(c *ui.Context, p tokens) bool {
	d := &t.w.sh.imp
	if !d.ready() {
		ui.Text(c, locales.XferNeedFile()).FontSize(fontBody).TextColor(p.fgMuted)
		return true
	}
	if d.sql != nil {
		ui.Text(c, locales.XferSQLNoMap()).FontSize(fontBody).TextColor(p.fgSecondary)
		return true
	}
	return false
}

// imTarget 是第二步：导入到已有表还是新建表、写入方式、事务等选项。
func (t *transferTab) imTarget(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.imp
	if t.needFile(c, p) {
		return
	}
	ui.Row(c.Key("xf-target")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.XferImportInto()).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine()
		if d.newTable {
			textField(c.Key("imp-new"), &d.newName).Placeholder(locales.ImportNewName()).Width(sizeWizCol + 40).Height(sizeInput)
		} else {
			var names []string
			for _, tb := range d.w.tables {
				names = append(names, tb.Name)
			}
			if ui.Combobox(c, &d.table, names).Width(sizeWizCol + 40).Height(sizeInput).Changed() {
				s.loadImportCols(d.table)
			}
			ui.Text(c, locales.XferExistingTbl(len(d.tableCols))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		}
		ui.Spacer(c)
		sel := 0
		if d.newTable {
			sel = 1
		}
		if segmented(c, &sel, locales.ImportTargetExisting(), locales.ImportTargetNew()) {
			d.newTable = sel == 1
		}
	})
	if !d.newTable {
		xferSection(c, p, locales.XferWriteMode(), func() {
			values, labels := importConflictValues(), importConflictLabels()
			sel := 0
			for i, v := range values {
				if v == d.conflict {
					sel = i
				}
			}
			if segmented(c, &sel, labels...) {
				d.conflict = values[sel]
			}
		})
	}
	xferSection(c, p, locales.XferOptions(), func() {
		ui.Column(c.Key("xf-iopts")).Gap(spaceSM + 2).FillWidth().Children(func() {
			switchRow(c, p, "o-atomic", locales.ImportAtomic(), &d.atomic)
			switchRow(c, p, "o-null", locales.ImportEmptyAsNull(), &d.emptyAsNull)
		})
	})
}
