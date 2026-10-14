package model

import "time"

// File is a note read from disk while syncing a change.
type File struct {
	Path       string
	Content    string
	ModifiedAt time.Time
}
