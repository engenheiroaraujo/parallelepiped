// Package handler exposes note operations to the frontend through the Wails
// bridge.
package handler

import (
	"parallelepiped/internal/modules/notes/dto"
	"parallelepiped/internal/modules/notes/service"
	"parallelepiped/internal/platform/bridge"
	"parallelepiped/internal/platform/desktop"
)

// Notes is bound to the frontend; every exported method becomes a JS call.
type Notes struct {
	runtime *desktop.Runtime
	svc     *service.Service
}

func NewNotes(runtime *desktop.Runtime, svc *service.Service) *Notes {
	return &Notes{runtime: runtime, svc: svc}
}

func (h *Notes) Read(path string) (dto.NoteResponse, error) {
	note, err := h.svc.Read(h.runtime.Context(), path)
	if err != nil {
		return dto.NoteResponse{}, bridge.Error(err)
	}
	return dto.NoteFromModel(note), nil
}

func (h *Notes) Save(path, content string) (dto.NoteResponse, error) {
	note, err := h.svc.Save(h.runtime.Context(), path, content)
	if err != nil {
		return dto.NoteResponse{}, bridge.Error(err)
	}
	return dto.NoteFromModel(note), nil
}

func (h *Notes) Create(path string) (dto.NoteResponse, error) {
	note, err := h.svc.Create(h.runtime.Context(), path)
	if err != nil {
		return dto.NoteResponse{}, bridge.Error(err)
	}
	return dto.NoteFromModel(note), nil
}

func (h *Notes) Rename(from, to string) (dto.NoteResponse, error) {
	note, err := h.svc.Rename(h.runtime.Context(), from, to)
	if err != nil {
		return dto.NoteResponse{}, bridge.Error(err)
	}
	return dto.NoteFromModel(note), nil
}

func (h *Notes) Delete(path string) error {
	return bridge.Error(h.svc.Delete(h.runtime.Context(), path))
}
