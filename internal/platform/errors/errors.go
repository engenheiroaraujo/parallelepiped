// Package errs holds the error categories and the single catalog of named
// sentinel errors used across the application.
package errs

import (
	"errors"
	"fmt"
)

// Category is the closed set of error kinds the desktop bridge understands.
type Category string

const (
	InvalidInput Category = "invalid_input"
	NotFound     Category = "not_found"
	Conflict     Category = "conflict"
	Unavailable  Category = "unavailable"
	Internal     Category = "internal"
)

// Error is a classified error. Message is safe to show to the user unless the
// category is Internal; Details carries per-field validation messages.
type Error struct {
	Category Category
	Message  string
	Details  map[string]any
	Err      error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// New builds a classified error without a cause.
func New(category Category, message string) error {
	return &Error{Category: category, Message: message}
}

// Validation builds an invalid_input error with per-field details.
func Validation(message string, details map[string]any) error {
	return &Error{Category: InvalidInput, Message: message, Details: details}
}

// Wrap classifies err with category, discarding any previous category.
func Wrap(category Category, message string, err error) error {
	return &Error{Category: category, Message: message, Err: err}
}

// PreserveOr keeps the category of an already classified error, or applies
// category when err has none yet.
func PreserveOr(category Category, message string, err error) error {
	if err == nil {
		return nil
	}
	var classified *Error
	if errors.As(err, &classified) {
		return err
	}
	return Wrap(category, message, err)
}

// CategoryOf returns the category of err, or Internal when it is unclassified.
func CategoryOf(err error) Category {
	var classified *Error
	if errors.As(err, &classified) {
		return classified.Category
	}
	return Internal
}

// Catalog of named sentinel errors. Search here before declaring a new one.
var (
	ErrNoVaultOpen     = New(Unavailable, "no vault is open")
	ErrVaultNotFound   = New(NotFound, "vault folder not found")
	ErrNoteNotFound    = New(NotFound, "note not found")
	ErrNoteExists      = New(Conflict, "a note with this path already exists")
	ErrIndexNotReady   = New(Unavailable, "index is not ready")
	ErrPathOutsideRoot = New(InvalidInput, "path is outside the vault")
)
