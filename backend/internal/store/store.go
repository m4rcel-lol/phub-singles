// Package store owns every SQLite interaction: schema migration, queries and
// the small domain types the HTTP layer serialises.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo required
)

//go:embed all:migrations
var migrationsFS embed.FS

var (
	// ErrNotFound is returned when a lookup matches no row.
	ErrNotFound = errors.New("not found")
	// ErrInvalid marks a caller mistake that must surface as HTTP 400.
	ErrInvalid = errors.New("invalid input")
	// ErrConflict marks a uniqueness violation, e.g. a duplicate username.
	ErrConflict = errors.New("conflict")
)

// Store is a thin, dependency-free wrapper around the SQLite connection pool.
type Store struct {
	db *sql.DB
}

// Open creates (or opens) the database file and applies pending migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	// WAL keeps readers from blocking the writer; busy_timeout absorbs the
	// short lock contention that remains under concurrent writes.
	dsn := "file:" + url.PathEscape(path) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(on)" +
		"&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// Ping reports whether the database still answers, for the readiness probe.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// migrate applies every embedded .sql file exactly once, in filename order.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name        TEXT PRIMARY KEY,
		applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var seen int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&seen); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if seen > 0 {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// AppliedMigrations lists the migrations recorded in the database, oldest first.
func (s *Store) AppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM schema_migrations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// nowUTC is the single timestamp format used across the schema (RFC3339, UTC).
func nowUTC() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

func today() string { return time.Now().UTC().Format("2006-01-02") }
