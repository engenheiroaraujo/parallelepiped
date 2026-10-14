package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"parallelepiped/internal/modules/brain/model"
	errs "parallelepiped/internal/platform/errors"
)

type fakeAnalyzer struct {
	project model.Project
	titles  map[string]bool
	notes   *fakeNotes
}

func (f *fakeAnalyzer) Analyze(ctx context.Context, root string) (model.Project, error) {
	return f.project, nil
}

func (f *fakeAnalyzer) VaultNotes(ctx context.Context, root string) (map[string]bool, []string, error) {
	var generated []string
	for path := range f.notes.files {
		if strings.HasPrefix(path, model.OutputDir+"/") {
			generated = append(generated, path)
		}
	}
	return f.titles, generated, nil
}

type fakeNotes struct {
	files   map[string]string
	saves   int
	deleted []string
}

func (f *fakeNotes) Read(ctx context.Context, path string) (string, bool, error) {
	content, ok := f.files[path]
	return content, ok, nil
}

func (f *fakeNotes) Save(ctx context.Context, path, content string) error {
	f.files[path] = content
	f.saves++
	return nil
}

func (f *fakeNotes) Delete(ctx context.Context, path string) error {
	delete(f.files, path)
	f.deleted = append(f.deleted, path)
	return nil
}

type fakeRoots struct{ err error }

func (f fakeRoots) Root() (string, error) { return "/vault", f.err }

type fakeProgress struct{ last, total int }

func (f *fakeProgress) ImportProgress(done, total int) { f.last, f.total = done, total }

func project(units ...string) model.Project {
	p := model.Project{Name: "demo"}
	for _, dir := range units {
		p.Units = append(p.Units, model.Unit{Dir: dir, Language: model.LanguageGo, Files: []model.SourceFile{{Name: "a.go", Lines: 1}}})
	}
	return p
}

func newTestService(p model.Project) (*Service, *fakeAnalyzer, *fakeNotes, *fakeProgress) {
	notes := &fakeNotes{files: map[string]string{}}
	analyzer := &fakeAnalyzer{project: p, notes: notes}
	progress := &fakeProgress{}
	return New(analyzer, fakeRoots{}, notes, progress), analyzer, notes, progress
}

func TestImportCreatesThenLeavesUnchanged(t *testing.T) {
	svc, _, notes, progress := newTestService(project("internal/modules/vault/service", "cmd/api"))

	first, err := svc.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// index + 2 hubs + 2 units
	if first.Created != 5 || first.Units != 2 || first.IndexNote != "docs/cerebro/demo.md" {
		t.Fatalf("first import = %+v", first)
	}
	if progress.last != progress.total || progress.total == 0 {
		t.Errorf("progress ended at %d/%d", progress.last, progress.total)
	}

	saves := notes.saves
	second, err := svc.Import(context.Background())
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if second.Unchanged != 5 || second.Created+second.Updated+second.Removed != 0 || notes.saves != saves {
		t.Fatalf("re-import was not a no-op: %+v (saves %d -> %d)", second, saves, notes.saves)
	}
}

func TestImportKeepsUserTextAndRetiresStaleNotes(t *testing.T) {
	svc, analyzer, notes, _ := newTestService(project("internal/modules/vault/service", "cmd/api", "old/pkg"))
	if _, err := svc.Import(context.Background()); err != nil {
		t.Fatal(err)
	}
	notes.files["docs/cerebro/cmd/api.md"] += "Minha anotação.\n"

	// "old/pkg" disappears; "cmd/api" too, but it holds user text.
	analyzer.project = project("internal/modules/vault/service")
	result, err := svc.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if _, ok := notes.files["docs/cerebro/old/pkg.md"]; ok {
		t.Error("stale note without user text was not removed")
	}
	if !strings.Contains(notes.files["docs/cerebro/cmd/api.md"], "Minha anotação.") {
		t.Error("note with user text was removed")
	}
	// Kept: cmd/api.md (user text). Removed: old/pkg.md and the two hubs
	// left without units ("Módulo old", "Módulo cmd").
	if len(result.Kept) != 1 || result.Kept[0] != "docs/cerebro/cmd/api.md" || result.Removed != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestImportNeverOverwritesForeignNotes(t *testing.T) {
	svc, _, notes, _ := newTestService(project("cmd/api"))
	notes.files["docs/cerebro/demo.md"] = "# Minha página escrita à mão\n"

	result, err := svc.Import(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if notes.files["docs/cerebro/demo.md"] != "# Minha página escrita à mão\n" {
		t.Fatal("hand-written note was overwritten")
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "docs/cerebro/demo.md" {
		t.Fatalf("skipped = %v", result.Skipped)
	}
}

func TestImportRequiresSourceCode(t *testing.T) {
	svc, _, _, _ := newTestService(model.Project{Name: "empty"})
	_, err := svc.Import(context.Background())
	if errs.CategoryOf(err) != errs.InvalidInput {
		t.Fatalf("error = %v, want invalid_input", err)
	}
}

func TestImportRequiresOpenVault(t *testing.T) {
	svc := New(&fakeAnalyzer{notes: &fakeNotes{}}, fakeRoots{err: errs.ErrNoVaultOpen}, &fakeNotes{}, &fakeProgress{})
	if _, err := svc.Import(context.Background()); !errors.Is(err, errs.ErrNoVaultOpen) {
		t.Fatalf("error = %v, want ErrNoVaultOpen", err)
	}
}
