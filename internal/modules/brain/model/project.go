// Package model holds the vocabulary of the project import: the structure
// read from a code project, the rules that name and group the notes, and the
// rendering of those notes as Markdown. Everything here is pure.
package model

import (
	"path"
	"sort"
	"strings"
)

// OutputDir is where generated notes live, relative to the vault root.
const OutputDir = "docs/cerebro"

// Language names shown in the notes.
const (
	LanguageGo    = "Go"
	LanguageOther = ""
)

// Project is the structure of a code project, read without executing it.
type Project struct {
	// Name is the project folder name.
	Name string
	// ModulePath is the Go module path from go.mod, or "".
	ModulePath string
	Units      []Unit
}

// Unit is a folder that holds source code.
type Unit struct {
	// Dir is slash-separated and relative to the project root; "." is the root.
	Dir      string
	Language string
	Files    []SourceFile
	// TestFiles counts test files, which are not listed individually.
	TestFiles int
	// Go-only fields.
	Package    string
	ImportPath string
	Doc        string
	Exported   []Symbol
	// DependsOn lists the Dir of other units this one imports.
	DependsOn []string
}

// SourceFile is one non-test source file of a unit.
type SourceFile struct {
	Name  string
	Lines int
}

// Symbol is an exported declaration of a Go package.
type Symbol struct {
	Kind string // "type", "func", "method", "const" or "var"
	Name string // methods are "Receiver.Name"
}

// containerSegments are wrapper folders that say nothing about the business
// module, so grouping looks past them.
var containerSegments = map[string]bool{
	"internal": true, "modules": true, "pkg": true, "src": true,
	"app": true, "lib": true, "libs": true, "packages": true,
}

// GroupOf returns the module a unit belongs to: its first folder that is not
// a generic wrapper ("internal/modules/vault/service" -> "vault",
// "internal/platform/errors" -> "platform", "cmd/api" -> "cmd"). Units at the
// root, or made only of wrappers, have no group ("").
func GroupOf(dir string) string {
	if dir == "." || dir == "" {
		return ""
	}
	for _, segment := range strings.Split(dir, "/") {
		if !containerSegments[segment] {
			return segment
		}
	}
	return ""
}

// HubTitle is the title of the note that gathers a group's units.
func HubTitle(group string) string {
	return "Módulo " + group
}

// RootUnitTitle is the title of the unit at the project root.
const RootUnitTitle = "raiz"

// UnitTitles gives each unit a title that is unique among the units and does
// not clash with reserved titles (existing notes, hubs, the index note):
// the shortest trailing part of its folder path that is unique, joined with
// dots ("vault.service"). Comparison ignores case, as link resolution does.
func UnitTitles(dirs []string, reserved map[string]bool) map[string]string {
	segments := make(map[string][]string, len(dirs))
	for _, dir := range dirs {
		if dir == "." || dir == "" {
			segments[dir] = []string{RootUnitTitle}
		} else {
			segments[dir] = strings.Split(dir, "/")
		}
	}

	taken := make(map[string]bool, len(reserved)+len(dirs))
	for key := range reserved {
		taken[key] = true
	}
	titles := make(map[string]string, len(dirs))
	assign := func(dir, title string) {
		titles[dir] = title
		taken[strings.ToLower(title)] = true
	}

	pending := append([]string(nil), dirs...)
	sort.Strings(pending)
	for depth := 1; len(pending) > 0; depth++ {
		candidates := make(map[string][]string)
		var keys []string
		for _, dir := range pending {
			key := strings.ToLower(suffix(segments[dir], depth))
			if _, seen := candidates[key]; !seen {
				keys = append(keys, key)
			}
			candidates[key] = append(candidates[key], dir)
		}
		sort.Strings(keys)

		var next []string
		for _, key := range keys {
			owners := candidates[key]
			var exhausted, deeper []string
			for _, owner := range owners {
				if depth >= len(segments[owner]) {
					exhausted = append(exhausted, owner)
				} else {
					deeper = append(deeper, owner)
				}
			}
			switch {
			case len(owners) == 1 && !taken[key]:
				assign(owners[0], suffix(segments[owners[0]], depth))
			case len(exhausted) == 1 && !taken[key]:
				// "b" and "a/b": the shorter path keeps the plain name and
				// the longer ones grow a level.
				assign(exhausted[0], suffix(segments[exhausted[0]], depth))
				next = append(next, deeper...)
			default:
				// A whole path that still clashes (with an existing note, or
				// one differing only in case) is marked as code.
				for _, owner := range exhausted {
					assign(owner, strings.Join(segments[owner], ".")+" (código)")
				}
				next = append(next, deeper...)
			}
		}
		sort.Strings(next)
		pending = next
	}
	return titles
}

func suffix(segments []string, depth int) string {
	if depth > len(segments) {
		depth = len(segments)
	}
	return strings.Join(segments[len(segments)-depth:], ".")
}

// NotePath returns where a note with the given title and group is stored.
func NotePath(group, title string) string {
	if group == "" {
		return path.Join(OutputDir, title+".md")
	}
	return path.Join(OutputDir, group, title+".md")
}
