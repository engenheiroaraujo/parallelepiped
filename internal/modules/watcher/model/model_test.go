package model

import (
	"slices"
	"testing"
	"time"
)

func TestIsNote(t *testing.T) {
	cases := map[string]bool{
		"note.md":              true,
		"Folder/Note.MD":       true,
		"folder/sub/n.md":      true,
		"image.png":            false,
		"notes":                false,
		".write-123.tmp":       false,
		"folder/.write-1.tmp":  false,
		".git/HEAD":            false,
		".trash/old.md":        false,
		"folder/.hidden.md":    false,
		".obsidian/sub/x.md":   false,
		"folder.md/child.txt":  false,
		"folder/.git/notes.md": false,
	}
	for path, want := range cases {
		if got := IsNote(path); got != want {
			t.Errorf("IsNote(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsHidden(t *testing.T) {
	if IsHidden("") {
		t.Error("the vault root must not be hidden")
	}
	if !IsHidden(".trash") || !IsHidden("a/.git/b") {
		t.Error("dot segments must be hidden")
	}
	if IsHidden("a/b.c/d") {
		t.Error("a dot inside a name is not hidden")
	}
}

func TestDebouncerWaitsForQuietPeriod(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDebouncer(300 * time.Millisecond)

	d.Add(Event{Path: "a.md"}, start)
	d.Add(Event{Path: "a.md"}, start.Add(200*time.Millisecond)) // pushes the deadline back
	d.Add(Event{Path: "b.md"}, start.Add(100*time.Millisecond))

	if next, ok := d.Next(); !ok || !next.Equal(start.Add(400*time.Millisecond)) {
		t.Fatalf("Next = %v, %v; want b.md deadline", next, ok)
	}
	if due := d.Due(start.Add(399 * time.Millisecond)); len(due) != 0 {
		t.Fatalf("nothing should be due yet, got %v", due)
	}
	if due := d.Due(start.Add(400 * time.Millisecond)); !slices.Equal(due, []Change{{Path: "b.md"}}) {
		t.Fatalf("due = %v, want b.md", due)
	}
	if due := d.Due(start.Add(500 * time.Millisecond)); !slices.Equal(due, []Change{{Path: "a.md"}}) {
		t.Fatalf("due = %v, want a.md once, after its last event", due)
	}
	if d.Len() != 0 {
		t.Fatalf("pending = %d, want 0", d.Len())
	}
	if _, ok := d.Next(); ok {
		t.Fatal("Next must report nothing pending")
	}
}

func TestDebouncerCollapsesBurstAndOrdersFoldersFirst(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDebouncer(time.Millisecond)
	for range 5 {
		d.Add(Event{Path: "z.md"}, now)
	}
	d.Add(Event{Path: "a.md"}, now)
	d.Add(Event{Path: "dir", Folder: true}, now)

	want := []Change{{Path: "dir", Folder: true}, {Path: "a.md"}, {Path: "z.md"}}
	if due := d.Due(now.Add(time.Second)); !slices.Equal(due, want) {
		t.Fatalf("due = %v, want %v", due, want)
	}
}

func TestLedger(t *testing.T) {
	l := NewLedger()
	if l.HasContent("a.md", "x") || l.IsRemoved("a.md") {
		t.Fatal("an unknown path is neither indexed nor removed")
	}

	l.RecordContent("a.md", "hello")
	if !l.HasContent("a.md", "hello") {
		t.Error("recorded content must match")
	}
	if l.HasContent("a.md", "hello, edited") {
		t.Error("different content must not match")
	}
	if l.IsRemoved("a.md") {
		t.Error("an indexed path is not removed")
	}

	l.RecordRemoved("a.md")
	if !l.IsRemoved("a.md") || l.HasContent("a.md", "hello") {
		t.Error("a removed path must be removed and hold no content")
	}

	l.Reset()
	if l.IsRemoved("a.md") {
		t.Error("Reset must forget every path")
	}
}
