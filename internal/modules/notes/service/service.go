// Package service holds the note use cases: read, save, create, rename and
// delete, keeping the index in step with every write.
package service

import (
	"context"
	"log/slog"
	"time"

	"parallelepiped/internal/modules/notes/model"
	errs "parallelepiped/internal/platform/errors"
)

// Repository is the note storage; repository.FileRepository implements it.
type Repository interface {
	Read(ctx context.Context, root, path string) (model.Note, error)
	Write(ctx context.Context, root string, note model.Note) (time.Time, error)
	Exists(ctx context.Context, root, path string) (bool, error)
	Move(ctx context.Context, root, from, to string) error
	Trash(ctx context.Context, root, path string) error
}

// RootProvider returns the folder of the open vault.
type RootProvider interface {
	Root() (string, error)
}

// Indexer keeps the search/graph index current. It takes plain values so
// this module does not depend on the index module's types.
type Indexer interface {
	Upsert(ctx context.Context, path, content string, modifiedAt time.Time) error
	Remove(ctx context.Context, path string) error
}

type Service struct {
	repo    Repository
	roots   RootProvider
	indexer Indexer
}

func New(repo Repository, roots RootProvider, indexer Indexer) *Service {
	return &Service{repo: repo, roots: roots, indexer: indexer}
}

// Read loads one note.
func (s *Service) Read(ctx context.Context, rawPath string) (model.Note, error) {
	root, notePath, err := s.locate(rawPath)
	if err != nil {
		return model.Note{}, err
	}
	note, err := s.repo.Read(ctx, root, notePath)
	return note, errs.PreserveOr(errs.Internal, "read note", err)
}

// Save writes content to an existing or new note and re-indexes it.
func (s *Service) Save(ctx context.Context, rawPath, content string) (model.Note, error) {
	root, err := s.roots.Root()
	if err != nil {
		return model.Note{}, err
	}
	note, err := model.NewNote(rawPath, content, time.Time{})
	if err != nil {
		return model.Note{}, err
	}
	return s.write(ctx, root, note)
}

// Create makes a new empty note; it fails if the path is taken.
func (s *Service) Create(ctx context.Context, rawPath string) (model.Note, error) {
	root, err := s.roots.Root()
	if err != nil {
		return model.Note{}, err
	}
	note, err := model.NewNote(rawPath, "", time.Time{})
	if err != nil {
		return model.Note{}, err
	}
	exists, err := s.repo.Exists(ctx, root, note.Path)
	if err != nil {
		return model.Note{}, errs.PreserveOr(errs.Internal, "check note", err)
	}
	if exists {
		return model.Note{}, errs.ErrNoteExists
	}
	return s.write(ctx, root, note)
}

// Rename moves a note to a new path. Links pointing to the old name are not
// rewritten yet.
func (s *Service) Rename(ctx context.Context, rawFrom, rawTo string) (model.Note, error) {
	root, from, err := s.locate(rawFrom)
	if err != nil {
		return model.Note{}, err
	}
	to, err := model.NormalizePath(rawTo)
	if err != nil {
		return model.Note{}, err
	}
	if from == to {
		return s.Read(ctx, from)
	}
	exists, err := s.repo.Exists(ctx, root, to)
	if err != nil {
		return model.Note{}, errs.PreserveOr(errs.Internal, "check note", err)
	}
	if exists {
		return model.Note{}, errs.ErrNoteExists
	}
	if err := s.repo.Move(ctx, root, from, to); err != nil {
		return model.Note{}, errs.PreserveOr(errs.Internal, "rename note", err)
	}

	note, err := s.repo.Read(ctx, root, to)
	if err != nil {
		return model.Note{}, errs.PreserveOr(errs.Internal, "read renamed note", err)
	}
	s.reindex(ctx, func() error { return s.indexer.Remove(ctx, from) })
	s.reindex(ctx, func() error { return s.indexer.Upsert(ctx, note.Path, note.Content, note.ModifiedAt) })
	return note, nil
}

// Delete moves a note to the vault's trash folder.
func (s *Service) Delete(ctx context.Context, rawPath string) error {
	root, notePath, err := s.locate(rawPath)
	if err != nil {
		return err
	}
	if err := s.repo.Trash(ctx, root, notePath); err != nil {
		return errs.PreserveOr(errs.Internal, "delete note", err)
	}
	s.reindex(ctx, func() error { return s.indexer.Remove(ctx, notePath) })
	return nil
}

func (s *Service) write(ctx context.Context, root string, note model.Note) (model.Note, error) {
	modifiedAt, err := s.repo.Write(ctx, root, note)
	if err != nil {
		return model.Note{}, errs.PreserveOr(errs.Internal, "write note", err)
	}
	note.ModifiedAt = modifiedAt
	s.reindex(ctx, func() error { return s.indexer.Upsert(ctx, note.Path, note.Content, note.ModifiedAt) })
	return note, nil
}

// reindex runs an index update. The file on disk is the source of truth, so
// an index failure is logged instead of failing a write that already
// succeeded; the next vault open rebuilds the index anyway.
func (s *Service) reindex(ctx context.Context, update func() error) {
	if err := update(); err != nil {
		slog.WarnContext(ctx, "index update failed", "error", err)
	}
}

func (s *Service) locate(rawPath string) (string, string, error) {
	root, err := s.roots.Root()
	if err != nil {
		return "", "", err
	}
	notePath, err := model.NormalizePath(rawPath)
	if err != nil {
		return "", "", err
	}
	return root, notePath, nil
}
