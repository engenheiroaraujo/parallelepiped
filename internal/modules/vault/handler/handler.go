// Package handler exposes vault operations to the frontend through the Wails
// bridge.
package handler

import (
	"parallelepiped/internal/modules/vault/dto"
	"parallelepiped/internal/modules/vault/service"
	"parallelepiped/internal/platform/bridge"
	"parallelepiped/internal/platform/desktop"
	errs "parallelepiped/internal/platform/errors"
)

// Vault is bound to the frontend; every exported method becomes a JS call.
type Vault struct {
	runtime *desktop.Runtime
	svc     *service.Service
}

func NewVault(runtime *desktop.Runtime, svc *service.Service) *Vault {
	return &Vault{runtime: runtime, svc: svc}
}

// Choose shows the folder picker and opens the chosen vault. An empty
// response means the user cancelled.
func (h *Vault) Choose() (dto.VaultInfo, error) {
	path, err := h.runtime.ChooseDirectory("Escolha a pasta do cofre")
	if err != nil {
		return dto.VaultInfo{}, bridge.Error(errs.Wrap(errs.Internal, "folder picker", err))
	}
	if path == "" {
		return dto.VaultInfo{}, nil
	}
	return h.Open(path)
}

// Open opens the vault at an absolute folder path.
func (h *Vault) Open(path string) (dto.VaultInfo, error) {
	v, err := h.svc.Open(h.runtime.Context(), path)
	if err != nil {
		return dto.VaultInfo{}, bridge.Error(err)
	}
	return dto.VaultFromModel(v), nil
}

// Current returns the open vault, or a not-open error.
func (h *Vault) Current() (dto.VaultInfo, error) {
	v, err := h.svc.Current()
	if err != nil {
		return dto.VaultInfo{}, bridge.Error(err)
	}
	return dto.VaultFromModel(v), nil
}

// Recent lists recently opened vaults.
func (h *Vault) Recent() (dto.RecentVaultsResponse, error) {
	vaults, err := h.svc.Recent(h.runtime.Context())
	if err != nil {
		return dto.RecentVaultsResponse{}, bridge.Error(err)
	}
	return dto.RecentFromModel(vaults), nil
}

// Tree returns the file tree of the open vault.
func (h *Vault) Tree() (dto.TreeResponse, error) {
	node, err := h.svc.Tree(h.runtime.Context())
	if err != nil {
		return dto.TreeResponse{}, bridge.Error(err)
	}
	return dto.TreeFromModel(node), nil
}
