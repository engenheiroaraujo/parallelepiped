// Package service holds the index use cases: rebuilding it for a vault,
// keeping it current as notes change, and answering graph, backlink and
// search queries.
package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"parallelepiped/internal/modules/index/model"
	errs "parallelepiped/internal/platform/errors"
	"parallelepiped/internal/platform/validation"
)

// Store is the persistence the service needs; repository.Store implements it.
type Store interface {
	Replace(ctx context.Context, docs []model.Document) error
	Upsert(ctx context.Context, doc model.Document) error
	Remove(ctx context.Context, path string) error
	Backlinks(ctx context.Context, path string) ([]model.Backlink, error)
	Search(ctx context.Context, match string, limit int) ([]model.SearchHit, error)
	GraphData(ctx context.Context) ([]model.NoteRef, []model.GraphEdge, []model.UnresolvedLink, error)
	Close() error
}

// StoreOpener opens the index store of the vault at root.
type StoreOpener func(ctx context.Context, root string) (Store, error)

// Scanner reads the source files of a vault.
type Scanner interface {
	Scan(ctx context.Context, root string) ([]model.SourceFile, error)
}

// Service owns the index of the currently open vault.
type Service struct {
	open    StoreOpener
	scanner Scanner

	mu    sync.RWMutex
	store Store
}

func New(open StoreOpener, scanner Scanner) *Service {
	return &Service{open: open, scanner: scanner}
}

// Rebuild switches the index to the vault at root and re-reads every note.
func (s *Service) Rebuild(ctx context.Context, root string) error {
	files, err := s.scanner.Scan(ctx, root)
	if err != nil {
		return errs.PreserveOr(errs.Unavailable, "could not read the vault", err)
	}
	docs := make([]model.Document, 0, len(files))
	for _, file := range files {
		docs = append(docs, model.ParseDocument(file))
	}

	store, err := s.open(ctx, root)
	if err != nil {
		return errs.PreserveOr(errs.Unavailable, "could not open the index", err)
	}
	if err := store.Replace(ctx, docs); err != nil {
		store.Close()
		return errs.PreserveOr(errs.Internal, "rebuild index", err)
	}

	s.mu.Lock()
	previous := s.store
	s.store = store
	s.mu.Unlock()
	if previous != nil {
		previous.Close()
	}
	return nil
}

// Upsert re-indexes one note after it was written.
func (s *Service) Upsert(ctx context.Context, path, content string, modifiedAt time.Time) error {
	store, err := s.current()
	if err != nil {
		return err
	}
	doc := model.ParseDocument(model.SourceFile{Path: path, Content: content, ModifiedAt: modifiedAt})
	return errs.PreserveOr(errs.Internal, "index note", store.Upsert(ctx, doc))
}

// Remove drops one note from the index.
func (s *Service) Remove(ctx context.Context, path string) error {
	store, err := s.current()
	if err != nil {
		return err
	}
	return errs.PreserveOr(errs.Internal, "remove note from index", store.Remove(ctx, path))
}

// Backlinks lists the notes linking to path.
func (s *Service) Backlinks(ctx context.Context, path string) ([]model.Backlink, error) {
	if strings.TrimSpace(path) == "" {
		return nil, validation.Field("path", "Informe a nota.")
	}
	store, err := s.current()
	if err != nil {
		return nil, err
	}
	backlinks, err := store.Backlinks(ctx, path)
	return backlinks, errs.PreserveOr(errs.Internal, "list backlinks", err)
}

// Search runs a full-text search. limit <= 0 uses the default; larger values
// are capped.
func (s *Service) Search(ctx context.Context, text string, limit int) ([]model.SearchHit, error) {
	if len([]rune(text)) > model.MaxQueryLength {
		return nil, validation.Field("query", "A busca é longa demais.")
	}
	if limit <= 0 {
		limit = model.DefaultSearchLimit
	}
	limit = min(limit, model.MaxSearchLimit)

	match := model.FTSQuery(text)
	if match == "" {
		return nil, nil
	}
	store, err := s.current()
	if err != nil {
		return nil, err
	}
	hits, err := store.Search(ctx, match, limit)
	return hits, errs.PreserveOr(errs.Internal, "search notes", err)
}

// Graph returns the full note graph of the vault.
func (s *Service) Graph(ctx context.Context) (model.Graph, error) {
	store, err := s.current()
	if err != nil {
		return model.Graph{}, err
	}
	notes, edges, unresolved, err := store.GraphData(ctx)
	if err != nil {
		return model.Graph{}, errs.PreserveOr(errs.Internal, "load graph", err)
	}
	return model.BuildGraph(notes, edges, unresolved), nil
}

// Close releases the current store.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	err := s.store.Close()
	s.store = nil
	return err
}

func (s *Service) current() (Store, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, errs.ErrIndexNotReady
	}
	return s.store, nil
}
