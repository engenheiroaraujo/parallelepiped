// Package handler exposes the project import to the frontend through the
// Wails bridge and reports its progress as events.
package handler

import (
	"parallelepiped/internal/modules/brain/dto"
	"parallelepiped/internal/modules/brain/service"
	"parallelepiped/internal/platform/bridge"
	"parallelepiped/internal/platform/desktop"
)

// Brain is bound to the frontend; every exported method becomes a JS call.
type Brain struct {
	runtime *desktop.Runtime
	svc     *service.Service
}

func NewBrain(runtime *desktop.Runtime, svc *service.Service) *Brain {
	return &Brain{runtime: runtime, svc: svc}
}

// ImportProject maps the code of the open vault into notes under docs/cerebro.
func (h *Brain) ImportProject() (dto.ImportResultResponse, error) {
	result, err := h.svc.Import(h.runtime.Context())
	if err != nil {
		return dto.ImportResultResponse{}, bridge.Error(err)
	}
	return dto.ImportResultFrom(result), nil
}

// Notifier emits import progress through the Wails runtime. It is not bound.
type Notifier struct {
	runtime *desktop.Runtime
}

func NewNotifier(runtime *desktop.Runtime) *Notifier {
	return &Notifier{runtime: runtime}
}

// ImportProgress emits dto.EventImportProgress.
func (n *Notifier) ImportProgress(done, total int) {
	n.runtime.Emit(dto.EventImportProgress, dto.ImportProgress{Done: done, Total: total})
}
