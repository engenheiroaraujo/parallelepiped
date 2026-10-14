// Package dto holds the project import payloads exchanged with the frontend.
// Type names are unique across all dto packages (Wails merges them).
package dto

import "parallelepiped/internal/modules/brain/service"

// EventImportProgress is emitted while an import runs.
const EventImportProgress = "brain:progress"

type ImportProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

type ImportResultResponse struct {
	IndexNote string   `json:"index_note"`
	Units     int      `json:"units"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Unchanged int      `json:"unchanged"`
	Removed   int      `json:"removed"`
	Skipped   []string `json:"skipped"`
	Kept      []string `json:"kept"`
}

func ImportResultFrom(r service.ImportResult) ImportResultResponse {
	return ImportResultResponse{
		IndexNote: r.IndexNote,
		Units:     r.Units,
		Created:   r.Created,
		Updated:   r.Updated,
		Unchanged: r.Unchanged,
		Removed:   r.Removed,
		Skipped:   nonNil(r.Skipped),
		Kept:      nonNil(r.Kept),
	}
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
