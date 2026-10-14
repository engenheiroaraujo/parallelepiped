// Package repository stores notes as files inside the vault folder.
package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"parallelepiped/internal/modules/notes/model"
	errs "parallelepiped/internal/platform/errors"
)

// trashDir receives deleted notes so they can be recovered by hand.
const trashDir = ".trash"

// FileRepository reads and writes note files under a vault root.
type FileRepository struct{}

// Read loads the note at the relative path.
func (FileRepository) Read(ctx context.Context, root, notePath string) (model.Note, error) {
	abs, err := resolve(root, notePath)
	if err != nil {
		return model.Note{}, err
	}
	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return model.Note{}, errs.ErrNoteNotFound
	}
	if err != nil {
		return model.Note{}, fmt.Errorf("stat note: %w", err)
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return model.Note{}, fmt.Errorf("read note: %w", err)
	}
	return model.Note{
		Path:       notePath,
		Title:      model.TitleFromPath(notePath),
		Content:    string(content),
		ModifiedAt: info.ModTime(),
	}, nil
}

// Write creates or overwrites the note file, creating parent folders, and
// returns its modification time. The file is written to a temporary sibling
// and renamed so a crash never leaves a half-written note.
func (FileRepository) Write(ctx context.Context, root string, note model.Note) (time.Time, error) {
	abs, err := resolve(root, note.Path)
	if err != nil {
		return time.Time{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return time.Time{}, fmt.Errorf("create note folder: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), ".write-*.tmp")
	if err != nil {
		return time.Time{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(note.Content); err != nil {
		tmp.Close()
		return time.Time{}, fmt.Errorf("write note: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return time.Time{}, fmt.Errorf("close note: %w", err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return time.Time{}, fmt.Errorf("replace note: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat note: %w", err)
	}
	return info.ModTime(), nil
}

// Exists reports whether a note file exists at the relative path.
func (FileRepository) Exists(ctx context.Context, root, notePath string) (bool, error) {
	abs, err := resolve(root, notePath)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat note: %w", err)
	}
	return true, nil
}

// Move renames a note file, creating the destination folder.
func (FileRepository) Move(ctx context.Context, root, from, to string) error {
	src, err := resolve(root, from)
	if err != nil {
		return err
	}
	dst, err := resolve(root, to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create note folder: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errs.ErrNoteNotFound
		}
		return fmt.Errorf("move note: %w", err)
	}
	return nil
}

// Trash moves a note into the vault's .trash folder.
func (FileRepository) Trash(ctx context.Context, root, notePath string) error {
	src, err := resolve(root, notePath)
	if err != nil {
		return err
	}
	stamp := time.Now().Format("20060102-150405")
	name := strings.TrimSuffix(filepath.Base(src), model.Extension) + " " + stamp + model.Extension
	dst := filepath.Join(root, trashDir, name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create trash folder: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errs.ErrNoteNotFound
		}
		return fmt.Errorf("trash note: %w", err)
	}
	return nil
}

// resolve joins root and a relative note path, refusing anything that would
// land outside root.
func resolve(root, notePath string) (string, error) {
	abs := filepath.Join(root, filepath.FromSlash(notePath))
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errs.ErrPathOutsideRoot
	}
	return abs, nil
}
