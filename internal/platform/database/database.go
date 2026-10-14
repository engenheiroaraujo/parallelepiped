// Package database opens SQLite databases, applies embedded schema versions
// and provides the shared transaction helper.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite file at filePath and applies every
// migration in migrations not yet recorded in PRAGMA user_version.
//
// Migration files are named "<version>_<description>.sql" with a positive
// integer version. Applying on open is deliberate: the databases this app
// owns are local, single-user and derived from the vault (see
// docs/ARCHITECTURE.md).
func Open(ctx context.Context, filePath string, migrations fs.FS) (*sql.DB, error) {
	query := url.Values{}
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	dsn := "file:" + toURLPath(filePath) + "?" + query.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection serialises writes and keeps PRAGMAs consistent.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(ctx, db, migrations); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// WithTx runs fn inside a transaction: rollback on error or panic, commit on
// success.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type migration struct {
	version int
	name    string
}

func migrate(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	var pending []migration
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if err != nil || version <= 0 {
			return fmt.Errorf("invalid migration name %q", entry.Name())
		}
		if version > current {
			pending = append(pending, migration{version: version, name: entry.Name()})
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].version < pending[j].version })

	for _, m := range pending {
		script, err := fs.ReadFile(migrations, m.name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.name, err)
		}
		err = WithTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(script)); err != nil {
				return fmt.Errorf("apply migration %s: %w", m.name, err)
			}
			// PRAGMA does not accept bound parameters; version is an int.
			_, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(m.version))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// toURLPath turns a Windows or POSIX path into the path part of a file: URI.
func toURLPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) > 1 && p[1] == ':' {
		p = "/" + p
	}
	return (&url.URL{Path: p}).EscapedPath()
}
