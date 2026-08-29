package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Stats is the payload behind the admin dashboard.
type Stats struct {
	TotalViews  int64      `json:"totalViews"`
	TotalClicks int64      `json:"totalClicks"`
	TotalLinks  int        `json:"totalLinks"`
	ActiveLinks int        `json:"activeLinks"`
	PerLink     []LinkStat `json:"perLink"`
	Daily       []DayStat  `json:"daily"`
}

// LinkStat is the click total for one link.
type LinkStat struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Icon    string `json:"icon"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Clicks  int64  `json:"clicks"`
}

// DayStat is a single bucket of the activity chart.
type DayStat struct {
	Day    string `json:"day"`
	Views  int64  `json:"views"`
	Clicks int64  `json:"clicks"`
}

// RegisterView records one page view (global counter plus today's bucket).
func (s *Store) RegisterView(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin view: %w", err)
	}
	defer tx.Rollback()

	if err := bumpMetric(ctx, tx, "page_views"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO daily_stats (day, views, clicks) VALUES (?, 1, 0)
		 ON CONFLICT(day) DO UPDATE SET views = views + 1`, today()); err != nil {
		return fmt.Errorf("bump daily views: %w", err)
	}
	return tx.Commit()
}

// RegisterClick records a click on an enabled link. Clicks on unknown or
// disabled links return ErrNotFound so the API cannot be used to probe them.
func (s *Store) RegisterClick(ctx context.Context, id int64) (Link, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Link{}, fmt.Errorf("begin click: %w", err)
	}
	defer tx.Rollback()

	var url string
	err = tx.QueryRowContext(ctx, `SELECT url FROM links WHERE id = ? AND enabled = 1`, id).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("load link for click: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE links SET clicks = clicks + 1 WHERE id = ?`, id); err != nil {
		return Link{}, fmt.Errorf("bump link clicks: %w", err)
	}
	if err := bumpMetric(ctx, tx, "link_clicks"); err != nil {
		return Link{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO daily_stats (day, views, clicks) VALUES (?, 0, 1)
		 ON CONFLICT(day) DO UPDATE SET clicks = clicks + 1`, today()); err != nil {
		return Link{}, fmt.Errorf("bump daily clicks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Link{}, fmt.Errorf("commit click: %w", err)
	}
	return s.Link(ctx, id)
}

// Stats aggregates the numbers shown on the dashboard. days limits the
// activity chart window.
func (s *Store) Stats(ctx context.Context, days int) (Stats, error) {
	if days <= 0 || days > 365 {
		days = 14
	}
	var st Stats

	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT value FROM metrics WHERE key = 'page_views'), 0),
		        COALESCE((SELECT value FROM metrics WHERE key = 'link_clicks'), 0)`).
		Scan(&st.TotalViews, &st.TotalClicks); err != nil {
		return st, fmt.Errorf("load metrics: %w", err)
	}

	links, err := s.Links(ctx, false)
	if err != nil {
		return st, err
	}
	st.TotalLinks = len(links)
	st.PerLink = make([]LinkStat, 0, len(links))
	for _, l := range links {
		if l.Enabled {
			st.ActiveLinks++
		}
		st.PerLink = append(st.PerLink, LinkStat{
			ID: l.ID, Title: l.Title, Icon: l.Icon, URL: l.URL,
			Enabled: l.Enabled, Clicks: l.Clicks,
		})
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT day, views, clicks FROM daily_stats
		 WHERE day >= date('now', ?) ORDER BY day ASC`,
		fmt.Sprintf("-%d days", days-1))
	if err != nil {
		return st, fmt.Errorf("load daily stats: %w", err)
	}
	defer rows.Close()

	st.Daily = make([]DayStat, 0, days)
	for rows.Next() {
		var d DayStat
		if err := rows.Scan(&d.Day, &d.Views, &d.Clicks); err != nil {
			return st, fmt.Errorf("scan daily stat: %w", err)
		}
		st.Daily = append(st.Daily, d)
	}
	return st, rows.Err()
}

func bumpMetric(ctx context.Context, tx *sql.Tx, key string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO metrics (key, value) VALUES (?, 1)
		 ON CONFLICT(key) DO UPDATE SET value = value + 1`, key); err != nil {
		return fmt.Errorf("bump metric %s: %w", key, err)
	}
	return nil
}

// ClaimEvent reserves a counting slot for a fingerprint. It returns true the
// first time a fingerprint is seen and false again until the window elapses,
// which is what stops a visitor from inflating the counters by reloading.
//
// The insert and the expiry check happen in one statement, so two concurrent
// requests from the same visitor can never both win.
func (s *Store) ClaimEvent(ctx context.Context, fingerprint string, window time.Duration) (bool, error) {
	now := time.Now().UTC()
	expires := now.Add(window).Format("2006-01-02T15:04:05Z")

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO view_events (fingerprint, expires_at) VALUES (?, ?)
		 ON CONFLICT(fingerprint) DO UPDATE SET expires_at = excluded.expires_at
		 WHERE view_events.expires_at <= ?`,
		fingerprint, expires, now.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return false, fmt.Errorf("claim event: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim event result: %w", err)
	}
	return n > 0, nil
}

// PurgeExpiredEvents drops fingerprints whose window has passed. Called on the
// maintenance ticker so the table stays small and nothing is kept for long.
func (s *Store) PurgeExpiredEvents(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM view_events WHERE expires_at <= ?`,
		time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return 0, fmt.Errorf("purge view events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// SetPageViewsForTest overwrites the page-view counter. It exists so tests can
// reach the automatic-verification threshold without ten thousand requests.
func (s *Store) SetPageViewsForTest(ctx context.Context, views int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metrics (key, value) VALUES ('page_views', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, views)
	return err
}
