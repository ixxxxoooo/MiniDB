package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// 导出对话框：左边是"导出什么"（表清单与列，或查询语句），右边是"怎么导出"
// （格式卡片、该格式的选项、行数上限、压缩与文件名），底部一句话总结并给出导出按钮。

// exportFormatInfo 是格式卡片上的图标、名称、扩展名与说明。
func exportFormatInfo(i int) (ic *ui.SVG, name, ext, desc string) {
	switch i {
	case 0:
		return icTable, "CSV", ".csv", locales.ExportFormatCSVDesc()
	case 1:
		return icCode, "JSON", ".json", locales.ExportFormatJSONDesc()
	case 2:
		return icDatabase, "SQL", ".sql", locales.ExportFormatSQLDesc()
	}
	return icColumns, "Excel", ".xlsx", locales.ExportFormatXLSXDesc()
}

func (s *Shell) exportDialogView(c *ui.Context) {
	d := &s.exp
	if !d.open {
		return
	}
	s.syncExportCols()
	p := s.tokens(c)
	chosen := d.chosenTables()
	var sub string
	switch {
	case d.query != "":
		sub = locales.ExportQueryResult()
	case len(chosen) == 1:
		sub = chosen[0]
	default:
		sub = locales.ExportTables()
	}
	_, fmtName, _, _ := exportFormatInfo(d.format)
	summary := locales.ExportSummaryQuery(fmtName)
	if d.query == "" {
		summary = locales.ExportSummaryTables(len(chosen), fmtName)
		if len(chosen) == 1 {
			summary = chosen[0] + " → " + fmtName
		}
	}
	s.dialog(c, &d.open, dlgOpts{title: locales.ExportTitle(), sub: sub, width: 780, height: 620,
		lead: func() {
			ui.Row(c).Size(34, 34).Shrink(0).Center().Radius(10).Background(p.accent.Alpha(0.12)).Children(func() {
				ui.Icon(c, icDownload).FontSize(17).TextColor(p.accent)
			})
		}}, func() {
		ui.Row(c).Gap(18).Grow(1).FillWidth().MinHeight(0).Children(func() {
			if d.query != "" {
				s.exportQueryPane(c, p)
			} else {
				s.exportSourcePane(c, p)
			}
			hairlineV(c, p.borderSubtle)
			scrollbars(c, ui.Scroll(c.Key("exp-body")).Grow(1).MinWidth(0).FillHeight()).Children(func() {
				ui.Column(c).Gap(16).FillWidth().Padding(0, 4, 4, 0).Children(func() { s.exportSettingsPane(c, p) })
			})
		})
	}, func() {
		ui.Text(c, summary).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().TruncateMiddle().Grow(1)
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			d.open = false
		}
		need := d.query == "" && len(chosen) == 0
		if ui.PrimaryButton(c, locales.ExportStart()).Height(sizeBtn).Disabled(need).Clicked() {
			d.open = false
			s.startExport(*d)
		}
	})
}

// hairlineV 是一条竖的细线。
func hairlineV(c *ui.Context, col ui.Color) ui.Element {
	return ui.Box(c).Width(1).FillHeight().Shrink(0).Background(col)
}

// exportSectionTitle 是右栏各节的小标题。
func exportSectionTitle(c *ui.Context, p tokens, title string) {
	ui.Text(c, title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
}

// exportQueryPane 是导出查询结果时的左栏：把要执行的语句高亮显示出来。
func (s *Shell) exportQueryPane(c *ui.Context, p tokens) {
	d := &s.exp
	ui.Column(c).Gap(8).Width(280).Shrink(0).FillHeight().Children(func() {
		exportSectionTitle(c, p, locales.ExportQuerySQL())
		dlgBox(c, p).FillWidth().Grow(1).MinHeight(0).Clip().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("exp-sql")).Grow(1).FillWidth()).Children(func() {
				ui.RichText(c, sqlSpans(p, d.query, p.fg)...).Font(mono).FontSize(font2XS).Selectable().Padding(10).FillWidth()
			})
		})
		ui.Text(c, locales.ExportQueryNote()).FontSize(font2XS - 1).TextColor(p.fgMuted).FillWidth()
	})
}

// exportSourcePane 是左栏：可筛选的表清单，只选一张表时下面多出列清单。
func (s *Shell) exportSourcePane(c *ui.Context, p tokens) {
	d := &s.exp
	type item struct {
		name string
		rows int64
	}
	var all, shown []item
	q := strings.ToLower(strings.TrimSpace(d.search))
	for _, t := range d.w.tables {
		it := item{t.Name, t.RowCount}
		all = append(all, it)
		if q == "" || strings.Contains(strings.ToLower(t.Name), q) {
			shown = append(shown, it)
		}
	}
	picked := len(d.chosenTables())
	ui.Column(c).Gap(8).Width(280).Shrink(0).FillHeight().Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).FillWidth().Children(func() {
			exportSectionTitle(c, p, locales.ExportTables())
			ui.Text(c, locales.ExportSelectedOf(picked, len(all))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			ui.Spacer(c)
			if ui.Button(c.Key("all"), locales.ExportAll()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
				for _, it := range shown {
					d.sel[it.name] = true
				}
			}
			if ui.Button(c.Key("inv"), locales.ExportInvert()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
				for _, it := range shown {
					d.sel[it.name] = !d.sel[it.name]
				}
			}
			if ui.Button(c.Key("none"), locales.ExportNone()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
				for _, it := range shown {
					d.sel[it.name] = false
				}
			}
		})
		ui.SearchField(c.Key("exp-q"), &d.search).Placeholder(locales.ExportFilterTables()).Height(sizeBtnSm).FontSize(font2XS).FillWidth()
		dlgBox(c, p).FillWidth().Grow(1).MinHeight(0).Clip().Children(func() {
			if len(shown) == 0 {
				ui.Text(c, locales.SidebarNoMatch()).FontSize(font2XS).TextColor(p.fgMuted).Padding(16).TextAlign(ui.Center).FillWidth()
				return
			}
			scrollbars(c, ui.Scroll(c.Key("exp-tables")).Grow(1).FillWidth()).Children(func() {
				ui.Column(c).Padding(4).Gap(1).FillWidth().Children(func() {
					for _, it := range shown {
						row := ui.Row(c.Key(it.name)).Gap(6).AlignItems(ui.Center).Padding(2, 6).Radius(6).FillWidth()
						if row.Hovered() {
							row.Background(p.hover)
						}
						row.Children(func() {
							v := d.sel[it.name]
							if ui.Checkbox(c.Key("cb"), &v, "").Changed() {
								d.sel[it.name] = v
							}
							ui.Icon(c, icTable).FontSize(12).TextColor(p.iconTable)
							ui.Text(c, it.name).FontSize(font2XS).SingleLine().TruncateMiddle().Grow(1).MinWidth(0)
							if it.rows > 0 {
								ui.Text(c, locales.TableRows(int(it.rows))).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Shrink(0)
							}
						})
						if row.Clicked() { // 点整行也能勾选
							d.sel[it.name] = !d.sel[it.name]
						}
					}
				})
			})
		})
		if len(d.cols) > 0 {
			on := 0
			for _, n := range d.cols {
				if d.colSel[n] {
					on++
				}
			}
			ui.Row(c).Gap(4).AlignItems(ui.Center).FillWidth().Children(func() {
				exportSectionTitle(c, p, locales.ExportColumnsOf(on, len(d.cols)))
				ui.Spacer(c)
				if ui.Button(c.Key("call"), locales.ExportAll()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
					for _, n := range d.cols {
						d.colSel[n] = true
					}
				}
				if ui.Button(c.Key("cnone"), locales.ExportNone()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
					for _, n := range d.cols {
						d.colSel[n] = false
					}
				}
			})
			dlgBox(c, p).FillWidth().Height(150).Shrink(0).Clip().Children(func() {
				scrollbars(c, ui.Scroll(c.Key("exp-cols")).Grow(1).FillWidth()).Children(func() {
					ui.Column(c).Padding(6).Gap(2).FillWidth().Children(func() {
						for _, n := range d.cols {
							v := d.colSel[n]
							if ui.Checkbox(c.Key("col-"+n), &v, n).Changed() {
								d.colSel[n] = v
							}
						}
					})
				})
			})
		}
	})
}

// exportSettingsPane 是右栏：格式卡片、该格式的选项、输出设置。
func (s *Shell) exportSettingsPane(c *ui.Context, p tokens) {
	d := &s.exp
	pf := &d.prefs

	exportSectionTitle(c, p, locales.ExportFormat())
	ui.Row(c).Gap(8).FillWidth().Children(func() {
		for i := range exportFormats {
			ic, name, ext, desc := exportFormatInfo(i)
			on := d.format == i
			card := ui.Column(c.Key(fmt.Sprint("fmt", i))).Gap(3).Padding(10, 10, 9, 10).Radius(radiusPanel).Grow(1).Basis(0).MinWidth(0).Border(1, p.border).Background(p.surface).Tooltip(desc)
			switch {
			case on:
				card.Border(1.5, p.accent).Background(p.accent.Alpha(0.07))
			case card.Hovered():
				card.Background(p.hover)
			}
			card.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					col := p.fgSecondary
					if on {
						col = p.accent
					}
					ui.Icon(c, ic).FontSize(14).TextColor(col)
					ui.Text(c, name).FontWeight(600).FontSize(font2XS + 1)
				})
				ui.Text(c, ext).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted)
			})
			if card.Clicked() {
				d.format = i
			}
		}
	})
	_, _, _, desc := exportFormatInfo(d.format)
	ui.Text(c, desc).FontSize(font2XS - 1).TextColor(p.fgMuted)

	exportSectionTitle(c, p, locales.ExportOptions())
	dlgBox(c, p).FillWidth().Padding(12).Gap(10).Children(func() {
		switch d.format {
		case 0:
			csvOptions(c, p, pf)
		case 1:
			ui.Checkbox(c, &pf.JSONLines, locales.ExportJSONLines())
		case 2:
			ui.Column(c).Gap(6).FillWidth().Children(func() {
				ui.Checkbox(c, &pf.SQLStructure, locales.ExportSQLStructure())
				ui.Checkbox(c, &pf.SQLDrop, locales.ExportSQLDrop())
				data := !pf.SQLNoData
				if ui.Checkbox(c, &data, locales.ExportSQLData()).Changed() {
					pf.SQLNoData = !data
				}
			})
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
			ui.Text(c, locales.ExportXLSXNote()).FontSize(font2XS).TextColor(p.fgSecondary).FillWidth()
		}
	})

	exportSectionTitle(c, p, locales.ExportOutput())
	dlgBox(c, p).FillWidth().Padding(12).Gap(10).Children(func() {
		dlgField(c, p, locales.ExportLimit(), func() {
			ui.TextInput(c.Key("exp-limit"), &d.limit).Placeholder(locales.ExportLimitPlaceholder()).Width(160).Height(sizeBtn)
			ui.Text(c, locales.ExportLimitHint()).FontSize(font2XS - 1).TextColor(p.fgMuted)
		})
		if d.format != 3 {
			ui.Checkbox(c, &pf.Gzip, locales.ExportGzip())
		}
		multi := d.query == "" && len(d.chosenTables()) > 1
		if multi {
			dlgField(c, p, locales.ExportPattern(), func() {
				ui.TextInput(c.Key("exp-pat"), &pf.Pattern).Placeholder("{table}_{datetime}").FillWidth().Height(sizeBtn)
				ui.Text(c, locales.ExportPatternHint("{table}", "{db}", "{date}", "{time}", "{datetime}")).FontSize(font2XS - 1).TextColor(p.fgMuted)
			})
		}
		// 文件名示例：让模板写错的人立刻看到结果
		name := "query_result"
		if t := d.chosenTables(); d.query == "" && len(t) > 0 {
			name = t[0]
		}
		opts := d.exportOptions()
		file := app.ExpandPattern(map[bool]string{true: pf.Pattern, false: ""}[multi], name, d.w.db, time.Now()) + "." + opts.Extension()
		ui.Text(c, locales.ExportFileExample(file)).Font(mono).FontSize(font2XS - 1).TextColor(p.fgSecondary).SingleLine().TruncateMiddle().FillWidth()
		ui.Text(c, locales.ExportSaveHint()).FontSize(font2XS - 1).TextColor(p.fgMuted)
	})
}
