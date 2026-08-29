package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// Setting reads a server setting, returning "" when it is absent.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", key, err)
	}
	return value, nil
}

// EnsureSecret returns the named secret, generating and storing it on first
// use. Rotating a secret is a matter of deleting the row.
func (s *Store) EnsureSecret(ctx context.Context, key string) (string, error) {
	if existing, err := s.Setting(ctx, key); err != nil {
		return "", err
	} else if existing != "" {
		return existing, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	secret := hex.EncodeToString(buf)

	// Another process may have won the race; keep whichever landed first.
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`,
		key, secret); err != nil {
		return "", fmt.Errorf("store secret %s: %w", key, err)
	}
	return s.Setting(ctx, key)
}
