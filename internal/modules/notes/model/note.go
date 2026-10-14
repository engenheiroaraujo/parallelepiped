// Package model holds the note entity and the rules for note paths.
package model

import (
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"parallelepiped/internal/platform/validation"
)

const (
	// Extension is the file extension of every note.
	Extension = ".md"
	// MaxPathLength bounds a relative note path, in runes.
	MaxPathLength = 240
	// MaxContentBytes bounds a single note.
	MaxContentBytes = 10 << 20
)

// forbiddenNameChars cannot appear in file names on Windows.
const forbiddenNameChars = `<>:"|?*`

// Note is a Markdown file inside the vault.
type Note struct {
	// Path is relative to the vault root, slash-separated, with extension.
	Path       string
	Title      string
	Content    string
	ModifiedAt time.Time
}

// NewNote validates path and content and returns the note.
func NewNote(rawPath, content string, modifiedAt time.Time) (Note, error) {
	var form validation.FormValidator

	notePath, pathErr := normalizePath(rawPath)
	if pathErr != "" {
		form.AddFieldError("path", pathErr)
	}
	if len(content) > MaxContentBytes {
		form.AddFieldError("content", "A nota passou do tamanho máximo de 10 MB.")
	}
	if !utf8.ValidString(content) {
		form.AddFieldError("content", "O conteúdo não é texto UTF-8 válido.")
	}
	if err := form.Err(); err != nil {
		return Note{}, err
	}

	return Note{Path: notePath, Title: TitleFromPath(notePath), Content: content, ModifiedAt: modifiedAt}, nil
}

// NormalizePath validates a user-supplied relative note path and returns its
// canonical form: slash-separated, cleaned and ending in ".md".
func NormalizePath(raw string) (string, error) {
	notePath, problem := normalizePath(raw)
	if problem != "" {
		return "", validation.Field("path", problem)
	}
	return notePath, nil
}

// TitleFromPath returns the file name without directory and extension.
func TitleFromPath(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}

func normalizePath(raw string) (string, string) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	if p == "" {
		return "", "Informe o nome da nota."
	}
	if strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':') {
		return "", "O caminho deve ser relativo ao cofre."
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", "O caminho não pode sair do cofre."
	}
	for _, segment := range strings.Split(p, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", "Pastas e arquivos ocultos não são permitidos."
		}
		if strings.ContainsAny(segment, forbiddenNameChars) {
			return "", "O nome não pode conter " + forbiddenNameChars + "."
		}
	}
	if !strings.EqualFold(path.Ext(p), Extension) {
		p += Extension
	}
	if utf8.RuneCountInString(p) > MaxPathLength {
		return "", "O caminho é longo demais."
	}
	return p, ""
}
