package model

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	errs "parallelepiped/internal/platform/errors"
)

func TestNewVault(t *testing.T) {
	abs, _ := filepath.Abs("vault-dir")
	v, err := NewVault("  "+abs+"  ", time.Time{})
	if err != nil || v.Path != abs || v.Name != "vault-dir" {
		t.Fatalf("NewVault = %+v, %v", v, err)
	}

	for _, in := range []string{"", "relative/dir"} {
		if _, err := NewVault(in, time.Time{}); errs.CategoryOf(err) != errs.InvalidInput {
			t.Errorf("NewVault(%q) error = %v, want invalid_input", in, err)
		}
	}
}

func TestAddRecent(t *testing.T) {
	var list []Vault
	for i := range MaxRecentVaults + 3 {
		list = AddRecent(list, Vault{Path: fmt.Sprintf("/v%d", i)})
	}
	if len(list) != MaxRecentVaults || list[0].Path != fmt.Sprintf("/v%d", MaxRecentVaults+2) {
		t.Fatalf("list = %+v", list)
	}

	list = AddRecent(list, Vault{Path: "/V5"})
	count := 0
	for _, v := range list {
		if v.SamePath(Vault{Path: "/v5"}) {
			count++
		}
	}
	if list[0].Path != "/V5" || count != 1 || len(list) != MaxRecentVaults {
		t.Fatalf("re-adding moved/duplicated wrongly: %+v", list)
	}
}
