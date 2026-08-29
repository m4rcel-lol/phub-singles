package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Link is one button on the public page.
type Link struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Icon      string `json:"icon"`
	Enabled   bool   `json:"enabled"`
	Position  int    `json:"position"`
	Clicks    int64  `json:"clicks"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// LinkInput carries the writable fields of a link.
type LinkInput struct {
	Title   string
	URL     string
	Icon    string
	Enabled bool
}

const linkColumns = `id, title, url, icon, enabled, position, clicks, created_at, updated_at`

func scanLink(row interface{ Scan(...any) error }) (Link, error) {
	var l Link
	var enabled int
	if err := row.Scan(&l.ID, &l.Title, &l.URL, &l.Icon, &enabled, &l.Position, &l.Clicks,
		&l.CreatedAt, &l.UpdatedAt); err != nil {
		return Link{}, err
	}
	l.Enabled = enabled == 1
	return l, nil
}

// Links returns links ordered for display. When onlyEnabled is true the
// disabled ones are filtered out (the public page path).
func (s *Store) Links(ctx context.Context, onlyEnabled bool) ([]Link, error) {
	query := `SELECT ` + linkColumns + ` FROM links`
	if onlyEnabled {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY position ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	links := make([]Link, 0, 16)
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// Link loads a single link by id.
func (s *Store) Link(ctx context.Context, id int64) (Link, error) {
	l, err := scanLink(s.db.QueryRowContext(ctx,
		`SELECT `+linkColumns+` FROM links WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("load link: %w", err)
	}
	return l, nil
}

// CreateLink appends a link to the end of the list.
func (s *Store) CreateLink(ctx context.Context, in LinkInput) (Link, error) {
	var next int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM links`).Scan(&next); err != nil {
		return Link{}, fmt.Errorf("next position: %w", err)
	}

	now := nowUTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO links (title, url, icon, enabled, position, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.Title, in.URL, in.Icon, boolToInt(in.Enabled), next, now, now)
	if err != nil {
		return Link{}, fmt.Errorf("insert link: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Link{}, fmt.Errorf("insert link id: %w", err)
	}
	return s.Link(ctx, id)
}

// UpdateLink rewrites the editable fields of an existing link.
func (s *Store) UpdateLink(ctx context.Context, id int64, in LinkInput) (Link, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE links SET title = ?, url = ?, icon = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		in.Title, in.URL, in.Icon, boolToInt(in.Enabled), nowUTC(), id)
	if err != nil {
		return Link{}, fmt.Errorf("update link: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Link{}, ErrNotFound
	}
	return s.Link(ctx, id)
}

// DeleteLink removes a link and closes the gap in the ordering.
func (s *Store) DeleteLink(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := resequence(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ReorderLinks applies a new ordering. ids must contain exactly the ids that
// currently exist, which makes the operation safe to replay and impossible to
// use for partial (and therefore ambiguous) reordering.
func (s *Store) ReorderLinks(ctx context.Context, ids []int64) ([]Link, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin reorder: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM links`)
	if err != nil {
		return nil, fmt.Errorf("load link ids: %w", err)
	}
	existing := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan link id: %w", err)
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) != len(existing) {
		return nil, fmt.Errorf("%w: expected %d ids, got %d", ErrInvalid, len(existing), len(ids))
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if !existing[id] {
			return nil, fmt.Errorf("%w: unknown link id %d", ErrInvalid, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: duplicate link id %d", ErrInvalid, id)
		}
		seen[id] = true
	}

	now := nowUTC()
	stmt, err := tx.PrepareContext(ctx, `UPDATE links SET position = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return nil, fmt.Errorf("prepare reorder: %w", err)
	}
	defer stmt.Close()
	for pos, id := range ids {
		if _, err := stmt.ExecContext(ctx, pos, now, id); err != nil {
			return nil, fmt.Errorf("reorder link %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reorder: %w", err)
	}
	return s.Links(ctx, false)
}

// resequence rewrites positions to 0..n-1 preserving the current order.
func resequence(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM links ORDER BY position ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("resequence read: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for pos, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE links SET position = ? WHERE id = ?`, pos, id); err != nil {
			return fmt.Errorf("resequence write: %w", err)
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
