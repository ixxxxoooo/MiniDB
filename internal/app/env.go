// Package app is the application layer: use cases over the domain packages.
// It knows nothing of the UI toolkit; the platform is reached through Host and
// notifications leave through Bus.
package app

import "sync"

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
		s.fn(payload)
	}
}

// Env bundles the platform and the bus; services hold one. A nil *Env is
// valid and does nothing, which keeps tests simple.
type Env struct {
	Host Host
	Bus  *Bus
}

// Emit sends an event; it is a no-op on a nil Env or Bus.
func (e *Env) Emit(topic string, payload any) {
	if e != nil && e.Bus != nil {
		e.Bus.Emit(topic, payload)
	}
}
