// Package model holds the vault entity, its file tree and the recent-vault
// list rules.
package model

import (
	"path/filepath"
	"strings"
	"time"

	"parallelepiped/internal/platform/validation"
)

// MaxRecentVaults bounds the recent vault list.
const MaxRecentVaults = 10

// Vault is a folder of Markdown notes.
type Vault struct {
	// Path is the absolute, cleaned folder path.
	Path     string
	Name     string
	OpenedAt time.Time
}

// NewVault validates an absolute folder path.
func NewVault(rawPath string, openedAt time.Time) (Vault, error) {
	p := strings.TrimSpace(rawPath)
	if p == "" {
		return Vault{}, validation.Field("path", "Escolha a pasta do cofre.")
	}
	if !filepath.IsAbs(p) {
		return Vault{}, validation.Field("path", "O caminho do cofre deve ser absoluto.")
	}
	p = filepath.Clean(p)
	return Vault{Path: p, Name: filepath.Base(p), OpenedAt: openedAt}, nil
}

// SamePath reports whether two vault paths point to the same folder.
// Comparison ignores case, as on Windows and macOS default file systems.
func (v Vault) SamePath(other Vault) bool {
	return strings.EqualFold(v.Path, other.Path)
}

// AddRecent puts v first in list, removing older entries for the same folder
// and keeping at most MaxRecentVaults.
func AddRecent(list []Vault, v Vault) []Vault {
	out := make([]Vault, 0, min(len(list)+1, MaxRecentVaults))
	out = append(out, v)
	for _, existing := range list {
		if len(out) == MaxRecentVaults {
			break
		}
		if !existing.SamePath(v) {
			out = append(out, existing)
		}
	}
	return out
}

// TreeNode is a folder or note in the vault file tree.
type TreeNode struct {
	Name string
	// Path is relative to the vault root and slash-separated; "" for the root.
	Path     string
	IsDir    bool
	Children []TreeNode
}
