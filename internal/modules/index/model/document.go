// Package model holds the index vocabulary: parsed documents, links, tags and
// the graph built from them. Everything here is pure and testable without a
// database or filesystem.
package model

import (
	"path"
	"regexp"
	"strings"
	"time"
)

// Document is a vault note as the index sees it.
type Document struct {
	// Path is relative to the vault root, slash-separated, with extension.
	Path       string
	Title      string
	Folder     string
	Content    string
	Links      []Link
	Tags       []string
	ModifiedAt time.Time
}

// Link is a [[wikilink]] found in a document.
type Link struct {
	// Target is the link text before any "#heading" or "|alias".
	Target string
	Alias  string
	// Embed is true for ![[...]] transclusions.
	Embed bool
}

// SourceFile is a raw file read from the vault before parsing.
type SourceFile struct {
	Path       string
	Content    string
	ModifiedAt time.Time
}

var (
	wikilinkPattern = regexp.MustCompile(`(!?)\[\[([^\[\]|#^]+)(?:[#^][^\[\]|]*)?(?:\|([^\[\]]*))?\]\]`)
	tagPattern      = regexp.MustCompile(`(?:^|[\s(])#([\p{L}\p{N}_\-/]+)`)
	inlineCode      = regexp.MustCompile("`[^`\n]*`")
	onlyDigits      = regexp.MustCompile(`^[0-9]+$`)
)

// ParseDocument extracts title, folder, links and tags from a source file.
func ParseDocument(file SourceFile) Document {
	return Document{
		Path:       file.Path,
		Title:      TitleFromPath(file.Path),
		Folder:     FolderOf(file.Path),
		Content:    file.Content,
		Links:      ParseLinks(file.Content),
		Tags:       ParseTags(file.Content),
		ModifiedAt: file.ModifiedAt,
	}
}

// TitleFromPath returns the file name without directory and extension.
func TitleFromPath(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}

// FolderOf returns the folder that directly contains p, or "" for notes at
// the root. It is the grouping used to colour the graph: the immediate
// folder tells notes apart even when a whole vault lives under one top-level
// folder such as "docs".
func FolderOf(p string) string {
	dir := path.Dir(p)
	if dir == "." {
		return ""
	}
	return dir
}

// LinkKey normalises a link target or note path for resolution: lower case,
// slash-separated, without the .md extension.
func LinkKey(target string) string {
	key := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(target, `\`, "/")))
	return strings.TrimSuffix(key, ".md")
}

// ParseLinks returns the wikilinks outside code, in order of appearance.
func ParseLinks(content string) []Link {
	var links []Link
	for _, line := range proseLines(content) {
		for _, match := range wikilinkPattern.FindAllStringSubmatch(line, -1) {
			target := strings.TrimSpace(match[2])
			if target == "" {
				continue
			}
			links = append(links, Link{
				Target: target,
				Alias:  strings.TrimSpace(match[3]),
				Embed:  match[1] == "!",
			})
		}
	}
	return links
}

// ParseTags returns the distinct #tags outside code, lower-cased, in order of
// first appearance. Purely numeric tokens such as "#123" are not tags.
func ParseTags(content string) []string {
	seen := make(map[string]bool)
	var tags []string
	for _, line := range proseLines(content) {
		// Headings ("# Title") never match: the pattern needs a word right after '#'.
		for _, match := range tagPattern.FindAllStringSubmatch(line, -1) {
			tag := strings.ToLower(strings.Trim(match[1], "/"))
			if tag == "" || onlyDigits.MatchString(tag) || seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

// proseLines returns the lines outside fenced code blocks, with inline code
// spans blanked out.
func proseLines(content string) []string {
	var lines []string
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		lines = append(lines, inlineCode.ReplaceAllString(line, ""))
	}
	return lines
}
