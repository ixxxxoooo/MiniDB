package ui

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 备份、还原向导（设计稿：和导出 / 导入同一套弹窗骨架，各三步）：
//   - 备份：选项（内容、压缩）→ 位置（文件名、工具；点「开始备份」才弹系统的保存对话框）→ 执行（已开始，进度在底部任务里）；
//   - 还原：选择备份（文件）→ 目标与选项（还原到哪个库、工具、风险提示；点「开始还原」先确认）→ 执行。
// 实际的备份 / 还原由外部工具在后台跑，进度在状态栏的任务里，所以「执行」步只告诉用户已经开始。

// backupView 画备份向导。
func (s *Shell) backupView(c *ui.Context) {
	d := &s.bak
	if !d.open {
		return
	}
	p := s.tokens(c)
	pf := &d.prefs
	sqlite := !app.CapsOf(d.w.cfg.Type).Network // 文件型库：备份就是复制文件
	names := []string{locales.DumpStepOptions(), locales.DumpStepLocation(), locales.DumpStepRun()}
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpBackupTitle(), sub: d.w.dbLabel(), width: sizeWiz, height: sizeWizH, lead: dlgTile(c, p, icBackup)}, func() {
		wizSteps(c, p, names, d.step, func(i int) bool { return i < 2 }, func(i int) { d.step = i })
		scrollbars(c, ui.Scroll(c.Key("bak-body")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c.Key(fmtStep("bak", d.step))).Gap(spaceMD).Padding(spaceMD, spaceXS).FillWidth().Children(func() {
				switch d.step {
				case 0:
					if sqlite {
						dlgNotice(c, p, locales.DumpSQLiteNote(), false)
					} else {
						content := 0
						if pf.SchemaOnly {
							content = 1
						}
						dlgRow(c, p, locales.DumpContent(), func() {
							if segmented(c, &content, locales.DumpContentFull(), locales.DumpContentSchema()) {
								pf.SchemaOnly = content == 1
							}
						})
						switchRow(c, p, "bak-gz", locales.ExportGzip(), &pf.Gzip)
					}
				case 1:
					dlgRow(c, p, locales.ExportPattern(), func() {
						textField(c.Key("bak-pat"), &pf.Pattern).Placeholder("{db}_{datetime}").FillWidth().Height(sizeInput)
						ui.Text(c, locales.DumpPatternHint("{db}", "{date}", "{time}", "{datetime}")).FontSize(fontCaption).TextColor(p.fgMuted)
					})
					if !sqlite {
						dlgRow(c, p, locales.DumpExtraArgs(), func() {
							textField(c.Key("bak-args"), &pf.ExtraArgs).Placeholder(locales.DumpExtraArgsHint()).FillWidth().Height(sizeInput)
						})
						dlgRow(c, p, locales.DumpTool(), func() {
							textField(c.Key("bak-tool"), &pf.Tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeInput)
						})
					}
				default:
					dlgNotice(c, p, locales.DumpStartedBackup(d.started), false)
					ui.Text(c, locales.DumpSeeTasks()).FontSize(fontBody).TextColor(p.fgMuted)
				}
				if d.err != "" {
					ui.Text(c, d.err).TextColor(p.danger).FontSize(fontCaption).Selectable()
				}
			})
		})
	}, func() {
		wizFoot(c, p, &d.step, 2, locales.DumpFooterTables(len(d.w.tables)), d.busy, locales.DumpStart(), false, func() { s.startBackup() }, func() { d.open = false })
	})
}

// restoreView 画还原向导。
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
	names := []string{locales.DumpStepRstFile(), locales.DumpStepRstTarget(), locales.DumpStepRun()}
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpRestoreTitle(), sub: d.w.dbLabel(), width: sizeWiz, height: sizeWizH, lead: dlgTile(c, p, icUpload)}, func() {
		wizSteps(c, p, names, d.step, func(i int) bool { return i < 2 && (i == 0 || d.path != "") }, func(i int) { d.step = i })
		scrollbars(c, ui.Scroll(c.Key("rst-body")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c.Key(fmtStep("rst", d.step))).Gap(spaceMD).Padding(spaceMD, spaceXS).FillWidth().Children(func() {
				switch d.step {
				case 0:
					s.restoreFileCard(c, p, d)
				case 1:
					s.restoreFileCard(c, p, d)
					dlgRow(c, p, locales.DumpRestoreTo(), func() {
						if segmented(c, &seg, locales.DumpTargetCurrent(), locales.DumpTargetNew()) {
							d.newDB = seg == 1
						}
						if d.newDB {
							textField(c.Key("rst-new"), &d.newName).Placeholder(locales.DumpNewName()).FillWidth().Height(sizeInput)
						}
					})
					dlgRow(c, p, locales.DumpExtraArgs(), func() {
						textField(c.Key("rst-args"), &d.extraArgs).FillWidth().Height(sizeInput)
					})
					dlgRow(c, p, locales.DumpTool(), func() {
						textField(c.Key("rst-tool"), &d.tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeInput)
					})
					dlgNotice(c, p, locales.DumpRestoreWarn(), true)
				default:
					dlgNotice(c, p, locales.DumpStartedRestore(d.startedTo), false)
					ui.Text(c, locales.DumpSeeTasks()).FontSize(fontBody).TextColor(p.fgMuted)
				}
				if d.err != "" {
					ui.Text(c, d.err).TextColor(p.danger).FontSize(fontCaption).Selectable()
				}
			})
		})
	}, func() {
		hint := ""
		if d.path != "" {
			hint = filepath.Base(d.path)
		}
		wizFoot(c, p, &d.step, 2, hint, d.busy, locales.DumpRestoreStart(), d.path == "" && d.step == 0, func() { s.confirmRestore() }, func() { d.open = false })
	})
}

// restoreFileCard 画还原向导里的备份文件卡：文件名（没选时是提示）和「选择 / 更换」。
func (s *Shell) restoreFileCard(c *ui.Context, p tokens, d *restoreDialog) {
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
}

// wizFoot 画备份 / 还原向导的页脚：左边一句概括，右边「上一步 / 下一步」，倒数第二步的主按钮是真正的动作（run），最后一步是「完成」。
// 参数：step 当前步；last 最后一步的序号；hint 左边的概括；busy 为真时转圈；main 主动作的文字；noNext 为真时「下一步」灰着；
// run 是主动作；done 在点「完成」或第一步的「取消」时调用。
func wizFoot(c *ui.Context, p tokens, step *int, last int, hint string, busy bool, main string, noNext bool, run func(), done func()) {
	ui.Text(c, hint).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().TruncateMiddle().Grow(1)
	if busy {
		ui.Spinner(c)
	}
	switch {
	case *step == last:
		if footPrimary(c, locales.DumpDone()).Clicked() {
			done()
		}
		return
	case *step == 0:
		if footBtn(c, locales.CommonCancel()).Clicked() {
			done()
		}
	default:
		if footBtn(c, locales.XferPrev()).Clicked() {
			*step--
		}
	}
	if *step == last-1 {
		if footPrimary(c, main).Disabled(busy).Clicked() {
			run()
		}
		return
	}
	if footPrimary(c, locales.XferNext()).Disabled(noNext).Clicked() {
		*step++
	}
}

// fmtStep 给向导每一步的内容列取不同的键，换步时才会重新播放进场动画。
func fmtStep(prefix string, step int) string { return prefix + "-step" + itoa(step) }
