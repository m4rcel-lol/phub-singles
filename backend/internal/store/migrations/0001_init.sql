-- Core schema for pornhub.singles.
-- The application is single-user by design: exactly one profile row (id = 1)
-- and a small set of admin accounts (normally one) that can manage it.

CREATE TABLE IF NOT EXISTS profile (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    display_name  TEXT    NOT NULL DEFAULT '',
    tagline       TEXT    NOT NULL DEFAULT '',
    bio           TEXT    NOT NULL DEFAULT '',
    avatar_url    TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS links (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    url         TEXT    NOT NULL,
    icon        TEXT    NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    position    INTEGER NOT NULL DEFAULT 0,
    clicks      INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Public page reads links ordered by position; keep that path index-backed.
CREATE INDEX IF NOT EXISTS idx_links_position ON links (position, id);

CREATE TABLE IF NOT EXISTS users (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    username       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash  TEXT    NOT NULL,
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Only the SHA-256 of the session token is stored, so a database leak does not
-- hand out live sessions.
CREATE TABLE IF NOT EXISTS sessions (
    token_hash  TEXT    PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at  TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions (expires_at);

-- Simple counter bag (page_views, link_clicks, ...).
CREATE TABLE IF NOT EXISTS metrics (
    key    TEXT    PRIMARY KEY,
    value  INTEGER NOT NULL DEFAULT 0
);

-- Per-day rollup powering the small activity chart in the admin dashboard.
CREATE TABLE IF NOT EXISTS daily_stats (
    day     TEXT    PRIMARY KEY,
    views   INTEGER NOT NULL DEFAULT 0,
    clicks  INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO metrics (key, value) VALUES ('page_views', 0), ('link_clicks', 0);
