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

// ChooseDirectory opens the native folder picker. An empty path means the
// user cancelled.
func (r *Runtime) ChooseDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(r.Context(), runtime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
	})
}
