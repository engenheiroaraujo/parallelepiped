package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"parallelepiped/internal/modules/watcher/model"
)

// waitFor reads events until want arrives or the deadline passes.
func waitFor(t *testing.T, sub *Subscription, want model.Event) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case got, ok := <-sub.Events():
			if !ok {
				t.Fatalf("events closed before %+v", want)
			}
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("no event %+v", want)
		}
	}
}

func TestFSSourceReportsNoteChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".trash"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub, err := FSSource{}.Watch(root)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Ignored: hidden folders, the app's temp files and non-notes.
	write(".trash/old.md", "x")
	write(".write-1.tmp", "x")
	write("image.png", "x")

	write("a.md", "one")
	waitFor(t, sub, model.Event{Path: "a.md"})

	// A note created in a brand-new folder is reported with its folder path.
	if err := os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("sub/deep/b.md", "two")
	waitFor(t, sub, model.Event{Path: "sub/deep/b.md"})

	// The notes repository saves through a temp file renamed over the note:
	// only the note itself is reported.
	write(".write-2.tmp", "three")
	if err := os.Rename(filepath.Join(root, ".write-2.tmp"), filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, sub, model.Event{Path: "a.md"})

	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, sub, model.Event{Path: "a.md"})

	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	for event := range sub.Events() {
		if model.IsHidden(event.Path) || (!event.Folder && !model.IsNote(event.Path)) {
			t.Errorf("ignored path reported: %+v", event)
		}
	}
}

func TestFileReader(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "n.md"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	file, found, err := FileReader{}.Read(ctx, root, "n.md")
	if err != nil || !found || file.Content != "body" {
		t.Fatalf("Read = %+v, %v, %v", file, found, err)
	}
	if _, found, err := (FileReader{}).Read(ctx, root, "missing.md"); err != nil || found {
		t.Fatalf("missing note: found=%v err=%v", found, err)
	}
	if _, _, err := (FileReader{}).Read(ctx, root, "../escape.md"); err == nil {
		t.Fatal("a path outside the vault must fail")
	}
}
