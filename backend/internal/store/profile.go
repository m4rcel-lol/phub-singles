package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Profile is the single bio-page identity shown above the links.
type Profile struct {
	// Username is the page handle: the site serves this profile at /<username>.
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Tagline     string `json:"tagline"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatarUrl"`
	UpdatedAt   string `json:"updatedAt"`
}

// Profile returns the singleton profile row, creating it if it is missing.
func (s *Store) Profile(ctx context.Context) (Profile, error) {
	var p Profile
	err := s.db.QueryRowContext(ctx,
		`SELECT username, display_name, tagline, bio, avatar_url, updated_at FROM profile WHERE id = 1`).
		Scan(&p.Username, &p.DisplayName, &p.Tagline, &p.Bio, &p.AvatarURL, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO profile (id, updated_at) VALUES (1, ?)`, nowUTC()); err != nil {
			return p, fmt.Errorf("create profile row: %w", err)
		}
		return s.Profile(ctx)
	}
	if err != nil {
		return p, fmt.Errorf("load profile: %w", err)
	}
	return p, nil
}

// UpdateProfile overwrites the text fields of the profile. The avatar is
// managed separately by SetAvatarURL so an upload cannot be clobbered by a
// concurrent text edit.
func (s *Store) UpdateProfile(ctx context.Context, p Profile) (Profile, error) {
	if _, err := s.Profile(ctx); err != nil { // ensures the row exists
		return Profile{}, err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE profile SET username = ?, display_name = ?, tagline = ?, bio = ?, updated_at = ?
		 WHERE id = 1`,
		p.Username, p.DisplayName, p.Tagline, p.Bio, nowUTC())
	if err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return s.Profile(ctx)
}

// SetAvatarURL points the profile at a new avatar and returns the previous
// value so the caller can clean up the replaced file.
func (s *Store) SetAvatarURL(ctx context.Context, avatarURL string) (previous string, err error) {
	current, err := s.Profile(ctx)
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE profile SET avatar_url = ?, updated_at = ? WHERE id = 1`,
		avatarURL, nowUTC()); err != nil {
		return "", fmt.Errorf("update avatar: %w", err)
	}
	return current.AvatarURL, nil
}
