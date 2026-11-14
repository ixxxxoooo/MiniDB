// Package app is the application layer: use cases over the domain packages.
// It knows nothing of the UI toolkit; the platform is reached through Host and
// notifications leave through Bus.
package app

import (
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"minidb/internal/logger"
)

// FileFilter restricts a file dialog, e.g. {"JSON", []string{"json"}}.
type FileFilter struct {
	Name string
	Exts []string
}

// OpenOptions configures an open-file dialog.
type OpenOptions struct {
	Title      string
	Directory  string
	ShowHidden bool
	Filters    []FileFilter
}

// SaveOptions configures a save-file dialog.
type SaveOptions struct {
	Title    string
	Filename string
	Filters  []FileFilter
}

// Host is what the application layer needs from the platform. Methods may
// block while a dialog is open and must be called off the UI thread.
type Host interface {
	// OpenFile returns the chosen path, or "" when the user cancels.
	OpenFile(OpenOptions) (string, error)
	// SaveFile returns the chosen path, or "" when the user cancels.
	SaveFile(SaveOptions) (string, error)
	// OpenPath opens a file or URL in its default application.
	OpenPath(path string) error
}

// Bus delivers events from background goroutines to subscribers. Handlers run
// on the emitting goroutine; the UI layer hops to its own thread inside them.
//
// 约定：订阅回调必须立即返回（只做"转到自己的线程 / 入队"这类事）。回调在发布者协程里同步执行，
// 一个慢回调会拖住发布者（schema 刷新、导出进度……）和排在它后面的其他订阅者。
// Emit 为此做两件事：回调 panic 不会带崩发布者，也不会影响其他订阅者；
// 回调耗时超过 busSlowHandler 会记一条警告，把违反约定的订阅者暴露出来。
type Bus struct {
	mu     sync.RWMutex
	subs   map[string][]busSub
	nextID uint64
}

// busSub 是一个订阅；id 用来退订。
type busSub struct {
	id uint64
	fn func(any)
}

func NewBus() *Bus { return &Bus{subs: map[string][]busSub{}} }

// On 订阅 topic，返回退订函数（可重复调用）。生命周期比 Bus 短的订阅者（工作区、对话框）
// 用完要调用它，否则回调会一直留在总线里。
func (b *Bus) On(topic string, fn func(payload any)) (off func()) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subs[topic] = append(b.subs[topic], busSub{id: id, fn: fn})
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subs[topic]
		for i, s := range list {
			if s.id == id {
				b.subs[topic] = append(append([]busSub(nil), list[:i]...), list[i+1:]...)
				return
			}
		}
	}
}

// Subscribe 订阅 topic 并只接收类型为 T 的载荷（其他类型忽略），返回退订函数。
func Subscribe[T any](b *Bus, topic string, fn func(T)) (off func()) {
	return b.On(topic, func(p any) {
		if v, ok := p.(T); ok {
			fn(v)
		}
	})
}

// Emit calls every subscriber of topic.
func (b *Bus) Emit(topic string, payload any) {
	b.mu.RLock()
	subs := append([]busSub(nil), b.subs[topic]...)
	b.mu.RUnlock()
	for _, s := range subs {
		deliver(topic, s.fn, payload)
	}
}

// busSlowHandler 订阅回调的耗时预算，超过就视为违反"立即返回"的约定。
const busSlowHandler = 50 * time.Millisecond

// deliver 调用一个订阅者：panic 被隔离在这一个订阅者里，耗时过长留下警告。
func deliver(topic string, fn func(any), payload any) {
	start := time.Now()
	defer func() {
		if p := recover(); p != nil {
			logger.Error("[Bus] 订阅者处理 %s 时 panic（已隔离，不影响其他订阅者）: %v\n%s", topic, p, debug.Stack())
		}
		if d := time.Since(start); d > busSlowHandler {
			logger.Warn("[Bus] 订阅者处理 %s 用了 %s，回调应该立即返回，耗时工作请放到自己的协程", topic, d)
		}
	}()
	fn(payload)
}

// Env bundles the platform and the bus; services hold one. A nil *Env is
// valid and does nothing, which keeps tests simple.
type Env struct {
	Host Host
	Bus  *Bus
}

// envRef 让服务读写 Env 时不产生数据竞争：后台协程会读它，SetEnv 随时可能换掉它。
type envRef struct{ p atomic.Pointer[Env] }

func (r *envRef) set(e *Env) { r.p.Store(e) }
func (r *envRef) get() *Env  { return r.p.Load() }

// Emit sends an event; it is a no-op on a nil Env or Bus.
func (e *Env) Emit(topic string, payload any) {
	if e != nil && e.Bus != nil {
		e.Bus.Emit(topic, payload)
	}
}
