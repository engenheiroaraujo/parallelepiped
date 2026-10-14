// Package config resolves the per-user directories the application writes to.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirName = "parallelepiped"

// Config lists the directories owned by the application.
type Config struct {
	// ConfigDir stores small user settings such as the recent vault list.
	ConfigDir string
	// CacheDir stores derived data that can be rebuilt, such as search indexes.
	CacheDir string
	// LogFile is the path of the application log.
	LogFile string
}

// Load resolves and creates the application directories.
func Load() (Config, error) {
	return load(os.UserConfigDir, os.UserCacheDir)
}

func load(userConfigDir, userCacheDir func() (string, error)) (Config, error) {
	configRoot, err := userConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user config dir: %w", err)
	}
	cacheRoot, err := userCacheDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user cache dir: %w", err)
	}

	cfg := Config{
		ConfigDir: filepath.Join(configRoot, appDirName),
		CacheDir:  filepath.Join(cacheRoot, appDirName),
	}
	cfg.LogFile = filepath.Join(cfg.ConfigDir, "parallelepiped.log")

	for _, dir := range []string{cfg.ConfigDir, cfg.CacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Config{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return cfg, nil
}
