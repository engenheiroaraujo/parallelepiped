package model

import (
	"strings"
	"unicode"
)

const (
	// MaxQueryLength bounds the search text, in runes.
	MaxQueryLength = 200
	// DefaultSearchLimit and MaxSearchLimit bound the number of hits.
	DefaultSearchLimit = 50
	MaxSearchLimit     = 200
)

// Snippet highlight markers. Control characters cannot appear in typed text,
// so the frontend can split on them safely.
const (
	HighlightStart = "\x02"
	HighlightEnd   = "\x03"
)

// SearchHit is one full-text search result.
type SearchHit struct {
	Path    string
	Title   string
	Snippet string
}

// Backlink is a note that links to another one.
type Backlink struct {
	Path  string
	Title string
}

// FTSQuery turns free text into an FTS5 MATCH expression: every word becomes
// a quoted prefix term, so user input never reaches FTS5 syntax.
// It returns "" when the text has no searchable word.
func FTSQuery(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
	})
	terms := make([]string, 0, len(words))
	for _, word := range words {
		terms = append(terms, `"`+word+`"*`)
	}
	return strings.Join(terms, " ")
}
