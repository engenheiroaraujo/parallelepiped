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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	indexhandler "parallelepiped/internal/modules/index/handler"
	indexrepository "parallelepiped/internal/modules/index/repository"
	indexservice "parallelepiped/internal/modules/index/service"
	noteshandler "parallelepiped/internal/modules/notes/handler"
	notesrepository "parallelepiped/internal/modules/notes/repository"
	notesservice "parallelepiped/internal/modules/notes/service"
	vaulthandler "parallelepiped/internal/modules/vault/handler"
	vaultrepository "parallelepiped/internal/modules/vault/repository"
	vaultservice "parallelepiped/internal/modules/vault/service"
	"parallelepiped/internal/platform/config"
	"parallelepiped/internal/platform/desktop"
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

	vaults := vaultservice.New(
		vaultrepository.NewRecentRepository(cfg.ConfigDir),
		vaultrepository.TreeReader{},
		index,
	)
	notes := notesservice.New(notesrepository.FileRepository{}, vaults, index)

	err = wails.Run(&options.App{
		Title:            "parallelepiped",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 1},
		OnStartup:        runtime.Startup,
		Bind: []any{
			vaulthandler.NewVault(runtime, vaults),
			noteshandler.NewNotes(runtime, notes),
			indexhandler.NewIndex(runtime, index),
		},
	})
	if err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}

// indexFileName derives a stable database file name from the vault folder.
func indexFileName(root string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return hex.EncodeToString(sum[:8]) + ".db"
}
