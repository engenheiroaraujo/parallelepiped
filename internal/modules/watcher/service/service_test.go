package service

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"parallelepiped/internal/modules/watcher/model"
)

const testDelay = 20 * time.Millisecond

type fakeSubscription struct {
	events chan model.Event
	once   sync.Once
}

func (f *fakeSubscription) Events() <-chan model.Event { return f.events }
func (f *fakeSubscription) Close() error {
	f.once.Do(func() { close(f.events) })
	return nil
}

// fakeFiles is an in-memory vault: a missing path is a deleted note.
type fakeFiles struct {
	mu    sync.Mutex
	notes map[string]string
}

func (f *fakeFiles) set(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes[path] = content
}

func (f *fakeFiles) remove(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.notes, path)
}

func (f *fakeFiles) Read(_ context.Context, _, path string) (model.File, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.notes[path]
	return model.File{Path: path, Content: content}, ok, nil
}

type fakeIndex struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeIndex) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeIndex) Upsert(_ context.Context, path, content string, _ time.Time) error {
	f.record("upsert " + path + "=" + content)
	return nil
}

func (f *fakeIndex) Remove(_ context.Context, path string) error {
	f.record("remove " + path)
	return nil
}

func (f *fakeIndex) Rebuild(_ context.Context, root string) error {
	f.record("rebuild " + root)
	return nil
}

func (f *fakeIndex) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

type fakeNotifier struct {
	batches chan []string
}

func (f *fakeNotifier) VaultChanged(paths []string) { f.batches <- paths }

type harness struct {
	svc      *Service
	sub      *fakeSubscription
	files    *fakeFiles
	index    *fakeIndex
	notifier *fakeNotifier
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		sub:      &fakeSubscription{events: make(chan model.Event, 16)},
		files:    &fakeFiles{notes: map[string]string{}},
		index:    &fakeIndex{},
		notifier: &fakeNotifier{batches: make(chan []string, 16)},
	}
	open := func(string) (Subscription, error) { return h.sub, nil }
	h.svc = New(h.index, h.files, open, h.notifier, testDelay)
	if err := h.svc.Watch(context.Background(), "root"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.svc.Stop)
	return h
}

func (h *harness) send(events ...model.Event) {
	for _, event := range events {
		h.sub.events <- event
	}
}

func (h *harness) nextBatch(t *testing.T) []string {
	t.Helper()
	select {
	case paths := <-h.notifier.batches:
		return paths
	case <-time.After(2 * time.Second):
		t.Fatal("no vault:changed notification")
		return nil
	}
}

// quiet asserts nothing is notified for several debounce periods.
func (h *harness) quiet(t *testing.T) {
	t.Helper()
	select {
	case paths := <-h.notifier.batches:
		t.Fatalf("unexpected notification for %v", paths)
	case <-time.After(5 * testDelay):
	}
}

func TestBurstOfEventsIndexesOnce(t *testing.T) {
	h := newHarness(t)
	h.files.set("a.md", "final")

	h.send(model.Event{Path: "a.md"}, model.Event{Path: "a.md"}, model.Event{Path: "a.md"})

	if got := h.nextBatch(t); !slices.Equal(got, []string{"a.md"}) {
		t.Fatalf("notified %v, want [a.md]", got)
	}
	if got := h.index.snapshot(); !slices.Equal(got, []string{"upsert a.md=final"}) {
		t.Fatalf("index calls = %v", got)
	}
}

func TestDeletedNoteIsRemoved(t *testing.T) {
	h := newHarness(t)

	h.send(model.Event{Path: "gone.md"})

	if got := h.nextBatch(t); !slices.Equal(got, []string{"gone.md"}) {
		t.Fatalf("notified %v", got)
	}
	if got := h.index.snapshot(); !slices.Equal(got, []string{"remove gone.md"}) {
		t.Fatalf("index calls = %v", got)
	}
}

func TestOwnSaveIsSkipped(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// The notes module saves through the service, then the file event of
	// that save arrives.
	h.files.set("a.md", "from app")
	if err := h.svc.Upsert(ctx, "a.md", "from app", time.Now()); err != nil {
		t.Fatal(err)
	}
	h.send(model.Event{Path: "a.md"})
	h.quiet(t)

	// The app deletes a note: the removal event is skipped too.
	h.files.remove("b.md")
	if err := h.svc.Remove(ctx, "b.md"); err != nil {
		t.Fatal(err)
	}
	h.send(model.Event{Path: "b.md"})
	h.quiet(t)

	want := []string{"upsert a.md=from app", "remove b.md"}
	if got := h.index.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("index calls = %v, want only the app's own %v", got, want)
	}

	// A later external edit of the same note is indexed.
	h.files.set("a.md", "edited outside")
	h.send(model.Event{Path: "a.md"})
	if got := h.nextBatch(t); !slices.Equal(got, []string{"a.md"}) {
		t.Fatalf("notified %v", got)
	}
	if got := h.index.snapshot(); got[len(got)-1] != "upsert a.md=edited outside" {
		t.Fatalf("index calls = %v", got)
	}
}

func TestFolderChangeRebuildsIndex(t *testing.T) {
	h := newHarness(t)

	h.files.set("renamed/a.md", "x")
	h.send(
		model.Event{Path: "projects", Folder: true},
		model.Event{Path: "projects", Folder: true},
		model.Event{Path: "renamed/a.md"},
	)

	// The rebuild already re-read the note, so it is reported, not upserted.
	if got := h.nextBatch(t); !slices.Equal(got, []string{"projects", "renamed/a.md"}) {
		t.Fatalf("notified %v", got)
	}
	if got := h.index.snapshot(); !slices.Equal(got, []string{"rebuild root"}) {
		t.Fatalf("index calls = %v", got)
	}
}

func TestWatchReplacesPreviousWatch(t *testing.T) {
	h := newHarness(t)
	first := h.sub

	h.sub = &fakeSubscription{events: make(chan model.Event, 16)}
	if err := h.svc.Watch(context.Background(), "other"); err != nil {
		t.Fatal(err)
	}
	if _, open := <-first.events; open {
		t.Fatal("the previous subscription must be closed")
	}

	h.files.set("n.md", "x")
	h.send(model.Event{Path: "n.md"})
	if got := h.nextBatch(t); !slices.Equal(got, []string{"n.md"}) {
		t.Fatalf("notified %v", got)
	}
}

func TestStopEndsLoopWithoutFurtherUpdates(t *testing.T) {
	h := newHarness(t)
	h.files.set("a.md", "x")
	h.send(model.Event{Path: "a.md"})
	h.svc.Stop()

	h.quiet(t)
	if got := h.index.snapshot(); len(got) != 0 {
		t.Fatalf("index calls after Stop = %v", got)
	}
}
