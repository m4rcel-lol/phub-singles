package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Badge IDs, mirrored by the frontend.
const (
	BadgeOwner    = "owner"
	BadgeAdmin    = "admin"
	BadgeVerified = "verified"
)

// Badge is one marker shown next to the display name on the public page.
type Badge struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Title is the tooltip: it says why the badge is there.
	Title string `json:"title"`
}

// BadgeState is the badge situation of one account, including the parts the
// admin panel needs to explain what is going on.
type BadgeState struct {
	// OwnerID is the account the page belongs to, or 0 when it has none.
	OwnerID      int64   `json:"ownerId"`
	Badges       []Badge `json:"badges"`
	ManualGrant  bool    `json:"manualGrant"`  // Verified was granted by hand
	AutoVerified bool    `json:"autoVerified"` // Verified unlocked by view count
	Views        int64   `json:"views"`        // views counted towards the threshold
	Threshold    int64   `json:"threshold"`
}

// ProfileBadges returns the badges shown on the public page: they belong to the
// account that owns it.
func (s *Store) ProfileBadges(ctx context.Context) ([]Badge, error) {
	state, err := s.ProfileBadgeState(ctx)
	if err != nil {
		return nil, err
	}
	return state.Badges, nil
}

// ProfileBadgeState resolves the owning account and computes its badges.
func (s *Store) ProfileBadgeState(ctx context.Context) (BadgeState, error) {
	settings, err := s.SiteSettings(ctx)
	if err != nil {
		return BadgeState{}, err
	}
	state := BadgeState{Badges: []Badge{}, Threshold: settings.VerifiedThreshold}

	var userID sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM profile WHERE id = 1`).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, nil
		}
		return state, fmt.Errorf("load profile owner: %w", err)
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT value FROM metrics WHERE key = 'page_views'), 0)`).
		Scan(&state.Views); err != nil {
		return state, fmt.Errorf("load view count: %w", err)
	}
	state.AutoVerified = state.Views >= state.Threshold

	state.OwnerID = userID.Int64
	if !userID.Valid {
		// No account owns the page yet (only possible before the first start
		// finishes); the view-based badge still applies.
		state.Badges = assembleBadges("", state)
		return state, nil
	}

	var role, verifiedAt string
	err = s.db.QueryRowContext(ctx,
		`SELECT role, COALESCE(verified_at, '') FROM users WHERE id = ?`, userID.Int64).
		Scan(&role, &verifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		state.Badges = assembleBadges("", state)
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("load profile owner role: %w", err)
	}

	state.ManualGrant = verifiedAt != ""
	state.Badges = assembleBadges(role, state)
	return state, nil
}

// UserBadges computes the badges of an arbitrary account. Only the account that
// owns the page can reach the Verified badge through the view threshold.
func (s *Store) UserBadges(u User, ownsProfile bool, views, threshold int64) []Badge {
	state := BadgeState{
		ManualGrant:  u.VerifiedAt != "",
		AutoVerified: ownsProfile && views >= threshold,
		Views:        views,
		Threshold:    threshold,
	}
	return assembleBadges(u.Role, state)
}

// assembleBadges is the single place that decides which badges exist and in
// which order they are shown.
func assembleBadges(role string, state BadgeState) []Badge {
	badges := make([]Badge, 0, 2)

	if state.ManualGrant || state.AutoVerified {
		title := "Verified by an administrator"
		if !state.ManualGrant {
			title = fmt.Sprintf("Verified automatically after %d page views", state.Threshold)
		}
		badges = append(badges, Badge{ID: BadgeVerified, Label: "Verified", Title: title})
	}

	switch role {
	case RoleOwner:
		badges = append(badges, Badge{
			ID: BadgeOwner, Label: "Owner", Title: "Owner of pornhub.singles",
		})
	case RoleAdmin:
		badges = append(badges, Badge{
			ID: BadgeAdmin, Label: "Administrator", Title: "Administrator of pornhub.singles",
		})
	}
	return badges
}
