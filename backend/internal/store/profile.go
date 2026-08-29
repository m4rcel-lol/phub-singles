package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Profile is one account's public bio-page identity.
type Profile struct {
	// Username is the page handle: the site serves this profile at /<username>.
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Tagline     string `json:"tagline"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatarUrl"`
	UpdatedAt   string `json:"updatedAt"`
}

const profileColumns = `username, display_name, tagline, bio, avatar_url, updated_at`

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	err := row.Scan(&p.Username, &p.DisplayName, &p.Tagline, &p.Bio, &p.AvatarURL, &p.UpdatedAt)
	return p, err
}

// Profile retains the old store contract by returning the owner's profile.
// It is used by legacy administrative reporting; public and self-service paths
// should use ProfileByHandle and ProfileByUser instead.
func (s *Store) Profile(ctx context.Context) (Profile, error) {
	var ownerID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM profile WHERE id = 1`).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("load owner profile: %w", err)
	}
	return s.ProfileByUser(ctx, ownerID)
}

// ProfileByUser returns the profile owned by one account.
func (s *Store) ProfileByUser(ctx context.Context, userID int64) (Profile, error) {
	p, err := scanProfile(s.db.QueryRowContext(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE user_id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("load profile: %w", err)
	}
	return p, nil
}

// ProfileByHandle resolves the public page address.
func (s *Store) ProfileByHandle(ctx context.Context, handle string) (Profile, error) {
	p, err := scanProfile(s.db.QueryRowContext(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE username = ? COLLATE NOCASE`, handle))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("load profile by handle: %w", err)
	}
	return p, nil
}

// UpdateProfile overwrites one account's text fields. The avatar is
// managed separately by SetAvatarURL so an upload cannot be clobbered by a
// concurrent text edit.
func (s *Store) UpdateProfile(ctx context.Context, userID int64, p Profile) (Profile, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE profiles SET username = ?, display_name = ?, tagline = ?, bio = ?, updated_at = ?
		 WHERE user_id = ?`,
		p.Username, p.DisplayName, p.Tagline, p.Bio, nowUTC(), userID)
	if err != nil {
		if isUniqueConstraint(err) {
			return Profile{}, fmt.Errorf("%w: handle is already taken", ErrConflict)
		}
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return s.ProfileByUser(ctx, userID)
}

// SetAvatarURL points the profile at a new avatar and returns the previous
// value so the caller can clean up the replaced file.
func (s *Store) SetAvatarURL(ctx context.Context, userID int64, avatarURL string) (previous string, err error) {
	current, err := s.ProfileByUser(ctx, userID)
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE profiles SET avatar_url = ?, updated_at = ? WHERE user_id = ?`,
		avatarURL, nowUTC(), userID); err != nil {
		return "", fmt.Errorf("update avatar: %w", err)
	}
	return current.AvatarURL, nil
}
