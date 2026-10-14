package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"parallelepiped/internal/modules/index/model"
	"parallelepiped/migrations"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(context.Background(), filepath.Join(t.TempDir(), "index.db"), migrations.FS)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func doc(path, content string) model.Document {
	return model.ParseDocument(model.SourceFile{Path: path, Content: content, ModifiedAt: time.Unix(1, 0)})
}

func TestStoreBacklinksSearchAndGraph(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	err := store.Replace(ctx, []model.Document{
		doc("Alpha.md", "Links to [[Beta]] and [[projects/Gamma]] and [[Missing]]."),
		doc("Beta.md", "Back to [[alpha]]. Café com leite."),
		doc("projects/Gamma.md", "No links here."),
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	backlinks, err := store.Backlinks(ctx, "Beta.md")
	if err != nil || len(backlinks) != 1 || backlinks[0].Path != "Alpha.md" {
		t.Fatalf("Backlinks(Beta) = %+v, %v", backlinks, err)
	}
	backlinks, err = store.Backlinks(ctx, "projects/Gamma.md")
	if err != nil || len(backlinks) != 1 {
		t.Fatalf("Backlinks(Gamma) by path link = %+v, %v", backlinks, err)
	}

	hits, err := store.Search(ctx, model.FTSQuery("cafe"), 10)
	if err != nil || len(hits) != 1 || hits[0].Path != "Beta.md" {
		t.Fatalf("Search(cafe) without accent = %+v, %v", hits, err)
	}

	notes, edges, unresolved, err := store.GraphData(ctx)
	if err != nil {
		t.Fatalf("GraphData: %v", err)
	}
	if len(notes) != 3 || len(edges) != 3 || len(unresolved) != 1 ||
		unresolved[0].TargetKey != "missing" || unresolved[0].Target != "Missing" {
		t.Fatalf("GraphData = %d notes, %v edges, %v unresolved", len(notes), edges, unresolved)
	}
}

func TestStoreUpsertAndRemove(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	if err := store.Replace(ctx, []model.Document{doc("A.md", "[[B]]"), doc("B.md", "")}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if err := store.Upsert(ctx, doc("A.md", "no more links, zebra")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if backlinks, _ := store.Backlinks(ctx, "B.md"); len(backlinks) != 0 {
		t.Fatalf("old link survived upsert: %+v", backlinks)
	}
	if hits, _ := store.Search(ctx, model.FTSQuery("zebra"), 10); len(hits) != 1 {
		t.Fatalf("upserted text not searchable: %+v", hits)
	}

	if err := store.Remove(ctx, "A.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if hits, _ := store.Search(ctx, model.FTSQuery("zebra"), 10); len(hits) != 0 {
		t.Fatalf("removed note still searchable: %+v", hits)
	}
}

func TestStoreBatchesLargeReplace(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	docs := make([]model.Document, 0, rowsPerInsert*2+7)
	for i := range cap(docs) {
		docs = append(docs, doc(filepath.ToSlash(filepath.Join("bulk", string(rune('a'+i%26))+itoa(i)+".md")), "[[Hub]]"))
	}
	if err := store.Replace(ctx, docs); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	notes, _, unresolved, err := store.GraphData(ctx)
	if err != nil || len(notes) != len(docs) || len(unresolved) != len(docs) {
		t.Fatalf("GraphData = %d notes, %d unresolved, %v", len(notes), len(unresolved), err)
	}
}

func TestScannerSkipsHiddenAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Note.md", "a")
	write("sub/Deep.MD", "b")
	write(".obsidian/Config.md", "hidden")
	write(".trash/Old.md", "hidden")
	write("image.png", "x")

	files, err := Scanner{}.Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[f.Path] = true
	}
	if len(got) != 2 || !got["Note.md"] || !got["sub/Deep.MD"] {
		t.Fatalf("Scan found %v", got)
	}
}

func itoa(i int) string {
	return string([]byte{byte('0' + i/100%10), byte('0' + i/10%10), byte('0' + i%10)})
}
