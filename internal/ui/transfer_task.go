package ui

import (
	"os"
	"path/filepath"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 导出 / 导入向导里「执行」那一步要看的运行状态。后台任务的进度和结果都走 app 总线事件（tasks.go），
// 这里把和向导有关的那一份记下来，让向导页能画出进度条、结果和「打开文件夹」。

// xferEvent 是一条任务事件里向导关心的部分。
type xferEvent struct {
	id, status, err, path string
	cur, total            int64
}

// xferRun 是向导里一次运行的状态：id 为空表示还没有启动。
type xferRun struct {
	xferEvent           // 当前这次运行的最新状态
	pending   xferEvent // 最近收到的事件：启动调用返回之前到达的进度先放在这里，begin 时补上
	active    bool
}

// begin 在后台任务启动成功后调用，id 是任务号。
func (r *xferRun) begin(id string) {
	r.xferEvent, r.active = xferEvent{id: id}, true
	if r.pending.id == id {
		r.xferEvent = r.pending
	}
}

// note 记下一条任务事件；是当前这次运行的就更新状态。
func (r *xferRun) note(ev xferEvent) {
	r.pending = ev
	if r.active && r.id == ev.id {
		r.xferEvent = ev
	}
}

// running 说明任务还在跑。
func (r *xferRun) running() bool {
	return r.active && (r.status == "" || r.status == "progress")
}

// fraction 是进度（0~1）；总数未知时返回 -1，进度条走不确定态。
func (r *xferRun) fraction() float64 {
	if r.total <= 0 {
		return -1
	}
	return min(1, float64(r.cur)/float64(r.total))
}

// bar 是进度条的值：完成了就是满的，否则是 fraction。
func (r *xferRun) bar() float64 {
	if r.status == "done" {
		return 1
	}
	return r.fraction()
}

// noteTransfer 由 tasks.go 在每条任务事件到达时调用，把导出 / 导入的事件交给向导。
func (s *Shell) noteTransfer(kind, id, status string, cur, total int64, errMsg, path string) {
	ev := xferEvent{id: id, status: status, err: errMsg, path: path, cur: cur, total: total}
	switch kind {
	case "export":
		s.exp.xfer.note(ev)
	case "import":
		s.imp.xfer.note(ev)
	}
}

// revealPath 在文件管理器里打开导出结果：单个文件打开它所在的文件夹，目录就直接打开。
func (s *Shell) revealPath(path string) {
	if path == "" {
		return
	}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		path = filepath.Dir(path)
	}
	if err := s.host.OpenPath(path); err != nil {
		s.toastErr(errMessage(err))
	}
}

// pickExportDest 弹保存对话框选导出位置：单表是文件路径，多表取它所在的目录。
func (s *Shell) pickExportDest() {
	d := &s.exp
	chosen := d.chosenTables()
	name := "export"
	if len(chosen) == 1 {
		name = chosen[0]
	}
	opts := d.exportOptions()
	file := name + "." + opts.Extension()
	var path string
	var err error
	s.up.bg(func() { path, err = s.host.SaveFile(app.SaveOptions{Title: locales.ExportTitle(), Filename: file}) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		if path != "" {
			d.dest, d.destMulti = path, len(chosen) != 1
		}
	})
}
