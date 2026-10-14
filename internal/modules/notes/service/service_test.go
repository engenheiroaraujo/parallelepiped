package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"parallelepiped/internal/modules/notes/model"
	errs "parallelepiped/internal/platform/errors"
)

type fakeRepo struct {
	files map[string]string
}

func (f *fakeRepo) Read(ctx context.Context, root, path string) (model.Note, error) {
	content, ok := f.files[path]
	if !ok {
		return model.Note{}, errs.ErrNoteNotFound
	}
	return model.Note{Path: path, Title: model.TitleFromPath(path), Content: content}, nil
}

func (f *fakeRepo) Write(ctx context.Context, root string, note model.Note) (time.Time, error) {
	f.files[note.Path] = note.Content
	return time.Unix(42, 0), nil
}

func (f *fakeRepo) Exists(ctx context.Context, root, path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

func (f *fakeRepo) Move(ctx context.Context, root, from, to string) error {
	f.files[to] = f.files[from]
	delete(f.files, from)
	return nil
}

func (f *fakeRepo) Trash(ctx context.Context, root, path string) error {
	if _, ok := f.files[path]; !ok {
		return errs.ErrNoteNotFound
	}
	delete(f.files, path)
	return nil
}

type fakeRoots struct{ err error }

func (f fakeRoots) Root() (string, error) { return "/vault", f.err }

type fakeIndexer struct {
	upserted []string
	removed  []string
	err      error
}

func (f *fakeIndexer) Upsert(ctx context.Context, path, content string, modifiedAt time.Time) error {
	f.upserted = append(f.upserted, path)
	return f.err
}

func (f *fakeIndexer) Remove(ctx context.Context, path string) error {
	f.removed = append(f.removed, path)
	return f.err
}

func newTestService() (*Service, *fakeRepo, *fakeIndexer) {
	repo := &fakeRepo{files: map[string]string{}}
	indexer := &fakeIndexer{}
	return New(repo, fakeRoots{}, indexer), repo, indexer
}

func TestSaveWritesAndIndexes(t *testing.T) {
	svc, repo, indexer := newTestService()

	note, err := svc.Save(context.Background(), "Daily", "hello")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if note.Path != "Daily.md" || repo.files["Daily.md"] != "hello" || note.ModifiedAt.Unix() != 42 {
		t.Fatalf("unexpected note %+v / files %v", note, repo.files)
	}
	if len(indexer.upserted) != 1 || indexer.upserted[0] != "Daily.md" {
		t.Fatalf("indexer.upserted = %v", indexer.upserted)
	}
}

func TestSaveSucceedsWhenIndexFails(t *testing.T) {
	svc, _, indexer := newTestService()
	indexer.err = errors.New("disk full")

	if _, err := svc.Save(context.Background(), "x", "y"); err != nil {
		t.Fatalf("Save must not fail on index error: %v", err)
	}
}

func TestCreateRejectsExisting(t *testing.T) {
	svc, repo, _ := newTestService()
	repo.files["Taken.md"] = ""

	if _, err := svc.Create(context.Background(), "Taken"); !errors.Is(err, errs.ErrNoteExists) {
		t.Fatalf("Create error = %v, want ErrNoteExists", err)
	}
}

func TestRenameMovesAndReindexes(t *testing.T) {
	svc, repo, indexer := newTestService()
	repo.files["Old.md"] = "body"

	note, err := svc.Rename(context.Background(), "Old.md", "dir/New")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if note.Path != "dir/New.md" || repo.files["dir/New.md"] != "body" {
		t.Fatalf("Rename result %+v / files %v", note, repo.files)
	}
	if len(indexer.removed) != 1 || indexer.removed[0] != "Old.md" {
		t.Fatalf("indexer.removed = %v", indexer.removed)
	}
}

func TestDeletePreservesNotFound(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.Delete(context.Background(), "ghost")
	if errs.CategoryOf(err) != errs.NotFound {
		t.Fatalf("Delete error category = %v, want not_found", errs.CategoryOf(err))
	}
}

func TestOperationsRequireOpenVault(t *testing.T) {
	svc := New(&fakeRepo{files: map[string]string{}}, fakeRoots{err: errs.ErrNoVaultOpen}, &fakeIndexer{})

	if _, err := svc.Read(context.Background(), "x"); !errors.Is(err, errs.ErrNoVaultOpen) {
		t.Fatalf("Read error = %v, want ErrNoVaultOpen", err)
	}
}
