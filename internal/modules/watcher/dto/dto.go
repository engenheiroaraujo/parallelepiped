// Package dto holds the payloads the watcher pushes to the frontend.
package dto

// EventVaultChanged is the Wails event emitted when notes change on disk
// outside the app.
const EventVaultChanged = "vault:changed"

// VaultChanged lists the vault-relative, slash-separated paths that changed.
// A folder path means everything below it may have changed; "" is the whole
// vault.
type VaultChanged struct {
	Paths []string `json:"paths"`
}
