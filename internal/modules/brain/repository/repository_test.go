package repository

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"parallelepiped/internal/modules/brain/model"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnalyzeGoAndTypeScript(t *testing.T) {
	root := filepath.Join(t.TempDir(), "demo")
	writeFiles(t, root, map[string]string{
		"go.mod":  "module example.com/demo\n\ngo 1.25\n",
		"main.go": "package main\n\nimport \"example.com/demo/internal/modules/vault/service\"\n\nfunc main() { service.New() }\n",
		"internal/modules/vault/service/service.go": "// Package service holds vault use cases. More text.\npackage service\n\n" +
			"import errs \"example.com/demo/internal/platform/errors\"\n\n" +
			"type Service struct{}\n\nfunc New() *Service { return nil }\n\nfunc (s *Service) Open() error { return errs.ErrX }\n\nfunc helper() {}\n",
		"internal/modules/vault/service/service_test.go": "package service\n",
		"internal/platform/errors/errors.go":             "package errs\n\nimport \"errors\"\n\nvar ErrX = errors.New(\"x\")\n\nconst Max = 1\n",
		"frontend/src/main.ts":                           "import { call } from './api/client';\nimport x from 'external-lib';\n",
		"frontend/src/api/client.ts":                     "export const call = 1;\n",
		"frontend/dist/bundle.js":                        "ignored build output",
		"vendor/lib/lib.go":                              "package lib\n",
		".git/hooks/x.go":                                "package x\n",
		"docs/cerebro/old.md":                            "generated",
	})

	project, err := Analyzer{}.Analyze(context.Background(), root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if project.Name != "demo" || project.ModulePath != "example.com/demo" {
		t.Fatalf("project = %q / %q", project.Name, project.ModulePath)
	}

	units := map[string]model.Unit{}
	for _, u := range project.Units {
		units[u.Dir] = u
	}
	wantDirs := []string{".", "frontend/src", "frontend/src/api", "internal/modules/vault/service", "internal/platform/errors"}
	var gotDirs []string
	for _, u := range project.Units {
		gotDirs = append(gotDirs, u.Dir)
	}
	if !reflect.DeepEqual(gotDirs, wantDirs) {
		t.Fatalf("units = %v, want %v", gotDirs, wantDirs)
	}

	service := units["internal/modules/vault/service"]
	if service.Package != "service" || service.Doc != "Package service holds vault use cases." || service.TestFiles != 1 {
		t.Errorf("service unit = %+v", service)
	}
	wantSymbols := []model.Symbol{{Kind: "type", Name: "Service"}, {Kind: "func", Name: "New"}, {Kind: "method", Name: "Service.Open"}}
	if !reflect.DeepEqual(service.Exported, wantSymbols) {
		t.Errorf("exported = %+v, want %+v", service.Exported, wantSymbols)
	}
	if !reflect.DeepEqual(service.DependsOn, []string{"internal/platform/errors"}) {
		t.Errorf("service deps = %v", service.DependsOn)
	}
	if deps := units["."].DependsOn; !reflect.DeepEqual(deps, []string{"internal/modules/vault/service"}) {
		t.Errorf("root deps = %v", deps)
	}
	if ts := units["frontend/src"]; ts.Language != "TypeScript" || !reflect.DeepEqual(ts.DependsOn, []string{"frontend/src/api"}) {
		t.Errorf("frontend/src = %+v", ts)
	}
}

func TestVaultNotes(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"docs/ADR 001.md":         "",
		"README.md":               "",
		"docs/cerebro/demo.md":    "",
		"docs/cerebro/vault/a.md": "",
		"vendor/pkg/README.md":    "",
		".trash/Old.md":           "",
	})

	titles, generated, err := Analyzer{}.VaultNotes(context.Background(), root)
	if err != nil {
		t.Fatalf("VaultNotes: %v", err)
	}
	if !reflect.DeepEqual(titles, map[string]bool{"adr 001": true, "readme": true}) {
		t.Errorf("titles = %v", titles)
	}
	if !reflect.DeepEqual(generated, []string{"docs/cerebro/demo.md", "docs/cerebro/vault/a.md"}) {
		t.Errorf("generated = %v", generated)
	}
}
