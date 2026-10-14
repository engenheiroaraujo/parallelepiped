// Package dto holds the vault payloads exchanged with the frontend.
package dto

import "parallelepiped/internal/modules/vault/model"

type VaultInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type RecentVaultsResponse struct {
	Vaults []VaultInfo `json:"vaults"`
}

type TreeEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Children []TreeEntry `json:"children"`
}

type TreeResponse struct {
	Root TreeEntry `json:"root"`
}

func VaultFromModel(v model.Vault) VaultInfo {
	return VaultInfo{Path: v.Path, Name: v.Name}
}

func RecentFromModel(vaults []model.Vault) RecentVaultsResponse {
	out := RecentVaultsResponse{Vaults: make([]VaultInfo, 0, len(vaults))}
	for _, v := range vaults {
		out.Vaults = append(out.Vaults, VaultFromModel(v))
	}
	return out
}

func TreeFromModel(node model.TreeNode) TreeResponse {
	return TreeResponse{Root: treeEntry(node)}
}

func treeEntry(node model.TreeNode) TreeEntry {
	entry := TreeEntry{Name: node.Name, Path: node.Path, IsDir: node.IsDir, Children: make([]TreeEntry, 0, len(node.Children))}
	for _, child := range node.Children {
		entry.Children = append(entry.Children, treeEntry(child))
	}
	return entry
}
