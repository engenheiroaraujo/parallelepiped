package vaultpath

import "testing"

func TestIsExcluded(t *testing.T) {
	cases := map[string]bool{
		"":                              false,
		"Note.md":                       false,
		"docs/adr/0001.md":              false,
		"my.notes/file.md":              false,
		".git/HEAD":                     true,
		"docs/.trash/old.md":            true,
		"vendor/github.com/x/README.md": true,
		"web/node_modules/pkg/a.md":     true,
		"vendors/list.md":               false,
	}
	for path, want := range cases {
		if got := IsExcluded(path); got != want {
			t.Errorf("IsExcluded(%q) = %v, want %v", path, got, want)
		}
	}
}
