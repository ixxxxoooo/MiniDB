package ui

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 备份、还原两个对话框的画法（设计稿「备份数据库」「恢复备份」）：标题前带图标方块，
// 表单行是「右对齐的标签 + 控件」，选项用开关，还原的风险用琥珀色提示条。
// 状态与动作（startBackup / startRestore 等）仍在 dump.go。

// dlgTile 返回对话框标题前的图标方块（强调色淡底），给 dlgOpts.lead 用。
func dlgTile(c *ui.Context, p tokens, icon *ui.SVG) func() {
	return func() {
		ui.Row(c).Size(sizeDlgTile, sizeDlgTile).Shrink(0).Center().Radius(radiusPanel).Background(p.accentSoft).Children(func() {
			ui.Icon(c, icon).FontSize(btnActIcon).TextColor(p.accent)
		})
	}
}

// dlgRow 画对话框表单的一行：左边是定宽、右对齐的标签，右边是控件 fn。
func dlgRow(c *ui.Context, p tokens, label string, fn func()) {
	ui.Row(c).Gap(spaceMD).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(fontCaption).TextColor(p.fgMuted).Width(sizeDlgLabel).TextAlign(ui.End).Shrink(0)
		ui.Column(c).Gap(spaceXS).Grow(1).MinWidth(0).Children(fn)
	})
}

// dlgNotice 是对话框里的提示条：warn 为真用琥珀色，否则用中性色。
func dlgNotice(c *ui.Context, p tokens, text string, warn bool) {
	bg, fg, icon := p.surfaceSecondary, p.fgSecondary, icInfo
	if warn {
		bg, fg, icon = p.warningBg, p.warning, icShieldAlert
	}
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(spaceSM+2, spaceMD).FillWidth().Radius(radiusPanel).Background(bg).Children(func() {
		ui.Icon(c, icon).FontSize(btnActIcon).TextColor(fg).Shrink(0)
		ui.Text(c, text).FontSize(fontDense).TextColor(p.fg).Grow(1).MinWidth(0)
	})
}

// backupView 画「备份数据库」对话框。
func (s *Shell) backupView(c *ui.Context) {
	d := &s.bak
	if !d.open {
		return
	}
	p := s.tokens(c)
	pf := &d.prefs
	sqlite := !app.CapsOf(d.w.cfg.Type).Network // 文件型库：备份就是复制文件
	content := 0
	if pf.SchemaOnly {
		content = 1
	}
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpBackupTitle(), sub: d.w.db, width: 480, lead: dlgTile(c, p, icBackup)}, func() {
		ui.Column(c).Gap(spaceMD).FillWidth().Children(func() {
			if sqlite {
				dlgNotice(c, p, locales.DumpSQLiteNote(), false)
			} else {
				dlgRow(c, p, locales.DumpContent(), func() {
					if segmented(c, &content, locales.DumpContentFull(), locales.DumpContentSchema()) {
						pf.SchemaOnly = content == 1
					}
				})
			}
			dlgRow(c, p, locales.ExportPattern(), func() {
				ui.TextInput(c.Key("bak-pat"), &pf.Pattern).Placeholder("{db}_{datetime}").FillWidth().Height(sizeInput)
				ui.Text(c, locales.DumpPatternHint("{db}", "{date}", "{time}", "{datetime}")).FontSize(fontCaption).TextColor(p.fgMuted)
			})
			if !sqlite {
				dlgRow(c, p, locales.DumpExtraArgs(), func() {
					ui.TextInput(c.Key("bak-args"), &pf.ExtraArgs).Placeholder(locales.DumpExtraArgsHint()).FillWidth().Height(sizeInput)
				})
				dlgRow(c, p, locales.DumpTool(), func() {
					ui.TextInput(c.Key("bak-tool"), &pf.Tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeInput)
				})
				switchRow(c, p, "bak-gz", locales.ExportGzip(), &pf.Gzip)
			}
			if d.err != "" {
				ui.Text(c, d.err).TextColor(p.danger).FontSize(fontCaption).Selectable()
			}
		})
	}, func() {
		ui.Text(c, locales.DumpFooterTables(len(d.w.tables))).FontSize(fontCaption).TextColor(p.fgMuted).Grow(1)
		if d.busy {
			ui.Spinner(c)
		}
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		if footPrimary(c, locales.DumpStart()).Disabled(d.busy).Clicked() {
			s.startBackup()
		}
	})
}

// restoreView 画「还原数据库」对话框。
func (s *Shell) restoreView(c *ui.Context) {
	d := &s.rst
	if !d.open {
		return
	}
	p := s.tokens(c)
	seg := 0
	if d.newDB {
		seg = 1
	}
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpRestoreTitle(), sub: d.w.db, width: 480, lead: dlgTile(c, p, icUpload)}, func() {
		ui.Column(c).Gap(spaceMD).FillWidth().Children(func() {
			xferCard(c, p).Padding(spaceSM+2, spaceMD+2).Background(p.surface).Children(func() {
				ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).FillWidth().Children(func() {
					ui.Icon(c, icFileSQL).FontSize(btnActIcon).TextColor(p.fgMuted).Shrink(0)
					name, col := locales.ImportFile(), p.fgMuted
					if d.path != "" {
						name, col = filepath.Base(d.path), p.fg
					}
					ui.Text(c, name).Font(mono).FontSize(fontCtl).FontWeight(600).TextColor(col).SingleLine().TruncateMiddle().Grow(1).MinWidth(0)
					label := locales.DumpChoose()
					if d.path != "" {
						label = locales.XferChange()
					}
					if fieldBtn(c, icFolder, label).Clicked() {
						s.pickRestoreFile()
					}
				})
			})
			dlgRow(c, p, locales.DumpRestoreTo(), func() {
				if segmented(c, &seg, locales.DumpTargetCurrent(), locales.DumpTargetNew()) {
					d.newDB = seg == 1
				}
				if d.newDB {
					ui.TextInput(c.Key("rst-new"), &d.newName).Placeholder(locales.DumpNewName()).FillWidth().Height(sizeInput)
				}
			})
			dlgRow(c, p, locales.DumpExtraArgs(), func() {
				ui.TextInput(c.Key("rst-args"), &d.extraArgs).FillWidth().Height(sizeInput)
			})
			dlgRow(c, p, locales.DumpTool(), func() {
				ui.TextInput(c.Key("rst-tool"), &d.tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeInput)
			})
			dlgNotice(c, p, locales.DumpRestoreWarn(), true)
			if d.err != "" {
				ui.Text(c, d.err).TextColor(p.danger).FontSize(fontCaption).Selectable()
			}
		})
	}, func() {
		ui.Spacer(c)
		if d.busy {
			ui.Spinner(c)
		}
		if footBtn(c, locales.CommonCancel()).Clicked() {
			d.open = false
		}
		if footPrimary(c, locales.DumpRestoreStart()).Disabled(d.busy).Clicked() {
			s.confirmRestore()
		}
	})
}
