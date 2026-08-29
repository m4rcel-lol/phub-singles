package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Settings keys. They share the settings table with server-generated secrets,
// which is why every one of them is namespaced.
const (
	keyHeadline           = "site.headline"
	keyLede               = "site.lede"
	keyVerifiedThreshold  = "site.verified_threshold"
	keyMaintenance        = "site.maintenance"
	keyMaintenanceMessage = "site.maintenance_message"
	keyIndexing           = "site.indexing"
)

// Defaults are what a fresh install shows; they are also the fallback whenever
// a stored value fails to parse, so a bad row can never take the site down.
const (
	DefaultHeadline = "Every link. One page."
	DefaultLede     = "A single, fast, self-hosted bio page. No feeds, no trackers, no clutter — " +
		"just the links that matter, in the order you choose."
	DefaultMaintenanceMessage = "We are doing a bit of maintenance. Back shortly."
	// DefaultVerifiedThreshold is the view count that unlocks Verified without
	// an administrator having to hand it out.
	DefaultVerifiedThreshold int64 = 10_000
)

// Field limits, enforced here so the CLI and the API cannot disagree.
const (
	MaxHeadline           = 80
	MaxLede               = 240
	MaxMaintenanceMessage = 200
	MinVerifiedThreshold  = 1
	MaxVerifiedThreshold  = 100_000_000
)

// SiteSettings are the owner-controlled knobs that are not deployment concerns:
// anything an operator would want to change without editing .env and
// redeploying lives here.
type SiteSettings struct {
	Headline           string `json:"headline"`
	Lede               string `json:"lede"`
	VerifiedThreshold  int64  `json:"verifiedThreshold"`
	Maintenance        bool   `json:"maintenance"`
	MaintenanceMessage string `json:"maintenanceMessage"`
	Indexing           bool   `json:"indexing"`
}

// DefaultSiteSettings is the starting point for a fresh database.
func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		Headline:           DefaultHeadline,
		Lede:               DefaultLede,
		VerifiedThreshold:  DefaultVerifiedThreshold,
		Maintenance:        false,
		MaintenanceMessage: DefaultMaintenanceMessage,
		Indexing:           true,
	}
}

// SiteSettings reads the stored settings, falling back to the defaults for
// anything missing or unparseable.
func (s *Store) SiteSettings(ctx context.Context) (SiteSettings, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM settings WHERE key LIKE 'site.%'`)
	if err != nil {
		return SiteSettings{}, fmt.Errorf("load site settings: %w", err)
	}
	defer rows.Close()

	stored := make(map[string]string, 6)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return SiteSettings{}, fmt.Errorf("scan site setting: %w", err)
		}
		stored[key] = value
	}
	if err := rows.Err(); err != nil {
		return SiteSettings{}, err
	}

	settings := DefaultSiteSettings()
	if v, ok := stored[keyHeadline]; ok && v != "" {
		settings.Headline = v
	}
	if v, ok := stored[keyLede]; ok && v != "" {
		settings.Lede = v
	}
	if v, ok := stored[keyMaintenanceMessage]; ok && v != "" {
		settings.MaintenanceMessage = v
	}
	if v, ok := stored[keyVerifiedThreshold]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= MinVerifiedThreshold {
			settings.VerifiedThreshold = n
		}
	}
	if v, ok := stored[keyMaintenance]; ok {
		settings.Maintenance = v == "true"
	}
	if v, ok := stored[keyIndexing]; ok {
		settings.Indexing = v != "false"
	}
	return settings, nil
}

// UpdateSiteSettings validates and stores every field in one transaction.
func (s *Store) UpdateSiteSettings(ctx context.Context, in SiteSettings) (SiteSettings, error) {
	in.Headline = strings.TrimSpace(in.Headline)
	in.Lede = strings.TrimSpace(in.Lede)
	in.MaintenanceMessage = strings.TrimSpace(in.MaintenanceMessage)

	if in.Headline == "" {
		return SiteSettings{}, fmt.Errorf("%w: a headline is required", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Headline) > MaxHeadline {
		return SiteSettings{}, fmt.Errorf("%w: headline is too long", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Lede) > MaxLede {
		return SiteSettings{}, fmt.Errorf("%w: intro text is too long", ErrInvalid)
	}
	if utf8.RuneCountInString(in.MaintenanceMessage) > MaxMaintenanceMessage {
		return SiteSettings{}, fmt.Errorf("%w: maintenance message is too long", ErrInvalid)
	}
	if in.MaintenanceMessage == "" {
		in.MaintenanceMessage = DefaultMaintenanceMessage
	}
	if in.VerifiedThreshold < MinVerifiedThreshold || in.VerifiedThreshold > MaxVerifiedThreshold {
		return SiteSettings{}, fmt.Errorf("%w: the verification threshold must be between %d and %d",
			ErrInvalid, MinVerifiedThreshold, MaxVerifiedThreshold)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SiteSettings{}, fmt.Errorf("begin settings update: %w", err)
	}
	defer tx.Rollback()

	for key, value := range map[string]string{
		keyHeadline:           in.Headline,
		keyLede:               in.Lede,
		keyMaintenanceMessage: in.MaintenanceMessage,
		keyVerifiedThreshold:  strconv.FormatInt(in.VerifiedThreshold, 10),
		keyMaintenance:        strconv.FormatBool(in.Maintenance),
		keyIndexing:           strconv.FormatBool(in.Indexing),
	} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			return SiteSettings{}, fmt.Errorf("store setting %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return SiteSettings{}, fmt.Errorf("commit settings update: %w", err)
	}
	return s.SiteSettings(ctx)
}
