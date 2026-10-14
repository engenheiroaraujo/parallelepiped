package model

import (
	"reflect"
	"testing"
)

func TestParseLinks(t *testing.T) {
	content := "See [[Alpha]] and [[folder/Beta|the beta]] plus ![[Image.png]].\n" +
		"Heading link [[Gamma#Intro]] and block [[Delta^abc]].\n" +
		"Inline `[[NotALink]]` code.\n" +
		"```\n[[InsideFence]]\n```\n" +
		"[[ ]] is empty."

	got := ParseLinks(content)
	want := []Link{
		{Target: "Alpha"},
		{Target: "folder/Beta", Alias: "the beta"},
		{Target: "Image.png", Embed: true},
		{Target: "Gamma"},
		{Target: "Delta"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseLinks:\n got %+v\nwant %+v", got, want)
	}
}

func TestParseTags(t *testing.T) {
	content := "# Heading\n#Projeto and #ideia/sub, again #projeto.\n" +
		"Issue #123 is not a tag; `#code` neither.\n" +
		"~~~\n#fenced\n~~~\n(#paren)"

	got := ParseTags(content)
	want := []string{"projeto", "ideia/sub", "paren"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTags: got %v, want %v", got, want)
	}
}

func TestParseDocument(t *testing.T) {
	doc := ParseDocument(SourceFile{Path: "Area/Sub/Note One.md", Content: "[[x]]"})
	if doc.Title != "Note One" || doc.Folder != "Area" || len(doc.Links) != 1 {
		t.Fatalf("unexpected document: %+v", doc)
	}
	if root := ParseDocument(SourceFile{Path: "Root.md"}); root.Folder != "" {
		t.Fatalf("root note folder = %q, want empty", root.Folder)
	}
}

func TestLinkKey(t *testing.T) {
	cases := map[string]string{
		"Note":            "note",
		"Folder/Note.md":  "folder/note",
		` Folder\Note.MD`: "folder/note",
	}
	for in, want := range cases {
		if got := LinkKey(in); got != want {
			t.Errorf("LinkKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFTSQuery(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"  ***  ":          "",
		"hello":            `"hello"*`,
		`go "AND" NOT x-y`: `"go"* "AND"* "NOT"* "x"* "y"*`,
		"ação rápida_2":    `"ação"* "rápida_2"*`,
	}
	for in, want := range cases {
		if got := FTSQuery(in); got != want {
			t.Errorf("FTSQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildGraph(t *testing.T) {
	notes := []NoteRef{
		{Path: "a.md", Title: "a"},
		{Path: "dir/b.md", Title: "b", Folder: "dir"},
	}
	resolved := []GraphEdge{
		{Source: "a.md", Target: "dir/b.md"},
		{Source: "a.md", Target: "dir/b.md"}, // duplicate
		{Source: "a.md", Target: "a.md"},     // self link
	}
	unresolved := []UnresolvedLink{{Source: "dir/b.md", TargetKey: "missing", Target: "Missing"}}

	g := BuildGraph(notes, resolved, unresolved)

	for _, n := range g.Nodes {
		if n.Unresolved && n.Title != "Missing" {
			t.Fatalf("unresolved node title = %q, want original casing", n.Title)
		}
	}

	if len(g.Edges) != 2 {
		t.Fatalf("edges = %+v, want 2", g.Edges)
	}
	degree := map[string]int{}
	for _, n := range g.Nodes {
		degree[n.ID] = n.Degree
	}
	want := map[string]int{"a.md": 1, "dir/b.md": 2, UnresolvedGroup + "/missing": 1}
	if !reflect.DeepEqual(degree, want) {
		t.Fatalf("degrees = %v, want %v", degree, want)
	}
}
