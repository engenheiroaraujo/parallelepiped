package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"parallelepiped/internal/modules/watcher/model"
	errs "parallelepiped/internal/platform/errors"
)

// maxNoteBytes matches the index scanner: larger files are not indexed.
const maxNoteBytes = 10 << 20

// FileReader reads changed notes from the vault folder.
type FileReader struct{}

// Read loads the note at the relative path. found is false when the file no
// longer exists or is too large to be indexed.
func (FileReader) Read(ctx context.Context, root, relPath string) (model.File, bool, error) {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return model.File{}, false, errs.ErrPathOutsideRoot
	}

	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return model.File{}, false, nil
	}
	if err != nil {
		return model.File{}, false, fmt.Errorf("stat note: %w", err)
	}
	if info.IsDir() || info.Size() > maxNoteBytes {
		return model.File{}, false, nil
	}
	content, err := os.ReadFile(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return model.File{}, false, nil
	}
	if err != nil {
		return model.File{}, false, fmt.Errorf("read note: %w", err)
	}
	return model.File{Path: relPath, Content: string(content), ModifiedAt: info.ModTime()}, true, nil
}
