package ui

import (
	"sync"
)

// stateWriter 是串行、合并同键写入的写盘队列。sync 为真（测试）时就地执行。
//
// 每份状态的写都是完整快照，所以同一个键排着队的旧写入可以直接被新的取代：
// 后台写得慢时队列也只会有"每个键最多一条"，do 永远不会阻塞界面线程。
type stateWriter struct {
	sync    bool
	mu      sync.Mutex
	cond    *sync.Cond
	pending map[string]func() // 键 -> 还没执行的最新写入
	order   []string          // 键的先后顺序（先排的先写）
	busy    bool              // 后台协程正在执行某个写入
}

// do 排一个写盘任务；key 相同且还没开始写的旧任务会被它取代。
func (w *stateWriter) do(key string, fn func()) {
	if w.sync {
		fn()
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cond == nil {
		w.cond = sync.NewCond(&w.mu)
		w.pending = map[string]func(){}
		go w.run()
	}
	if _, queued := w.pending[key]; !queued {
		w.order = append(w.order, key)
	}
	w.pending[key] = fn
	w.cond.Broadcast()
}

// run 是后台写盘协程：每次取最早排队的键，执行它最新的那一份写入。
func (w *stateWriter) run() {
	w.mu.Lock()
	for {
		for len(w.order) == 0 {
			w.cond.Wait()
		}
		key := w.order[0]
		w.order = w.order[1:]
		fn := w.pending[key]
		delete(w.pending, key)
		w.busy = true
		w.mu.Unlock()
		fn()
		w.mu.Lock()
		w.busy = false
		w.cond.Broadcast()
	}
}

// wait 等队列里已有的任务都做完。
func (w *stateWriter) wait() {
	if w.sync {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for w.cond != nil && (len(w.order) > 0 || w.busy) {
		w.cond.Wait()
	}
}
