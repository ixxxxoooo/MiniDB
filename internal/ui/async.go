package ui

import "github.com/egoist/mygo"

// uiThread schedules fn on the UI thread. Background goroutines never touch
// view state directly; they publish through it. In tests (win == nil) work
// runs inline, which keeps them deterministic.
type uiThread struct {
	win  func() *mygo.Window
	sync bool
}

// bg runs work off the UI thread, then done on it.
func (u uiThread) bg(work func(), done func()) {
	if u.sync || u.win() == nil {
		work()
		done()
		return
	}
	go func() {
		work()
		u.post(done)
	}()
}

// post runs fn on the UI thread.
func (u uiThread) post(fn func()) {
	if w := u.win(); w != nil && !u.sync {
		w.Update(fn)
		return
	}
	fn()
}
