package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"parallelepiped/internal/modules/vault/model"
	errs "parallelepiped/internal/platform/errors"
	"parallelepiped/internal/platform/vaultpath"
)

// TreeReader reads the folder structure of a vault.
type TreeReader struct{}

// IsDir reports whether root exists and is a folder.
func (TreeReader) IsDir(ctx context.Context, root string) (bool, error) {
	info, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat vault: %w", err)
	}
	return info.IsDir(), nil
}

// Tree returns folders and .md notes under root, folders first, each level
// sorted by name. Excluded entries (see vaultpath) are skipped.
func (TreeReader) Tree(ctx context.Context, root string) (model.TreeNode, error) {
	node := model.TreeNode{Name: filepath.Base(root), Path: "", IsDir: true}
	if err := readLevel(ctx, root, "", &node); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return model.TreeNode{}, errs.ErrVaultNotFound
		}
		return model.TreeNode{}, fmt.Errorf("read vault tree: %w", err)
	}
	return node, nil
}

func readLevel(ctx context.Context, root, rel string, parent *model.TreeNode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if vaultpath.IsExcludedName(name) {
			continue
		}
		childPath := path.Join(rel, name)
		if entry.IsDir() {
			child := model.TreeNode{Name: name, Path: childPath, IsDir: true}
			if err := readLevel(ctx, root, childPath, &child); err != nil {
				return err
			}
			parent.Children = append(parent.Children, child)
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			parent.Children = append(parent.Children, model.TreeNode{Name: name, Path: childPath})
		}
	}
	sort.SliceStable(parent.Children, func(i, j int) bool {
		a, b := parent.Children[i], parent.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return nil
}
