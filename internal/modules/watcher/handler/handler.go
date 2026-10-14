// Package handler pushes watcher notifications to the frontend as Wails
// events. Unlike the other modules' handlers it is not bound: the frontend
// never calls it, it only listens.
package handler

import (
	"parallelepiped/internal/modules/watcher/dto"
	"parallelepiped/internal/platform/desktop"
)

// Notifier emits vault change events through the Wails runtime.
type Notifier struct {
	runtime *desktop.Runtime
}

func NewNotifier(runtime *desktop.Runtime) *Notifier {
	return &Notifier{runtime: runtime}
}

// VaultChanged emits dto.EventVaultChanged with the affected paths.
func (n *Notifier) VaultChanged(paths []string) {
	n.runtime.Emit(dto.EventVaultChanged, dto.VaultChanged{Paths: paths})
}
