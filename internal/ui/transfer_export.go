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
// 导出向导的四步：选择对象 → 格式与选项 → 目标位置 → 执行。数据放在 Shell.exp（exportDialog）里，
// 真正的导出还是 app 的 Exports.StartExport；执行步的进度来自总线事件（transfer_task.go）。

// exportBody 画当前这一步。
func (t *transferTab) exportBody(c *ui.Context, p tokens) {
	switch t.step {
	case 0:
		t.exObjects(c, p)
	case 1:
		t.exFormat(c, p)
	case 2:
		t.exDest(c, p)
	default:
		t.exRun(c, p)
	}
}

// exportFoot 画页脚：左边一句概括，右边上一步 / 下一步（目标位置那步的主按钮是「导出」）。
func (t *transferTab) exportFoot(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.exp
	chosen := d.chosenTables()
	_, fmtName, _, _ := exportFormatInfo(d.format)
	hint := locales.ExportSummaryTables(len(chosen), fmtName)
	if t.step == 3 {
		hint = ""
	}
	ui.Text(c, hint).FontSize(fontBody).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
	ui.Spacer(c)
	switch t.step {
	case 0:
		t.navButtons(c, false, locales.XferNext(), len(chosen) == 0, func() { t.step++ })
	case 1:
		t.navButtons(c, true, locales.XferNext(), false, func() { t.step++ })
	case 2:
		t.navButtons(c, true, locales.ExportStart(), len(chosen) == 0, t.startExport)
	default:
		if d.xfer.running() {
			return
		}
		if fieldBtn(c.Key("xf-again"), nil, locales.XferAgain()).Clicked() {
			d.xfer, t.step = xferRun{}, 0
		}
		if d.xfer.status == "done" && fieldPrimary(c.Key("xf-open"), icFolder, locales.XferOpenFolder()).Clicked() {
			s.revealPath(d.xfer.path)
		}
	}
}

// startExport 带着向导里的选择开始导出；启动成功才跳到执行步（用户在保存对话框里取消就留在原地）。
func (t *transferTab) startExport() {
	s := t.w.sh
	d := &s.exp
	d.prefs.Format = d.format
	s.prefs.Export = d.prefs
	s.savePrefs()
	req := app.ExportRequest{ConnID: t.w.cfg.ID, Database: t.w.db, Tables: d.chosenTables(), Options: d.exportOptions(), Pattern: d.prefs.Pattern, Limit: d.limitRows()}
	if d.dest != "" {
		if len(req.Tables) == 1 {
			req.Path = d.dest
		} else {
			req.Path = filepath.Dir(d.dest)
		}
	}
	d.xfer = xferRun{}
	var id string
	var err error
	s.up.bg(func() { id, err = s.app.Exports.StartExport(req) }, func() { // 没选保存位置时这里会弹对话框，所以放到后台
		switch {
		case err != nil:
			s.toastErr(errMessage(err))
		case id != "":
			d.xfer.begin(id)
			t.step = 3
		}
	})
}

// exObjects 是第一步：筛选、勾选要导出的表和视图；只选一张表时可以挑列。
func (t *transferTab) exObjects(c *ui.Context, p tokens) {
	s := t.w.sh
	d := &s.exp
	s.syncExportCols()
	type item struct {
		name string
		rows int64
		view bool
	}
	var all, shown []item
	var picked int
	var rows int64
	q := strings.ToLower(strings.TrimSpace(d.search))
	for _, tb := range d.w.tables {
		it := item{tb.Name, tb.RowCount, strings.EqualFold(tb.Type, "view")}
		all = append(all, it)
		if d.sel[tb.Name] {
			picked++
			rows += tb.RowCount
		}
		if q == "" || strings.Contains(strings.ToLower(tb.Name), q) {
			shown = append(shown, it)
		}
	}
	ui.Row(c.Key("xf-find")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.SearchField(c.Key("exp-q"), &d.search).Placeholder(locales.ExportFilterTables()).Height(sizeInput).Grow(1).MaxWidth(sizeWizFind)
		if actBtn(c.Key("all"), icCheckAll, locales.ExportAll()).Clicked() {
			for _, it := range shown {
				d.sel[it.name] = true
			}
		}
		if actBtn(c.Key("inv"), nil, locales.ExportInvert()).Clicked() {
			for _, it := range shown {
				d.sel[it.name] = !d.sel[it.name]
			}
		}
		ui.Spacer(c)
		ui.Text(c, locales.XferPicked(picked, len(all), int(rows))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	})
	xferCard(c, p).Children(func() {
		if len(shown) == 0 {
			ui.Text(c, locales.SidebarNoMatch()).FontSize(fontBody).TextColor(p.fgMuted).Padding(spaceLG).TextAlign(ui.Center).FillWidth()
			return
		}
		scrollbars(c, ui.Scroll(c.Key("xf-objs")).MaxHeight(sizeWizList).FillWidth()).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for i, it := range shown {
					row := ui.Row(c.Key(it.name)).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceMD).Height(sizeObjRow).FillWidth().Shrink(0)
					if row.Hovered() {
						row.Background(p.hover)
					}
					row.Children(func() {
						v := d.sel[it.name]
						if ui.Checkbox(c.Key("cb"), &v, "").Changed() {
							d.sel[it.name] = v
						}
						if it.view {
							ui.Icon(c, icView).FontSize(btnActIcon).TextColor(p.iconView)
						} else {
							ui.Icon(c, icTable).FontSize(btnActIcon).TextColor(p.iconTable)
						}
						ui.Text(c, it.name).FontSize(fontBody).FontWeight(500).SingleLine().TruncateMiddle().Grow(1).MinWidth(0)
						switch {
						case it.view:
							ui.Text(c, locales.XferView()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(0)
						case it.rows > 0:
							ui.Text(c, locales.TableRows(int(it.rows))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Shrink(0)
						}
					})
					if row.Clicked() { // 点整行也能勾选
						d.sel[it.name] = !d.sel[it.name]
					}
					if i < len(shown)-1 {
						hairline(c, p.borderSubtle)
					}
				}
			})
		})
	})
	t.exColumns(c, p)
}

// exColumns 是清单下面的「导出的列」：只选了一张表时才有，点「自定义列…」展开勾选。
func (t *transferTab) exColumns(c *ui.Context, p tokens) {
	d := &t.w.sh.exp
	if len(d.cols) == 0 {
		return
	}
	on := 0
	for _, n := range d.cols {
		if d.colSel[n] {
			on++
		}
	}
	ui.Row(c.Key("xf-colhead")).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.ExportColumnsOf(on, len(d.cols))).FontSize(fontBody).TextColor(p.fgSecondary)
		ui.Spacer(c)
		if ghostBtn(c.Key("xf-cols"), nil, locales.XferCustomCols()).Clicked() {
			t.showCols = !t.showCols
		}
	})
	if !t.showCols {
		return
	}
	xferCard(c, p).Padding(spaceMD).Children(func() {
		ui.Row(c.Key("xf-collist")).Gap(spaceLG).Wrap().FillWidth().Children(func() {
			for _, n := range d.cols {
				v := d.colSel[n]
				if ui.Checkbox(c.Key("col-"+n), &v, n).Changed() {
					d.colSel[n] = v
				}
			}
		})
	})
}

// exFormat 是第二步：格式卡片和该格式的选项。
func (t *transferTab) exFormat(c *ui.Context, p tokens) {
	d := &t.w.sh.exp
	xferSection(c, p, locales.XferFormat(), func() {
		ui.Row(c.Key("xf-fmts")).Gap(spaceSM).Wrap().Children(func() {
			for i := range exportFormats {
				ic, name, ext, desc := exportFormatInfo(i)
				on := d.format == i
				card := tipOn(c, ui.Column(c.Key(fmt.Sprint("fmt", i))).Gap(2).Width(sizeFmtCard).Padding(spaceSM+2, spaceMD).Radius(radiusPanel).Border(1, p.border).Background(p.surface), desc)
				switch {
				case on:
					card.Border(1, p.accent).Background(p.accentSoft).Shadow(0, 0, 0, 3, p.accentSoft)
				case card.Hovered():
					card.Border(1, p.fgMuted)
				}
				card.Children(func() {
					ui.Row(c).Gap(spaceXS + 2).AlignItems(ui.Center).Children(func() {
						col := p.fgSecondary
						if on {
							col = p.accentText
						}
						ui.Icon(c, ic).FontSize(btnActIcon).TextColor(col)
						ui.Text(c, name).FontWeight(600).FontSize(fontBody)
					})
					ui.Text(c, ext).Font(mono).FontSize(fontCaption).TextColor(p.fgMuted)
				})
				if card.Clicked() {
					d.format = i
				}
			}
		})
		_, _, _, desc := exportFormatInfo(d.format)
		ui.Text(c, desc).FontSize(fontCaption).TextColor(p.fgMuted)
	})
	xferSection(c, p, locales.XferOptions(), func() {
		ui.Column(c.Key("xf-opts")).Gap(spaceSM + 2).FillWidth().Children(func() { t.exOptions(c, p) })
	})
}

// exOptions 是第二步的选项：按格式不同各是一组开关。
func (t *transferTab) exOptions(c *ui.Context, p tokens) {
	d := &t.w.sh.exp
	pf := &d.prefs
	switch d.format {
	case 0:
		csvSelects(c, p, pf)
		switchNot(c, p, "o-head", locales.ExportHeader(), &pf.NoHeader)
		switchNot(c, p, "o-bom", locales.ExportBOM(), &pf.NoBOM)
		switchRow(c, p, "o-crlf", locales.ExportCRLF(), &pf.CRLF)
		switchNot(c, p, "o-esc", locales.ExportEscapeFormulas(), &pf.RawFormulas)
		switchRow(c, p, "o-dec", locales.ExportDecimalComma(), &pf.DecimalComma)
		switchNot(c, p, "o-null", locales.ExportNullEmpty(), &pf.NullKeep)
	case 1:
		switchRow(c, p, "o-lines", locales.ExportJSONLines(), &pf.JSONLines)
	case 2:
		switchRow(c, p, "o-struct", locales.ExportSQLStructure(), &pf.SQLStructure)
		switchNot(c, p, "o-data", locales.ExportSQLData(), &pf.SQLNoData)
		switchRow(c, p, "o-drop", locales.ExportSQLDrop(), &pf.SQLDrop)
		if !pf.SQLNoData {
			dlgField(c, p, locales.ExportSQLBatch(), func() {
				values := []string{"1", "50", "200", "1000"}
				labels := []string{locales.ExportSQLBatchOne(), locales.ExportSQLBatchN(50), locales.ExportSQLBatchN(200), locales.ExportSQLBatchN(1000)}
				cur := fmt.Sprint(max(pf.SQLBatch, 1))
				if labelSelect(c, &cur, values, labels, 220) {
					fmt.Sscan(cur, &pf.SQLBatch)
				}
			})
		}
	default:
		ui.Text(c, locales.ExportXLSXNote()).FontSize(fontBody).TextColor(p.fgSecondary).FillWidth()
	}
	if d.format != 3 {
		switchRow(c, p, "o-gz", locales.ExportGzip(), &pf.Gzip)
	}
}
