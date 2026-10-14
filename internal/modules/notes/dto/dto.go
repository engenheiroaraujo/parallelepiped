// Package dto holds the note payloads exchanged with the frontend.
package dto

import (
	"time"

	"parallelepiped/internal/modules/notes/model"
)

type NoteResponse struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
	// ModifiedAt is RFC 3339.
	ModifiedAt string `json:"modified_at"`
}

func NoteFromModel(n model.Note) NoteResponse {
	return NoteResponse{
		Path:       n.Path,
		Title:      n.Title,
		Content:    n.Content,
		ModifiedAt: n.ModifiedAt.Format(time.RFC3339),
	}
}
