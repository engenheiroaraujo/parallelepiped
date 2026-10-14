// Package repository persists the recent vault list and reads vault folders.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"parallelepiped/internal/modules/vault/model"
)

// RecentRepository stores the recent vault list as a JSON file.
type RecentRepository struct {
	file string
}

func NewRecentRepository(configDir string) *RecentRepository {
	return &RecentRepository{file: filepath.Join(configDir, "recent-vaults.json")}
}

// recentFile is the on-disk format of the recent vault list.
type recentFile struct {
	Vaults []recentRow `json:"vaults"`
}

type recentRow struct {
	Path     string    `json:"path"`
	OpenedAt time.Time `json:"opened_at"`
}

// List returns the stored vaults, or none when the file does not exist yet.
func (r *RecentRepository) List(ctx context.Context) ([]model.Vault, error) {
	data, err := os.ReadFile(r.file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read recent vaults: %w", err)
	}
	var stored recentFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode recent vaults: %w", err)
	}

	vaults := make([]model.Vault, 0, len(stored.Vaults))
	for _, row := range stored.Vaults {
		v, err := model.NewVault(row.Path, row.OpenedAt)
		if err != nil {
			continue
		}
		vaults = append(vaults, v)
	}
	return vaults, nil
}

// Save replaces the stored list.
func (r *RecentRepository) Save(ctx context.Context, vaults []model.Vault) error {
	stored := recentFile{Vaults: make([]recentRow, 0, len(vaults))}
	for _, v := range vaults {
		stored.Vaults = append(stored.Vaults, recentRow{Path: v.Path, OpenedAt: v.OpenedAt})
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode recent vaults: %w", err)
	}
	if err := os.WriteFile(r.file, data, 0o644); err != nil {
		return fmt.Errorf("write recent vaults: %w", err)
	}
	return nil
}
