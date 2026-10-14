// Package repository persists the index in SQLite and reads source files
// from the vault.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	"parallelepiped/internal/modules/index/model"
	"parallelepiped/internal/platform/database"
)

// rowsPerInsert bounds multi-row INSERTs. With at most 6 columns per row this
// stays far below SQLite's bound-parameter limit (32766).
const rowsPerInsert = 200

// Store is the SQLite-backed index of one vault.
type Store struct {
	db *sql.DB
}

// OpenStore opens or creates the index database at filePath, applying the
// schema in migrations (the root migrations package).
func OpenStore(ctx context.Context, filePath string, migrations fs.FS) (*Store, error) {
	db, err := database.Open(ctx, filePath, migrations)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// Replace discards the whole index and stores docs in its place.
func (s *Store) Replace(ctx context.Context, docs []model.Document) error {
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, table := range []string{"links", "tags", "notes", "notes_fts"} {
			// Table names come from the fixed list above, never from input.
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return fmt.Errorf("clear %s: %w", table, err)
			}
		}
		return insertDocuments(ctx, tx, docs)
	})
}

// Upsert replaces the indexed data of a single document.
func (s *Store) Upsert(ctx context.Context, doc model.Document) error {
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := deleteDocument(ctx, tx, doc.Path); err != nil {
			return err
		}
		return insertDocuments(ctx, tx, []model.Document{doc})
	})
}

// Remove drops a document from the index. Removing a missing path is not an error.
func (s *Store) Remove(ctx context.Context, path string) error {
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return deleteDocument(ctx, tx, path)
	})
}

// Backlinks lists the notes that link to path, ordered by title.
func (s *Store) Backlinks(ctx context.Context, path string) ([]model.Backlink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT source.path, source.title
		FROM   notes target
		JOIN   links l       ON l.target_key IN (target.title_key, target.path_key)
		JOIN   notes source  ON source.path = l.source_path
		WHERE  target.path = ?
		  AND  source.path <> target.path
		ORDER  BY source.title COLLATE NOCASE`, path)
	if err != nil {
		return nil, fmt.Errorf("query backlinks: %w", err)
	}
	defer rows.Close()

	var backlinks []model.Backlink
	for rows.Next() {
		var b model.Backlink
		if err := rows.Scan(&b.Path, &b.Title); err != nil {
			return nil, fmt.Errorf("scan backlink: %w", err)
		}
		backlinks = append(backlinks, b)
	}
	return backlinks, rows.Err()
}

// Search runs an FTS5 MATCH expression built by model.FTSQuery.
func (s *Store) Search(ctx context.Context, match string, limit int) ([]model.SearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, title, snippet(notes_fts, 2, ?, ?, '…', 12)
		FROM   notes_fts
		WHERE  notes_fts MATCH ?
		ORDER  BY rank
		LIMIT  ?`, model.HighlightStart, model.HighlightEnd, match, limit)
	if err != nil {
		return nil, fmt.Errorf("query search: %w", err)
	}
	defer rows.Close()

	var hits []model.SearchHit
	for rows.Next() {
		var hit model.SearchHit
		if err := rows.Scan(&hit.Path, &hit.Title, &hit.Snippet); err != nil {
			return nil, fmt.Errorf("scan search hit: %w", err)
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

// GraphData returns the notes, the resolved links and the unresolved links.
func (s *Store) GraphData(ctx context.Context) ([]model.NoteRef, []model.GraphEdge, []model.UnresolvedLink, error) {
	notes, err := s.noteRefs(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	edgeRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT l.source_path, target.path
		FROM   links l
		JOIN   notes target ON l.target_key IN (target.title_key, target.path_key)`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("query graph edges: %w", err)
	}
	defer edgeRows.Close()
	var edges []model.GraphEdge
	for edgeRows.Next() {
		var edge model.GraphEdge
		if err := edgeRows.Scan(&edge.Source, &edge.Target); err != nil {
			return nil, nil, nil, fmt.Errorf("scan graph edge: %w", err)
		}
		edges = append(edges, edge)
	}
	if err := edgeRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	missingRows, err := s.db.QueryContext(ctx, `
		SELECT l.source_path, l.target_key, min(l.target)
		FROM   links l
		WHERE  NOT EXISTS (
		           SELECT 1 FROM notes target
		           WHERE  l.target_key IN (target.title_key, target.path_key))
		GROUP  BY l.source_path, l.target_key`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("query unresolved links: %w", err)
	}
	defer missingRows.Close()
	var unresolved []model.UnresolvedLink
	for missingRows.Next() {
		var link model.UnresolvedLink
		if err := missingRows.Scan(&link.Source, &link.TargetKey, &link.Target); err != nil {
			return nil, nil, nil, fmt.Errorf("scan unresolved link: %w", err)
		}
		unresolved = append(unresolved, link)
	}
	return notes, edges, unresolved, missingRows.Err()
}

func (s *Store) noteRefs(ctx context.Context) ([]model.NoteRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, title, folder FROM notes ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var notes []model.NoteRef
	for rows.Next() {
		var note model.NoteRef
		if err := rows.Scan(&note.Path, &note.Title, &note.Folder); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func deleteDocument(ctx context.Context, tx *sql.Tx, path string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE path = ?`, path); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts WHERE path = ?`, path); err != nil {
		return fmt.Errorf("delete note text: %w", err)
	}
	return nil
}

func insertDocuments(ctx context.Context, tx *sql.Tx, docs []model.Document) error {
	var notes, links, tags, texts [][]any
	for _, doc := range docs {
		notes = append(notes, []any{
			doc.Path, doc.Title, doc.Folder,
			strings.ToLower(doc.Title), model.LinkKey(doc.Path), doc.ModifiedAt.Unix(),
		})
		texts = append(texts, []any{doc.Path, doc.Title, doc.Content})
		for _, link := range doc.Links {
			links = append(links, []any{doc.Path, link.Target, model.LinkKey(link.Target), link.Alias, link.Embed})
		}
		for _, tag := range doc.Tags {
			tags = append(tags, []any{doc.Path, tag})
		}
	}

	if err := insertRows(ctx, tx, `INSERT INTO notes (path, title, folder, title_key, path_key, modified_at) VALUES `, notes); err != nil {
		return fmt.Errorf("insert notes: %w", err)
	}
	if err := insertRows(ctx, tx, `INSERT INTO links (source_path, target, target_key, alias, embed) VALUES `, links); err != nil {
		return fmt.Errorf("insert links: %w", err)
	}
	if err := insertRows(ctx, tx, `INSERT OR IGNORE INTO tags (path, tag) VALUES `, tags); err != nil {
		return fmt.Errorf("insert tags: %w", err)
	}
	if err := insertRows(ctx, tx, `INSERT INTO notes_fts (path, title, content) VALUES `, texts); err != nil {
		return fmt.Errorf("insert note text: %w", err)
	}
	return nil
}

// insertRows executes insert (which ends in "VALUES ") in blocks of
// rowsPerInsert rows, appending one "(?, ?, ...)" group per row.
func insertRows(ctx context.Context, tx *sql.Tx, insert string, rows [][]any) error {
	for start := 0; start < len(rows); start += rowsPerInsert {
		block := rows[start:min(start+rowsPerInsert, len(rows))]
		group := "(" + strings.TrimSuffix(strings.Repeat("?, ", len(block[0])), ", ") + ")"

		var query strings.Builder
		query.WriteString(insert)
		args := make([]any, 0, len(block)*len(block[0]))
		for i, row := range block {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(group)
			args = append(args, row...)
		}
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return err
		}
	}
	return nil
}
