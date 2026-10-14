// Package vaultpath holds the rule for which files and folders inside a vault
// are never shown, indexed or watched, so every module agrees on it.
package vaultpath

import "strings"

// excludedNames are dependency folders whose Markdown files belong to third
// parties, not to the user's notes.
var excludedNames = map[string]bool{
	"node_modules": true,
	"vendor":       true,
}

// IsExcludedName reports whether a single file or folder name is excluded:
// hidden entries (".git", ".trash", ".obsidian", ...) and dependency folders.
func IsExcludedName(name string) bool {
	return strings.HasPrefix(name, ".") || excludedNames[name]
}

// IsExcluded reports whether any segment of a slash-separated path relative
// to the vault root is excluded. The vault root itself ("") is not.
func IsExcluded(relPath string) bool {
	if relPath == "" {
		return false
	}
	for _, segment := range strings.Split(relPath, "/") {
		if IsExcludedName(segment) {
			return true
		}
	}
	return false
}
