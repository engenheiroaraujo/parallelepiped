// Package model holds the pure rules of the vault watcher: which paths are
// watched, how bursts of file events collapse into one change per path, and
// how writes made by the app itself are recognised.
package model

import (
	"crypto/sha256"
	"path"
	"slices"
	"strings"
	"time"
)

// DebounceDelay is how long a path must stay quiet before it is re-indexed.
const DebounceDelay = 300 * time.Millisecond

// noteExtension is the extension of every indexed note.
const noteExtension = ".md"

// Event is a raw file system event, already made relative to the vault root.
type Event struct {
	// Path is slash-separated and relative to the vault root; "" is the root.
	Path string
	// Folder is true when the event is about a directory, or when the watcher
	// lost track of changes (buffer overflow) and the whole vault must be
	// re-read.
	Folder bool
}

// Change is a path whose quiet period has ended and is ready to be synced.
type Change struct {
	Path   string
	Folder bool
}

// IsHidden reports whether any segment of the relative path starts with a
// dot (".git", ".trash", ".obsidian", ...). The vault root itself ("") is not
// hidden.
func IsHidden(relPath string) bool {
	if relPath == "" {
		return false
	}
	for _, segment := range strings.Split(relPath, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

// IsNote reports whether a relative file path is an indexed note: a visible
// Markdown file. The app's own temporary files (".write-*.tmp") are hidden
// and so never match.
func IsNote(relPath string) bool {
	return !IsHidden(relPath) && strings.EqualFold(path.Ext(relPath), noteExtension)
}

// Debouncer collapses bursts of events into one Change per path, released
// once the path has been quiet for the configured delay. It holds no timers:
// callers pass the current time, which keeps it deterministic in tests.
type Debouncer struct {
	delay   time.Duration
	pending map[Change]time.Time
}

func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{delay: delay, pending: make(map[Change]time.Time)}
}

// Add schedules the event, pushing back the deadline of a path that is
// already pending.
func (d *Debouncer) Add(event Event, now time.Time) {
	d.pending[Change(event)] = now.Add(d.delay)
}

// Due removes and returns the changes whose deadline has passed, folders
// first and then by path.
func (d *Debouncer) Due(now time.Time) []Change {
	var due []Change
	for change, deadline := range d.pending {
		if !deadline.After(now) {
			due = append(due, change)
			delete(d.pending, change)
		}
	}
	sortChanges(due)
	return due
}

// Next returns the earliest pending deadline, or false when nothing waits.
func (d *Debouncer) Next() (time.Time, bool) {
	var next time.Time
	for _, deadline := range d.pending {
		if next.IsZero() || deadline.Before(next) {
			next = deadline
		}
	}
	return next, !next.IsZero()
}

// Len returns the number of pending changes.
func (d *Debouncer) Len() int { return len(d.pending) }

func sortChanges(changes []Change) {
	slices.SortFunc(changes, func(a, b Change) int {
		if a.Folder != b.Folder {
			if a.Folder {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
}

// Ledger remembers what the index last saw for each note, so an event caused
// by the app's own write — or an external save that changed nothing — is
// recognised and skipped. Paths it has never seen are unknown.
type Ledger struct {
	entries map[string]ledgerEntry
}

type ledgerEntry struct {
	exists bool
	sum    [sha256.Size]byte
}

func NewLedger() *Ledger {
	return &Ledger{entries: make(map[string]ledgerEntry)}
}

// RecordContent notes that the index now holds content for path.
func (l *Ledger) RecordContent(path, content string) {
	l.entries[path] = ledgerEntry{exists: true, sum: sha256.Sum256([]byte(content))}
}

// RecordRemoved notes that path is no longer in the index.
func (l *Ledger) RecordRemoved(path string) {
	l.entries[path] = ledgerEntry{}
}

// HasContent reports whether the index already holds exactly content for path.
func (l *Ledger) HasContent(path, content string) bool {
	entry, ok := l.entries[path]
	return ok && entry.exists && entry.sum == sha256.Sum256([]byte(content))
}

// IsRemoved reports whether path is known to be absent from the index.
func (l *Ledger) IsRemoved(path string) bool {
	entry, ok := l.entries[path]
	return ok && !entry.exists
}

// Reset forgets every path, for when another vault is opened.
func (l *Ledger) Reset() {
	clear(l.entries)
}
