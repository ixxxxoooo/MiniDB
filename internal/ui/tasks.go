package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// task is a running export or import, fed by progress events.
type task struct {
	id      string
	kind    string // "export" | "import"
	name    string
	current int64
	total   int64
	cancel  func()
}

func (s *Shell) subscribeTasks() {
	s.app.Bus.On(app.TopicExport, func(p any) {
		if ev, ok := p.(app.ExportProgressEvent); ok {
			s.up.post(func() {
				s.taskEvent("export", ev.TaskID, ev.FileName, ev.Status, ev.Current, ev.Total, ev.Error, ev.FilePath, false)
			})
		}
	})
	s.app.Bus.On(app.TopicImport, func(p any) {
		if ev, ok := p.(app.ImportProgressEvent); ok {
			s.up.post(func() {
				s.taskEvent("import", ev.TaskID, ev.FileName, ev.Status, ev.Current, ev.Total, ev.Error, "", ev.RolledBack)
			})
		}
	})
}

func (s *Shell) findTask(id string) *task {
	for _, t := range s.tasks {
		if t.id == id {
			return t
		}
	}
	return nil
}

func (s *Shell) taskEvent(kind, id, name, status string, cur, total int64, errMsg, path string, rolledBack bool) {
	t := s.findTask(id)
	if t == nil {
		t = &task{id: id, kind: kind, name: name}
		t.cancel = func() {
			if kind == "export" {
				s.app.Exports.CancelExport(id)
			} else {
				s.app.Imports.CancelImport(id)
			}
		}
		s.tasks = append(s.tasks, t)
	}
	t.current, t.total = cur, total
	if name != "" {
		t.name = name
	}
	if status == "progress" || status == "" {
		return
	}
	// finished: drop it and say how it went
	for i, x := range s.tasks {
		if x == t {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			break
		}
	}
	switch status {
	case "done":
		if kind == "export" {
			s.toastAction(locales.TaskExportDone(t.name), locales.TaskReveal(), func() { _ = s.host.OpenPath(path) })
		} else {
			s.toast(locales.TaskImportDone(t.name))
			s.refreshTables()
		}
	case "cancelled":
		msg := locales.TaskCancelled(t.name)
		if rolledBack {
			msg += locales.TaskRolledBack()
		}
		s.toast(msg)
	default:
		msg := locales.TaskFailed(t.name, errMsg)
		if rolledBack {
			msg += locales.TaskRolledBack()
		}
		s.toast(msg)
	}
}

// refreshTables reloads the data of the open tabs after an import.
func (s *Shell) refreshTables() {
	for _, w := range s.spaces {
		for _, tb := range w.tabs {
			if tt, ok := tb.(*tableTab); ok && tt.pending() == 0 {
				tt.load()
			}
		}
	}
}

// taskBar shows running tasks in the status bar.
func (s *Shell) taskBar(c *ui.Context) {
	th := c.Theme()
	for _, t := range s.tasks {
		t := t
		ui.Row(c.Key(t.id)).Gap(6).AlignItems(ui.Center).Children(func() {
			title := locales.TaskExport(t.name)
			if t.kind == "import" {
				title = locales.TaskImport(t.name)
			}
			ui.Text(c, title).FontSize(12).TextColor(th.TextMuted).SingleLine()
			if t.total > 0 {
				ui.Progress(c, float64(t.current)/float64(t.total)).Width(90)
			} else {
				ui.Spinner(c)
			}
			if iconButton(c, icX, locales.TaskCancel()).Size(18, 18).Clicked() {
				t.cancel()
			}
		})
	}
}
