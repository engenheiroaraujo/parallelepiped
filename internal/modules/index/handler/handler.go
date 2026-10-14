// Package handler exposes the index to the frontend through the Wails bridge.
package handler

import (
	"parallelepiped/internal/modules/index/dto"
	"parallelepiped/internal/modules/index/service"
	"parallelepiped/internal/platform/bridge"
	"parallelepiped/internal/platform/desktop"
)

// Index is bound to the frontend; every exported method becomes a JS call.
type Index struct {
	runtime *desktop.Runtime
	svc     *service.Service
}

func NewIndex(runtime *desktop.Runtime, svc *service.Service) *Index {
	return &Index{runtime: runtime, svc: svc}
}

// Graph returns every note and link for the graph view.
func (h *Index) Graph() (dto.GraphResponse, error) {
	graph, err := h.svc.Graph(h.runtime.Context())
	if err != nil {
		return dto.GraphResponse{}, bridge.Error(err)
	}
	return dto.GraphFromModel(graph), nil
}

// Backlinks lists the notes linking to path.
func (h *Index) Backlinks(path string) (dto.BacklinksResponse, error) {
	items, err := h.svc.Backlinks(h.runtime.Context(), path)
	if err != nil {
		return dto.BacklinksResponse{}, bridge.Error(err)
	}
	return dto.BacklinksFromModel(items), nil
}

// Search runs a full-text search; limit <= 0 uses the default.
func (h *Index) Search(query string, limit int) (dto.SearchResponse, error) {
	hits, err := h.svc.Search(h.runtime.Context(), query, limit)
	if err != nil {
		return dto.SearchResponse{}, bridge.Error(err)
	}
	return dto.SearchFromModel(hits), nil
}
