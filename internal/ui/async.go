package ui

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"

	"github.com/egoist/mygo"
)

// uiThread schedules fn on the UI thread. Background goroutines never touch
// view state directly; they publish through it. In tests (win == nil) work
// runs inline, which keeps them deterministic.
type uiThread struct {
	win  func() *mygo.Window
	sync bool
	q    *postQueue // 仅同步模式：别的协程投递来的任务在这里排队
}

// newUIThread 创建调度器。sync 为 true 是同步模式（测试用），调用它的协程就是"UI 线程"。
func newUIThread(win func() *mygo.Window, sync bool) uiThread {
	if !sync {
		return uiThread{win: win}
	}
	return uiThread{win: win, sync: true, q: &postQueue{owner: goid()}}
}

// postQueue 让同步模式也守住"UI 状态只在 UI 线程上改"的规矩：
// 后台协程（总线事件、导入任务等）调用 post 时不直接执行，而是排队，等 UI 协程在下一帧开头执行。
type postQueue struct {
	owner uint64
	mu    sync.Mutex
	fns   []func()
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
	if u.q != nil && goid() != u.q.owner {
		u.q.mu.Lock()
		u.q.fns = append(u.q.fns, fn)
		u.q.mu.Unlock()
		return
	}
	fn()
}

// drain 执行排队的任务（只在同步模式、由 UI 协程调用；其他模式什么也不做）。
func (u uiThread) drain() {
	if u.q == nil || goid() != u.q.owner {
		return
	}
	for {
		u.q.mu.Lock()
		fns := u.q.fns
		u.q.fns = nil
		u.q.mu.Unlock()
		if len(fns) == 0 {
			return
		}
		for _, fn := range fns {
			fn()
		}
	}
}

// goid 返回当前协程的编号。只在测试用的同步模式里调用，生产路径不碰它。
func goid() uint64 {
	var buf [32]byte
	b := bytes.TrimPrefix(buf[:runtime.Stack(buf[:], false)], []byte("goroutine "))
	n, _ := strconv.ParseUint(string(b[:bytes.IndexByte(b, ' ')]), 10, 64)
	return n
}
