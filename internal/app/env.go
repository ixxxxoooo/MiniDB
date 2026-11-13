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
	mu   sync.RWMutex
	subs map[string][]func(any)
}

func NewBus() *Bus { return &Bus{subs: map[string][]func(any){}} }

// On subscribes fn to topic.
func (b *Bus) On(topic string, fn func(payload any)) {
	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], fn)
	b.mu.Unlock()
}

// Emit calls every subscriber of topic.
func (b *Bus) Emit(topic string, payload any) {
	b.mu.RLock()
	fns := append([]func(any){}, b.subs[topic]...)
	b.mu.RUnlock()
	for _, fn := range fns {
		fn(payload)
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
