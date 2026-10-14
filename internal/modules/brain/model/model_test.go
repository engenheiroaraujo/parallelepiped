package model

import (
	"strings"
	"testing"
)

func TestGroupOf(t *testing.T) {
	cases := map[string]string{
		".":                              "",
		"internal/modules/vault/service": "vault",
		"internal/platform/errors":       "platform",
		"cmd/api":                        "cmd",
		"frontend/src/views":             "frontend",
		"internal":                       "",
		"src/app":                        "",
	}
	for dir, want := range cases {
		if got := GroupOf(dir); got != want {
			t.Errorf("GroupOf(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestUnitTitlesAreUniqueAndShort(t *testing.T) {
	dirs := []string{
		".",
		"internal/modules/vault/service",
		"internal/modules/notes/service",
		"internal/modules/notes/model",
		"internal/platform/errors",
		"b",
		"a/b",
		"x/docs",
	}
	reserved := map[string]bool{"docs": true} // an existing note called "Docs"

	titles := UnitTitles(dirs, reserved)

	want := map[string]string{
		".":                              RootUnitTitle,
		"internal/modules/vault/service": "vault.service",
		"internal/modules/notes/service": "notes.service",
		"internal/modules/notes/model":   "model",
		"internal/platform/errors":       "errors",
		"b":                              "b",
		"a/b":                            "a.b",
		"x/docs":                         "x.docs",
	}
	for dir, title := range want {
		if titles[dir] != title {
			t.Errorf("title of %q = %q, want %q", dir, titles[dir], title)
		}
	}

	seen := map[string]string{}
	for dir, title := range titles {
		key := strings.ToLower(title)
		if other, dup := seen[key]; dup {
			t.Fatalf("%q and %q share the title %q", dir, other, title)
		}
		if reserved[key] {
			t.Fatalf("%q reuses the reserved title %q", dir, title)
		}
		seen[key] = dir
	}
}

func TestUnitTitlesMarkFullPathClash(t *testing.T) {
	titles := UnitTitles([]string{"docs"}, map[string]bool{"docs": true})
	if titles["docs"] != "docs (código)" {
		t.Fatalf("title = %q, want %q", titles["docs"], "docs (código)")
	}
}

func sampleProject() Project {
	return Project{
		Name:       "demo",
		ModulePath: "example.com/demo",
		Units: []Unit{
			{Dir: "internal/modules/vault/service", Language: LanguageGo, Package: "service", Doc: "Vault use cases.",
				Files:     []SourceFile{{Name: "service.go", Lines: 10}},
				Exported:  []Symbol{{Kind: "type", Name: "Service"}, {Kind: "method", Name: "Service.Open"}},
				DependsOn: []string{"internal/platform/errors"}},
			{Dir: "internal/platform/errors", Language: LanguageGo, Package: "errs",
				Files: []SourceFile{{Name: "errors.go", Lines: 1}}, TestFiles: 1},
		},
	}
}

func TestRenderNotes(t *testing.T) {
	notes := RenderNotes(sampleProject(), nil)

	byPath := map[string]string{}
	for _, n := range notes {
		byPath[n.Path] = n.Content
	}
	for _, p := range []string{
		"docs/cerebro/demo.md",
		"docs/cerebro/vault/Módulo vault.md",
		"docs/cerebro/platform/Módulo platform.md",
		"docs/cerebro/vault/service.md",
		"docs/cerebro/platform/errors.md",
	} {
		if _, ok := byPath[p]; !ok {
			t.Fatalf("missing note %s; got %v", p, keys(byPath))
		}
	}

	service := byPath["docs/cerebro/vault/service.md"]
	for _, want := range []string{
		GeneratedMarker, "# service", "> Vault use cases.", "[[Módulo vault]]",
		"## Depende de\n\n- [[errors]]", "método `Service.Open`", UserSectionHeading,
	} {
		if !strings.Contains(service, want) {
			t.Errorf("service note lacks %q:\n%s", want, service)
		}
	}
	if errors := byPath["docs/cerebro/platform/errors.md"]; !strings.Contains(errors, "## Usado por\n\n- [[service]]") {
		t.Errorf("errors note lacks its reverse link:\n%s", errors)
	}

	again := RenderNotes(sampleProject(), nil)
	for i := range notes {
		if notes[i] != again[i] {
			t.Fatalf("rendering is not deterministic for %s", notes[i].Path)
		}
	}
}

func TestMergeKeepsUserSection(t *testing.T) {
	generated := RenderNotes(sampleProject(), nil)[0].Content
	existing := strings.Replace(generated, "Mapa da estrutura", "Texto antigo", 1) + "Minha ideia sobre o projeto.\n"

	merged := Merge(existing, generated)

	if !strings.Contains(merged, "Mapa da estrutura") || strings.Contains(merged, "Texto antigo") {
		t.Errorf("generated part was not refreshed:\n%s", merged)
	}
	if !strings.Contains(merged, "Minha ideia sobre o projeto.") {
		t.Errorf("user text was lost:\n%s", merged)
	}
	if !HasUserText(merged) || HasUserText(generated) {
		t.Errorf("HasUserText: merged=%v generated=%v", HasUserText(merged), HasUserText(generated))
	}
}

func TestIsGenerated(t *testing.T) {
	if !IsGenerated(RenderNotes(sampleProject(), nil)[0].Content) {
		t.Error("generated note not recognised")
	}
	if IsGenerated("# My note\n\ngerado-por: parallelepiped") {
		t.Error("marker outside front matter must not count")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
