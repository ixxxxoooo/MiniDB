package ui

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"

	"github.com/egoist/mygo"

	"minidb/internal/logger"
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
// 两段都自动打点，名字取自调用 bg 的函数：bg.<函数>（后台部分）和 ui.done.<函数>（收尾，在 UI 线程上）。
func (u uiThread) bg(work func(), done func()) { u.bgSite(perfSite(2), true, work, done) }

// bgSite 是 bg 的底层：site 是打点用的名字（通常是发起者的函数名）。
// 泛型封装（loader 等）替发起者调用它，这样日志里看到的是真正的发起者；
// timeWork 为 false 时不给后台部分计时，由 work 自己量真正干活的那一段（比如先故意等一会儿再取数的加载）。
func (u uiThread) bgSite(site string, timeWork bool, work func(), done func()) {
	if u.sync || u.win() == nil {
		work()
		done()
		return
	}
	go func() {
		if timeWork {
			end := logger.Perf("bg." + site)
			work()
			end()
		} else {
			work()
		}
		u.postNamed("ui.done."+site, done)
	}()
}

// post runs fn on the UI thread.
func (u uiThread) post(fn func()) { u.postNamed("ui.post."+perfSite(2), fn) }

// postNamed 把 fn 投递到 UI 线程，并以 name 给排队延迟和执行耗时打点。
func (u uiThread) postNamed(name string, fn func()) {
	if w := u.win(); w != nil && !u.sync {
		waited := queueWatch(name)
		w.Update(func() {
			waited()
			defer logger.PerfUI(name)()
			fn()
		})
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
