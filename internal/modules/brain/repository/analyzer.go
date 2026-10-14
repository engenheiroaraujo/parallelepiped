// Package repository reads a code project from disk (without executing it)
// and lists the notes already in the vault.
package repository

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"parallelepiped/internal/modules/brain/model"
	"parallelepiped/internal/platform/vaultpath"
)

// maxSourceBytes skips generated or minified files too large to be useful.
const maxSourceBytes = 1 << 20

// skippedDirs hold build output or fixtures rather than the project's code.
var skippedDirs = map[string]bool{
	"dist": true, "build": true, "bin": true, "out": true,
	"coverage": true, "testdata": true, "wailsjs": true,
}

// languages maps source extensions to the language shown in the notes.
var languages = map[string]string{
	".go": model.LanguageGo,
	".ts": "TypeScript", ".tsx": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript",
	".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".py": "Python", ".java": "Java", ".kt": "Kotlin", ".cs": "C#", ".rs": "Rust",
	".rb": "Ruby", ".php": "PHP", ".swift": "Swift", ".vue": "Vue", ".svelte": "Svelte",
	".c": "C", ".h": "C", ".cpp": "C++", ".hpp": "C++", ".sql": "SQL",
}

var (
	goModuleLine = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	// Relative imports of JavaScript/TypeScript modules: import/export ...
	// from "x", import "x", import("x") and require("x").
	jsImport = regexp.MustCompile(`(?:from|import|require)\s*\(?\s*['"](\.{1,2}/[^'"]+)['"]`)
)

// Analyzer reads the structure of a project.
type Analyzer struct{}

type unitBuilder struct {
	unit  model.Unit
	paths []string // absolute paths of non-test source files
	langs map[string]int
}

// Analyze walks root and returns its folders with source code. Hidden and
// dependency folders, build output and the generated notes are skipped.
func (Analyzer) Analyze(ctx context.Context, root string) (model.Project, error) {
	project := model.Project{Name: filepath.Base(root)}
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		if m := goModuleLine.FindSubmatch(data); m != nil {
			project.ModulePath = string(m[1])
		}
	}

	builders := map[string]*unitBuilder{}
	err := filepath.WalkDir(root, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := relative(root, abs)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel != "." && (vaultpath.IsExcludedName(entry.Name()) || skippedDirs[entry.Name()] || rel == model.OutputDir) {
				return filepath.SkipDir
			}
			return nil
		}
		language, ok := languages[strings.ToLower(path.Ext(entry.Name()))]
		if !ok || vaultpath.IsExcludedName(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > maxSourceBytes {
			return err
		}

		dir := path.Dir(rel)
		b := builders[dir]
		if b == nil {
			b = &unitBuilder{unit: model.Unit{Dir: dir}, langs: map[string]int{}}
			builders[dir] = b
		}
		b.langs[language]++
		if isTestFile(entry.Name()) {
			b.unit.TestFiles++
			return nil
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		b.unit.Files = append(b.unit.Files, model.SourceFile{Name: entry.Name(), Lines: countLines(content)})
		b.paths = append(b.paths, abs)
		return nil
	})
	if err != nil {
		return model.Project{}, fmt.Errorf("analyze project: %w", err)
	}

	for _, b := range builders {
		b.unit.Language = mainLanguage(b.langs)
		sort.Slice(b.unit.Files, func(i, j int) bool { return b.unit.Files[i].Name < b.unit.Files[j].Name })
		sort.Strings(b.paths)
	}
	for _, b := range builders {
		switch {
		case b.unit.Language == model.LanguageGo:
			readGoPackage(b, project.ModulePath, builders)
		default:
			b.unit.DependsOn = jsDependencies(b, builders)
		}
		project.Units = append(project.Units, b.unit)
	}
	sort.Slice(project.Units, func(i, j int) bool { return project.Units[i].Dir < project.Units[j].Dir })
	return project, nil
}

// readGoPackage fills package name, documentation, exported symbols and the
// in-module imports of a Go unit.
func readGoPackage(b *unitBuilder, modulePath string, units map[string]*unitBuilder) {
	fset := token.NewFileSet()
	deps := map[string]bool{}
	for _, abs := range b.paths {
		if filepath.Ext(abs) != ".go" {
			continue
		}
		file, err := parser.ParseFile(fset, abs, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			continue // a file that does not parse still counts in the listing
		}
		if b.unit.Package == "" {
			b.unit.Package = file.Name.Name
		}
		if b.unit.Doc == "" && file.Doc != nil {
			b.unit.Doc = firstSentence(file.Doc.Text())
		}
		for _, spec := range file.Imports {
			if dir, ok := moduleDir(strings.Trim(spec.Path.Value, `"`), modulePath); ok && dir != b.unit.Dir && units[dir] != nil {
				deps[dir] = true
			}
		}
		b.unit.Exported = append(b.unit.Exported, exportedSymbols(file)...)
	}
	if modulePath != "" {
		b.unit.ImportPath = path.Join(modulePath, b.unit.Dir)
	}
	sort.Slice(b.unit.Exported, func(i, j int) bool {
		if b.unit.Exported[i].Kind != b.unit.Exported[j].Kind {
			return kindOrder(b.unit.Exported[i].Kind) < kindOrder(b.unit.Exported[j].Kind)
		}
		return b.unit.Exported[i].Name < b.unit.Exported[j].Name
	})
	b.unit.DependsOn = sortedKeys(deps)
}

func exportedSymbols(file *ast.File) []model.Symbol {
	var symbols []model.Symbol
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			if d.Recv == nil || len(d.Recv.List) == 0 {
				symbols = append(symbols, model.Symbol{Kind: "func", Name: d.Name.Name})
			} else if recv := receiverName(d.Recv.List[0].Type); ast.IsExported(recv) {
				symbols = append(symbols, model.Symbol{Kind: "method", Name: recv + "." + d.Name.Name})
			}
		case *ast.GenDecl:
			kind := map[token.Token]string{token.TYPE: "type", token.CONST: "const", token.VAR: "var"}[d.Tok]
			if kind == "" {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						symbols = append(symbols, model.Symbol{Kind: kind, Name: s.Name.Name})
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							symbols = append(symbols, model.Symbol{Kind: kind, Name: name.Name})
						}
					}
				}
			}
		}
	}
	return symbols
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// moduleDir maps an import path inside the module to its folder.
func moduleDir(importPath, modulePath string) (string, bool) {
	if modulePath == "" {
		return "", false
	}
	if importPath == modulePath {
		return ".", true
	}
	if rest, ok := strings.CutPrefix(importPath, modulePath+"/"); ok {
		return rest, true
	}
	return "", false
}

// jsDependencies resolves relative JavaScript/TypeScript imports to the
// folders that hold the imported modules.
func jsDependencies(b *unitBuilder, units map[string]*unitBuilder) []string {
	deps := map[string]bool{}
	for _, abs := range b.paths {
		switch strings.ToLower(filepath.Ext(abs)) {
		case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte":
		default:
			continue
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		for _, m := range jsImport.FindAllSubmatch(content, -1) {
			target := path.Clean(path.Join(b.unit.Dir, string(m[1])))
			for _, dir := range []string{target, path.Dir(target)} {
				if dir != b.unit.Dir && units[dir] != nil {
					deps[dir] = true
					break
				}
			}
		}
	}
	return sortedKeys(deps)
}

// VaultNotes lists the Markdown notes already in the vault: the lower-case
// titles of notes outside the generated folder, and the paths of the notes
// inside it.
func (Analyzer) VaultNotes(ctx context.Context, root string) (map[string]bool, []string, error) {
	titles := map[string]bool{}
	var generated []string
	err := filepath.WalkDir(root, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if abs != root && vaultpath.IsExcludedName(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		rel, err := relative(root, abs)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, model.OutputDir+"/") {
			generated = append(generated, rel)
		} else {
			name := entry.Name()
			titles[strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))] = true
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list vault notes: %w", err)
	}
	sort.Strings(generated)
	return titles, generated, nil
}

func relative(root, abs string) (string, error) {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func isTestFile(name string) bool {
	lower := strings.ToLower(name)
	base := strings.TrimSuffix(lower, path.Ext(lower))
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".spec") ||
		strings.HasPrefix(lower, "test_") && strings.HasSuffix(lower, ".py") ||
		strings.HasSuffix(lower, "_test.py")
}

func mainLanguage(counts map[string]int) string {
	best, bestCount := "", -1
	for language, count := range counts {
		if count > bestCount || count == bestCount && language < best {
			best, bestCount = language, count
		}
	}
	return best
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := bytes.Count(content, []byte("\n"))
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines
}

// firstSentence returns the first sentence of a doc comment, on one line.
func firstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if i := strings.Index(text, ". "); i >= 0 {
		return text[:i+1]
	}
	return text
}

func kindOrder(kind string) int {
	return map[string]int{"type": 0, "func": 1, "method": 2, "const": 3, "var": 4}[kind]
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
