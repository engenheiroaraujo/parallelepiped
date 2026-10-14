// Package desktop wraps the Wails runtime so handlers depend on a small type
// instead of the global runtime functions and the startup context.
package desktop

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Runtime holds the context Wails hands to OnStartup.
type Runtime struct {
	mu  sync.RWMutex
	ctx context.Context
}

// Startup stores the Wails context; wire it to options.App.OnStartup.
func (r *Runtime) Startup(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctx = ctx
}

// Context returns the Wails context, or Background before startup.
func (r *Runtime) Context() context.Context {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// Emit sends an event to the frontend. Before startup there is no Wails
// context and the event is dropped: the runtime functions abort the process
// when given any other context.
func (r *Runtime) Emit(name string, data any) {
	r.mu.RLock()
	ctx := r.ctx
	r.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, name, data)
}

// ChooseDirectory opens the native folder picker. An empty path means the
// user cancelled.
func (r *Runtime) ChooseDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(r.Context(), runtime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
	})
}
