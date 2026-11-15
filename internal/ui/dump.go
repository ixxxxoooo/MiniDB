package ui

import (
	"fmt"
	"path/filepath"
	"strings"

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
	// 向导：当前第几步（选项 → 位置 → 执行），以及备份开始后选定的保存位置
	step    int
	started string
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
	// 向导：当前第几步（选择备份 → 目标与选项 → 执行），以及开始后实际写入的库
	step      int
	startedTo string
}

func (s *Shell) openBackup() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.bak = dumpDialog{open: true, w: w, prefs: s.prefs.Dump} // 从第一步开始
}

func (s *Shell) openRestore() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.rst = restoreDialog{open: true, w: w, tool: s.prefs.Dump.Tool}
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
			d.err = errMessage(err)
			return
		}
		if started != "" { // 空串表示用户取消了位置选择，留在这一步
			d.step, d.started = 2, started
		}
	})
}

func (s *Shell) pickRestoreFile() {
	d := &s.rst
	var path string
	var err error
	s.up.bg(func() { path, err = s.app.Dumps.PickDumpFile() }, func() {
		if err != nil {
			d.err = errMessage(err)
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
			d.err = errMessage(err)
			return
		}
		d.step, d.startedTo = 2, target
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
