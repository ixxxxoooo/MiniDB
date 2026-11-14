package ui

import (
	"fmt"
	"minidb/internal/database"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// dumpPrefs 记住上次的备份选项。
type dumpPrefs struct {
	Gzip       bool   `json:"gzip,omitempty"`
	SchemaOnly bool   `json:"schemaOnly,omitempty"`
	ExtraArgs  string `json:"extraArgs,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
}

// dumpDialog 是备份对话框。
type dumpDialog struct {
	open  bool
	w     *Workspace
	prefs dumpPrefs
	err   string
	busy  bool
}

// restoreDialog 是还原对话框。
type restoreDialog struct {
	open      bool
	w         *Workspace
	path      string
	newDB     bool
	newName   string
	tool      string
	extraArgs string
	err       string
	busy      bool
}

func (s *Shell) openBackup() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.bak = dumpDialog{open: true, w: w, prefs: s.prefs.Dump}
}

func (s *Shell) openRestore() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.rst = restoreDialog{open: true, w: w, tool: s.prefs.Dump.Tool}
}

func (s *Shell) backupView(c *ui.Context) {
	d := &s.bak
	if !d.open {
		return
	}
	p := s.tokens(c)
	pf := &d.prefs
	sqlite := !database.CapsOf(d.w.cfg.Type).Network // 文件型库：备份就是复制文件
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpBackupTitle(), sub: d.w.db, width: 480}, func() {
		ui.Column(c).Gap(12).FillWidth().Children(func() {
			if sqlite {
				ui.Text(c, locales.DumpSQLiteNote()).FontSize(font2XS).TextColor(p.fgMuted)
			} else {
				ui.Column(c).Gap(6).FillWidth().Children(func() {
					ui.Checkbox(c, &pf.Gzip, locales.ExportGzip())
					ui.Checkbox(c, &pf.SchemaOnly, locales.DumpSchemaOnly())
				})
				dlgField(c, p, locales.DumpExtraArgs(), func() {
					ui.TextInput(c.Key("bak-args"), &pf.ExtraArgs).Placeholder(locales.DumpExtraArgsHint()).FillWidth().Height(sizeBtn)
				})
				dlgField(c, p, locales.DumpTool(), func() {
					ui.TextInput(c.Key("bak-tool"), &pf.Tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeBtn)
				})
			}
			dlgField(c, p, locales.ExportPattern(), func() {
				ui.TextInput(c.Key("bak-pat"), &pf.Pattern).Placeholder("{db}_{datetime}").FillWidth().Height(sizeBtn)
				ui.Text(c, locales.DumpPatternHint("{db}", "{date}", "{time}", "{datetime}")).FontSize(font2XS).TextColor(p.fgMuted)
			})
			if d.err != "" {
				ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
			}
		})
	}, func() {
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

func (s *Shell) startBackup() {
	d := &s.bak
	w := d.w
	s.prefs.Dump = d.prefs
	s.savePrefs()
	req := app.DumpRequest{
		ConnID: w.cfg.ID, Database: w.db, Pattern: d.prefs.Pattern,
		Options: app.DumpOptions{Tool: d.prefs.Tool, ExtraArgs: d.prefs.ExtraArgs, Gzip: d.prefs.Gzip, SchemaOnly: d.prefs.SchemaOnly},
	}
	d.busy, d.err = true, ""
	var started string
	var err error
	// 保存对话框会阻塞，所以放在后台
	s.up.bg(func() { started, err = s.app.Dumps.StartBackup(req) }, func() {
		d.busy = false
		if err != nil {
			d.err = err.Error()
			return
		}
		d.open = false
		_ = started // 空串表示用户取消了位置选择，什么也不用做
	})
}

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
	s.dialog(c, &d.open, dlgOpts{title: locales.DumpRestoreTitle(), sub: d.w.db, width: 480}, func() {
		ui.Column(c).Gap(12).FillWidth().Children(func() {
			dlgField(c, p, locales.ImportFile(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
					ui.Text(c, d.path).SingleLine().TruncateMiddle().Grow(1).TextColor(p.fgSecondary).Padding(0, 8).Height(sizeBtn).Background(p.surfaceSecondary).Radius(radiusBtn)
					if fieldBtn(c, icFolder, locales.DumpChoose()).Clicked() {
						s.pickRestoreFile()
					}
				})
			})
			dlgField(c, p, locales.ImportTarget(), func() {
				if ui.Segmented(c, &seg, locales.DumpTargetCurrent(), locales.DumpTargetNew()).Changed() {
					d.newDB = seg == 1
				}
				if d.newDB {
					ui.TextInput(c.Key("rst-new"), &d.newName).Placeholder(locales.DumpNewName()).FillWidth().Height(sizeBtn)
				}
			})
			dlgField(c, p, locales.DumpExtraArgs(), func() {
				ui.TextInput(c.Key("rst-args"), &d.extraArgs).FillWidth().Height(sizeBtn)
			})
			dlgField(c, p, locales.DumpTool(), func() {
				ui.TextInput(c.Key("rst-tool"), &d.tool).Placeholder(locales.DumpToolHint()).FillWidth().Height(sizeBtn)
			})
			ui.Text(c, locales.DumpRestoreWarn()).FontSize(font2XS).TextColor(p.fgMuted)
			if d.err != "" {
				ui.Text(c, d.err).TextColor(p.danger).FontSize(font2XS).Selectable()
			}
		})
	}, func() {
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

func (s *Shell) pickRestoreFile() {
	d := &s.rst
	var path string
	var err error
	s.up.bg(func() { path, err = s.app.Dumps.PickDumpFile() }, func() {
		if err != nil {
			d.err = err.Error()
			return
		}
		if path != "" {
			d.path, d.err = path, ""
		}
	})
}

// restoreTarget 是还原到的库名。
func (d *restoreDialog) restoreTarget() string {
	if d.newDB {
		return strings.TrimSpace(d.newName)
	}
	return d.w.db
}

// confirmRestore 先确认，再过安全模式的口令关，然后开始。
func (s *Shell) confirmRestore() {
	d := &s.rst
	switch {
	case d.path == "":
		d.err = locales.DumpNeedFile()
		return
	case d.newDB && d.restoreTarget() == "":
		d.err = locales.DumpNewName()
		return
	}
	target := d.restoreTarget()
	msg := locales.DumpRestoreConfirm(filepath.Base(d.path), target) + "\n" + locales.DumpRestoreWarn()
	w := d.w
	s.askWarn(locales.DumpRestoreTitle(), msg, locales.DumpRestoreStart(), func() {
		s.safeGatePassword(w, []string{"DROP DATABASE " + target}, s.startRestore)
	})
}

func (s *Shell) startRestore() {
	d := &s.rst
	w := d.w
	target := d.restoreTarget()
	req := app.RestoreRequest{ConnID: w.cfg.ID, Database: target, FilePath: d.path, Tool: d.tool, ExtraArgs: d.extraArgs}
	create := d.newDB
	d.busy, d.err = true, ""
	var err error
	s.up.bg(func() {
		if create {
			// 先检查，免得建好了空库才发现没装工具
			if err = s.app.Dumps.CheckRestore(req); err != nil {
				return
			}
			if err = s.app.Databases.CreateDatabase(w.cfg.ID, target, ""); err != nil {
				return
			}
		}
		_, err = s.app.Dumps.StartRestore(req)
	}, func() {
		d.busy = false
		if err != nil {
			d.err = err.Error()
			return
		}
		d.open = false
		if create {
			s.reloadDatabases(w)
		}
	})
}

// humanBytes 把字节数写成 1.5 MB 这样的短文字。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
