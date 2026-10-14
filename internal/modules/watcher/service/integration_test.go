package service_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"parallelepiped/internal/modules/watcher/repository"
	"parallelepiped/internal/modules/watcher/service"
)

type recordingIndex struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingIndex) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *recordingIndex) Upsert(_ context.Context, path, content string, _ time.Time) error {
	r.add("upsert " + path + "=" + content)
	return nil
}
func (r *recordingIndex) Remove(_ context.Context, path string) error {
	r.add("remove " + path)
	return nil
}
func (r *recordingIndex) Rebuild(context.Context, string) error { r.add("rebuild"); return nil }

type channelNotifier chan []string

func (c channelNotifier) VaultChanged(paths []string) { c <- paths }

// TestWatcherOnRealFolder runs the service over fsnotify: an app save (temp
// file renamed over the note) is skipped, an outside edit is indexed.
func TestWatcherOnRealFolder(t *testing.T) {
	root := t.TempDir()
	index := &recordingIndex{}
	notified := make(channelNotifier, 8)
	open := func(root string) (service.Subscription, error) {
		sub, err := repository.FSSource{}.Watch(root)
		if err != nil {
			return nil, err
		}
		return sub, nil
	}
	svc := service.New(index, repository.FileReader{}, open, notified, 50*time.Millisecond)
	if err := svc.Watch(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	note := filepath.Join(root, "n.md")
	tmp := filepath.Join(root, ".write-1.tmp")
	if err := os.WriteFile(tmp, []byte("from app"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, note); err != nil {
		t.Fatal(err)
	}
	if err := svc.Upsert(context.Background(), "n.md", "from app", time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case paths := <-notified:
		t.Fatalf("the app's own save was reported: %v", paths)
	case <-time.After(500 * time.Millisecond):
	}

	if err := os.WriteFile(note, []byte("from outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case paths := <-notified:
		if !slices.Equal(paths, []string{"n.md"}) {
			t.Fatalf("notified %v", paths)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("outside edit not reported")
	}

	index.mu.Lock()
	defer index.mu.Unlock()
	want := []string{"upsert n.md=from app", "upsert n.md=from outside"}
	if !slices.Equal(index.calls, want) {
		t.Fatalf("index calls = %v, want %v", index.calls, want)
	}
}
