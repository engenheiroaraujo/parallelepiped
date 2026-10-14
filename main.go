// Command parallelepiped is the desktop application. This file is the
// composition root: it is the only place that knows every concrete type.
// Wails builds the main package from the project root, so it lives here
// instead of under cmd/.
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	brainhandler "parallelepiped/internal/modules/brain/handler"
	brainrepository "parallelepiped/internal/modules/brain/repository"
	brainservice "parallelepiped/internal/modules/brain/service"
	indexhandler "parallelepiped/internal/modules/index/handler"
	indexrepository "parallelepiped/internal/modules/index/repository"
	indexservice "parallelepiped/internal/modules/index/service"
	noteshandler "parallelepiped/internal/modules/notes/handler"
	notesrepository "parallelepiped/internal/modules/notes/repository"
	notesservice "parallelepiped/internal/modules/notes/service"
	vaulthandler "parallelepiped/internal/modules/vault/handler"
	vaultrepository "parallelepiped/internal/modules/vault/repository"
	vaultservice "parallelepiped/internal/modules/vault/service"
	watcherhandler "parallelepiped/internal/modules/watcher/handler"
	watchermodel "parallelepiped/internal/modules/watcher/model"
	watcherrepository "parallelepiped/internal/modules/watcher/repository"
	watcherservice "parallelepiped/internal/modules/watcher/service"
	"parallelepiped/internal/platform/config"
	"parallelepiped/internal/platform/desktop"
	errs "parallelepiped/internal/platform/errors"
	applog "parallelepiped/internal/platform/log"
	"parallelepiped/migrations"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, logFile, err := applog.New(cfg.LogFile)
	if err != nil {
		return err
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	runtime := &desktop.Runtime{}

	indexDir := filepath.Join(cfg.CacheDir, "index")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return fmt.Errorf("create index dir: %w", err)
	}
	openIndex := func(ctx context.Context, root string) (indexservice.Store, error) {
		return indexrepository.OpenStore(ctx, filepath.Join(indexDir, indexFileName(root)), migrations.FS)
	}
	index := indexservice.New(openIndex, indexrepository.Scanner{})
	defer index.Close()

	openSource := func(root string) (watcherservice.Subscription, error) {
		sub, err := watcherrepository.FSSource{}.Watch(root)
		if err != nil {
			return nil, err
		}
		return sub, nil
	}
	watcher := watcherservice.New(
		index,
		watcherrepository.FileReader{},
		openSource,
		watcherhandler.NewNotifier(runtime),
		watchermodel.DebounceDelay,
	)
	// Runs before index.Close, so no event re-indexes into a closed store.
	defer watcher.Close()

	vaults := vaultservice.New(
		vaultrepository.NewRecentRepository(cfg.ConfigDir),
		vaultrepository.TreeReader{},
		index,
		watcher,
	)
	// Notes write through the watcher so it recognises the app's own saves.
	notes := notesservice.New(notesrepository.FileRepository{}, vaults, watcher)
	// The project import writes through the notes service for the same reason.
	brain := brainservice.New(brainrepository.Analyzer{}, vaults, brainNotes{notes}, brainhandler.NewNotifier(runtime))

	err = wails.Run(&options.App{
		Title:            "parallelepiped",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 1},
		OnStartup:        runtime.Startup,
		OnShutdown:       func(context.Context) { watcher.Close() },
		Bind: []any{
			vaulthandler.NewVault(runtime, vaults),
			noteshandler.NewNotes(runtime, notes),
			indexhandler.NewIndex(runtime, index),
			brainhandler.NewBrain(runtime, brain),
		},
	})
	if err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}

// brainNotes adapts the notes service to the plain-string contract of the
// project import, so the brain module does not depend on notes types.
type brainNotes struct {
	svc *notesservice.Service
}

func (n brainNotes) Read(ctx context.Context, path string) (string, bool, error) {
	note, err := n.svc.Read(ctx, path)
	if errors.Is(err, errs.ErrNoteNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return note.Content, true, nil
}

func (n brainNotes) Save(ctx context.Context, path, content string) error {
	_, err := n.svc.Save(ctx, path, content)
	return err
}

func (n brainNotes) Delete(ctx context.Context, path string) error {
	return n.svc.Delete(ctx, path)
}

// indexFileName derives a stable database file name from the vault folder.
func indexFileName(root string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return hex.EncodeToString(sum[:8]) + ".db"
}
