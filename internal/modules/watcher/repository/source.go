// Package repository adapts the file system to the watcher: fsnotify events
// for the vault folder and reads of changed note files.
package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"parallelepiped/internal/modules/watcher/model"
)

// eventBuffer absorbs short bursts (a git checkout, a sync client) while the
// service is busy re-indexing.
const eventBuffer = 256

// FSSource watches vault folders with fsnotify. fsnotify is not recursive, so
// every visible folder gets its own watch and new folders are added as they
// appear.
type FSSource struct{}

// Watch starts watching root and every visible folder below it.
func (FSSource) Watch(root string) (*Subscription, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	sub := &Subscription{
		root:    root,
		watcher: watcher,
		events:  make(chan model.Event, eventBuffer),
		done:    make(chan struct{}),
		folders: make(map[string]struct{}),
	}
	if err := sub.addTree(root, false); err != nil {
		watcher.Close()
		return nil, err
	}
	sub.wg.Add(1)
	go sub.run()
	return sub, nil
}

// Subscription delivers the events of one watched vault until closed.
type Subscription struct {
	root    string
	watcher *fsnotify.Watcher
	events  chan model.Event
	done    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup

	// folders is only touched by addTree before run starts and by run itself.
	folders map[string]struct{}
}

// Events is closed after Close.
func (s *Subscription) Events() <-chan model.Event { return s.events }

// Close stops watching and waits for the event loop to end.
func (s *Subscription) Close() error {
	var err error
	s.once.Do(func() {
		close(s.done)
		err = s.watcher.Close()
		s.wg.Wait()
	})
	return err
}

func (s *Subscription) run() {
	defer s.wg.Done()
	defer close(s.events)
	for {
		select {
		case <-s.done:
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handle(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				// Events were lost: ask for the whole vault to be re-read.
				s.emit(model.Event{Path: "", Folder: true})
				continue
			}
			slog.Warn("vault watcher error", "error", err)
		}
	}
}

func (s *Subscription) handle(event fsnotify.Event) {
	rel, ok := s.relative(event.Name)
	if !ok || rel == "" || model.IsHidden(rel) {
		return
	}
	switch {
	case event.Has(fsnotify.Create):
		info, err := os.Stat(event.Name)
		if err != nil {
			// Gone already (a temporary file, a quick rename): a later event
			// or the flush-time read settles it.
			if model.IsNote(rel) {
				s.emit(model.Event{Path: rel})
			}
			return
		}
		if info.IsDir() {
			// Notes created inside before the watch was attached would be
			// missed, so the new folder's notes are reported as created.
			if err := s.addTree(event.Name, true); err != nil {
				slog.Warn("watch new folder failed", "folder", rel, "error", err)
			}
			return
		}
		if model.IsNote(rel) {
			s.emit(model.Event{Path: rel})
		}
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		if s.forgetTree(event.Name) {
			s.emit(model.Event{Path: rel, Folder: true})
			return
		}
		if model.IsNote(rel) {
			s.emit(model.Event{Path: rel})
		}
	case event.Has(fsnotify.Write):
		if model.IsNote(rel) {
			s.emit(model.Event{Path: rel})
		}
	}
}

// addTree watches dir and its visible subfolders. With report set, the notes
// found are emitted as changed.
func (s *Subscription) addTree(dir string, report bool) error {
	return filepath.WalkDir(dir, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A folder removed mid-walk is not worth failing the whole walk.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, ok := s.relative(abs)
		if !ok || model.IsHidden(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			if report && model.IsNote(rel) {
				s.emit(model.Event{Path: rel})
			}
			return nil
		}
		if err := s.watcher.Add(abs); err != nil {
			return fmt.Errorf("watch folder %s: %w", rel, err)
		}
		s.folders[abs] = struct{}{}
		return nil
	})
}

// forgetTree drops the watches of a removed or renamed folder and reports
// whether abs was a watched folder.
func (s *Subscription) forgetTree(abs string) bool {
	if _, ok := s.folders[abs]; !ok {
		return false
	}
	prefix := abs + string(filepath.Separator)
	for folder := range s.folders {
		if folder == abs || strings.HasPrefix(folder, prefix) {
			// The OS usually dropped the watch already; the error is moot.
			_ = s.watcher.Remove(folder)
			delete(s.folders, folder)
		}
	}
	return true
}

func (s *Subscription) emit(event model.Event) {
	select {
	case s.events <- event:
	case <-s.done:
	}
}

// relative turns an absolute path into a slash-separated path relative to
// the vault root; "" is the root itself.
func (s *Subscription) relative(abs string) (string, bool) {
	rel, err := filepath.Rel(s.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	return filepath.ToSlash(rel), true
}
