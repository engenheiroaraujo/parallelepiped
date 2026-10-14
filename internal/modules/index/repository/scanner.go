package repository

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"parallelepiped/internal/modules/index/model"
	"parallelepiped/internal/platform/vaultpath"
)

// maxIndexedFileBytes skips huge files that are unlikely to be notes.
const maxIndexedFileBytes = 10 << 20

// Scanner reads every Markdown file of a vault.
type Scanner struct{}

// Scan walks root and returns its .md files with slash-separated relative
// paths. Excluded entries (hidden ones and dependency folders, see
// vaultpath) are skipped.
func (Scanner) Scan(ctx context.Context, root string) ([]model.SourceFile, error) {
	var files []model.SourceFile
	err := filepath.WalkDir(root, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if abs != root && vaultpath.IsExcludedName(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxIndexedFileBytes {
			return nil
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		files = append(files, model.SourceFile{
			Path:       filepath.ToSlash(rel),
			Content:    string(content),
			ModifiedAt: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan vault: %w", err)
	}
	return files, nil
}
