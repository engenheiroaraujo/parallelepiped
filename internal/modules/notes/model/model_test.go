package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	errs "parallelepiped/internal/platform/errors"
)

func TestNormalizePath(t *testing.T) {
	valid := map[string]string{
		"Note":               "Note.md",
		" folder\\Sub/x.md ": "folder/Sub/x.md",
		"a/./b/../c":         "a/c.md",
		"Ideia.MD":           "Ideia.MD",
	}
	for in, want := range valid {
		got, err := NormalizePath(in)
		if err != nil || got != want {
			t.Errorf("NormalizePath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	invalid := []string{"", "   ", "/abs.md", `C:\x.md`, "../escape", "a/../../b", ".hidden/x", "a/.git", "what?.md", strings.Repeat("a", MaxPathLength+1)}
	for _, in := range invalid {
		if _, err := NormalizePath(in); errs.CategoryOf(err) != errs.InvalidInput {
			t.Errorf("NormalizePath(%q) error = %v, want invalid_input", in, err)
		}
	}
}

func TestNewNoteAccumulatesFieldErrors(t *testing.T) {
	_, err := NewNote("../bad", "\xff", time.Time{})

	var classified *errs.Error
	if !errors.As(err, &classified) {
		t.Fatalf("NewNote error = %v, want *errs.Error", err)
	}
	if _, ok := classified.Details["path"]; !ok {
		t.Errorf("missing path detail: %v", classified.Details)
	}
	if _, ok := classified.Details["content"]; !ok {
		t.Errorf("missing content detail: %v", classified.Details)
	}
}

func TestNewNoteTitle(t *testing.T) {
	note, err := NewNote("projects/My Plan", "body", time.Time{})
	if err != nil || note.Title != "My Plan" || note.Path != "projects/My Plan.md" {
		t.Fatalf("NewNote = %+v, %v", note, err)
	}
}
