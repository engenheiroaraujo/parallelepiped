// Package service keeps the index in step with changes made to the vault
// outside the app: it consumes file events, waits for each path to settle,
// re-indexes what really changed and tells the frontend.
package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"parallelepiped/internal/modules/watcher/model"
)

// Subscription is a running watch of one vault folder.
type Subscription interface {
	// Events is closed once the subscription is closed.
	Events() <-chan model.Event
	Close() error
}

// SourceOpener starts watching the vault at root; the repository's FSSource
// implements it.
type SourceOpener func(root string) (Subscription, error)

// FileReader reads one note of the vault. found is false when it is gone.
type FileReader interface {
	Read(ctx context.Context, root, relPath string) (file model.File, found bool, err error)
}

// Indexer is the index module, seen through plain values.
type Indexer interface {
	Upsert(ctx context.Context, path, content string, modifiedAt time.Time) error
	Remove(ctx context.Context, path string) error
	Rebuild(ctx context.Context, root string) error
}

// Notifier tells the frontend which vault paths changed on disk.
type Notifier interface {
	VaultChanged(paths []string)
}

// Service watches the open vault. It also wraps the index for the notes
// module (Upsert/Remove), recording what the app itself wrote so the file
// events of its own saves are recognised and skipped.
type Service struct {
	index    Indexer
	files    FileReader
	open     SourceOpener
	notifier Notifier
	delay    time.Duration

	// mu guards ledger, which the event loop and the notes module share.
	mu     sync.Mutex
	ledger *model.Ledger

	// runMu serialises Watch and Stop.
	runMu  sync.Mutex
	sub    Subscription
	cancel context.CancelFunc
	done   chan struct{}
}

func New(index Indexer, files FileReader, open SourceOpener, notifier Notifier, delay time.Duration) *Service {
	return &Service{
		index:    index,
		files:    files,
		open:     open,
		notifier: notifier,
		delay:    delay,
		ledger:   model.NewLedger(),
	}
}

// Watch stops any previous watch and starts watching root.
func (s *Service) Watch(ctx context.Context, root string) error {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.stopLocked()

	s.mu.Lock()
	s.ledger.Reset()
	s.mu.Unlock()

	sub, err := s.open(root)
	if err != nil {
		return err
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.sub, s.cancel, s.done = sub, cancel, done
	go func() {
		defer close(done)
		s.run(loopCtx, root, sub.Events())
	}()
	slog.InfoContext(ctx, "watching vault", "root", root)
	return nil
}

// Stop ends the current watch and waits until no index update is running.
func (s *Service) Stop() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.stopLocked()
}

// Close stops watching; wire it to the application shutdown.
func (s *Service) Close() error {
	s.Stop()
	return nil
}

func (s *Service) stopLocked() {
	if s.sub == nil {
		return
	}
	s.cancel()
	if err := s.sub.Close(); err != nil {
		slog.Warn("close vault watcher failed", "error", err)
	}
	<-s.done
	s.sub, s.cancel, s.done = nil, nil, nil
}

// Upsert indexes a note written by the app and remembers its content.
func (s *Service) Upsert(ctx context.Context, path, content string, modifiedAt time.Time) error {
	if err := s.index.Upsert(ctx, path, content, modifiedAt); err != nil {
		return err
	}
	s.mu.Lock()
	s.ledger.RecordContent(path, content)
	s.mu.Unlock()
	return nil
}

// Remove drops a note removed by the app and remembers it is gone.
func (s *Service) Remove(ctx context.Context, path string) error {
	if err := s.index.Remove(ctx, path); err != nil {
		return err
	}
	s.mu.Lock()
	s.ledger.RecordRemoved(path)
	s.mu.Unlock()
	return nil
}

// run collects events until each path has been quiet for the debounce
// delay, then syncs the settled paths in one batch.
func (s *Service) run(ctx context.Context, root string, events <-chan model.Event) {
	pending := model.NewDebouncer(s.delay)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			pending.Add(event, time.Now())
		case <-timer.C:
			s.sync(ctx, root, pending.Due(time.Now()))
		}
		if next, ok := pending.Next(); ok {
			timer.Reset(max(0, time.Until(next)))
		}
	}
}

// sync re-indexes the settled changes and notifies the frontend about the
// paths that really changed.
func (s *Service) sync(ctx context.Context, root string, changes []model.Change) {
	var affected []string
	rebuilt := false
	// Folders come first, so once the vault was re-read the notes of the
	// same batch need no separate update.
	for _, change := range changes {
		if ctx.Err() != nil {
			return
		}
		var changed bool
		switch {
		case change.Folder && rebuilt:
			changed = true
		case change.Folder:
			rebuilt = s.syncFolder(ctx, root, change.Path)
			changed = rebuilt
		case rebuilt:
			changed = true
		default:
			changed = s.syncNote(ctx, root, change.Path)
		}
		if changed {
			affected = append(affected, change.Path)
		}
	}
	if len(affected) > 0 && ctx.Err() == nil {
		s.notifier.VaultChanged(affected)
	}
}

// syncFolder handles a removed or renamed folder (or lost events). The index
// cannot list a folder's notes, so the vault is re-read.
func (s *Service) syncFolder(ctx context.Context, root, folder string) bool {
	if err := s.index.Rebuild(ctx, root); err != nil {
		slog.WarnContext(ctx, "rebuild index after folder change failed", "folder", folder, "error", err)
		return false
	}
	s.mu.Lock()
	s.ledger.Reset()
	s.mu.Unlock()
	return true
}

// syncNote brings one note of the index in line with the disk, skipping
// what the index already holds (the app's own saves, no-op external saves).
func (s *Service) syncNote(ctx context.Context, root, path string) bool {
	file, found, err := s.files.Read(ctx, root, path)
	if err != nil {
		// Typically a file still locked by the editor saving it; its next
		// write event queues it again.
		slog.WarnContext(ctx, "read changed note failed", "path", path, "error", err)
		return false
	}

	s.mu.Lock()
	upToDate := (found && s.ledger.HasContent(path, file.Content)) || (!found && s.ledger.IsRemoved(path))
	s.mu.Unlock()
	if upToDate {
		return false
	}

	if found {
		err = s.Upsert(ctx, path, file.Content, file.ModifiedAt)
	} else {
		err = s.Remove(ctx, path)
	}
	if err != nil {
		slog.WarnContext(ctx, "index external change failed", "path", path, "error", err)
		return false
	}
	return true
}
