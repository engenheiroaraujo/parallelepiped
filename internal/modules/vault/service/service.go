// Package service holds the vault use cases: opening a vault, remembering
// recent ones and reading its file tree.
package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"parallelepiped/internal/modules/vault/model"
	errs "parallelepiped/internal/platform/errors"
)

// RecentRepository persists the recent vault list.
type RecentRepository interface {
	List(ctx context.Context) ([]model.Vault, error)
	Save(ctx context.Context, vaults []model.Vault) error
}

// TreeReader reads vault folders.
type TreeReader interface {
	IsDir(ctx context.Context, root string) (bool, error)
	Tree(ctx context.Context, root string) (model.TreeNode, error)
}

// IndexBuilder rebuilds the index for a vault folder.
type IndexBuilder interface {
	Rebuild(ctx context.Context, root string) error
}

// Watcher follows changes made to the vault folder outside the app.
type Watcher interface {
	Watch(ctx context.Context, root string) error
	Stop()
}

// Service tracks the open vault. It also implements the RootProvider
// contract the other modules use to find the vault folder.
type Service struct {
	recent RecentRepository
	tree   TreeReader
	index  IndexBuilder
	watch  Watcher
	now    func() time.Time

	mu      sync.RWMutex
	current *model.Vault
}

func New(recent RecentRepository, tree TreeReader, index IndexBuilder, watch Watcher) *Service {
	return &Service{recent: recent, tree: tree, index: index, watch: watch, now: time.Now}
}

// Open makes the folder at path the current vault and indexes it.
func (s *Service) Open(ctx context.Context, path string) (model.Vault, error) {
	v, err := model.NewVault(path, s.now())
	if err != nil {
		return model.Vault{}, err
	}
	isDir, err := s.tree.IsDir(ctx, v.Path)
	if err != nil {
		return model.Vault{}, errs.PreserveOr(errs.Internal, "check vault folder", err)
	}
	if !isDir {
		return model.Vault{}, errs.ErrVaultNotFound
	}
	// The previous vault's watcher must not touch the index once it switches.
	s.watch.Stop()
	if err := s.index.Rebuild(ctx, v.Path); err != nil {
		// The index still serves the previous vault, so keep watching it.
		if previous, currentErr := s.Current(); currentErr == nil {
			if watchErr := s.watch.Watch(ctx, previous.Path); watchErr != nil {
				slog.WarnContext(ctx, "watch vault failed", "error", watchErr)
			}
		}
		return model.Vault{}, err
	}

	s.mu.Lock()
	s.current = &v
	s.mu.Unlock()

	// Without the watcher the app still works; outside edits wait for the
	// next open.
	if err := s.watch.Watch(ctx, v.Path); err != nil {
		slog.WarnContext(ctx, "watch vault failed", "error", err)
	}

	// The vault is open either way; failing to remember it is not fatal.
	if err := s.remember(ctx, v); err != nil {
		slog.WarnContext(ctx, "save recent vaults failed", "error", err)
	}
	return v, nil
}

// Current returns the open vault.
func (s *Service) Current() (model.Vault, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return model.Vault{}, errs.ErrNoVaultOpen
	}
	return *s.current, nil
}

// Root returns the folder of the open vault.
func (s *Service) Root() (string, error) {
	v, err := s.Current()
	return v.Path, err
}

// Recent lists the recently opened vaults, most recent first.
func (s *Service) Recent(ctx context.Context) ([]model.Vault, error) {
	vaults, err := s.recent.List(ctx)
	return vaults, errs.PreserveOr(errs.Internal, "list recent vaults", err)
}

// Tree returns the file tree of the open vault.
func (s *Service) Tree(ctx context.Context) (model.TreeNode, error) {
	root, err := s.Root()
	if err != nil {
		return model.TreeNode{}, err
	}
	node, err := s.tree.Tree(ctx, root)
	return node, errs.PreserveOr(errs.Internal, "read vault tree", err)
}

func (s *Service) remember(ctx context.Context, v model.Vault) error {
	list, err := s.recent.List(ctx)
	if err != nil {
		return err
	}
	return s.recent.Save(ctx, model.AddRecent(list, v))
}
