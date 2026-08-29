-- A profile belongs to every account.  Keep the old singleton table during the
-- transition so existing installs retain their published owner page exactly as
-- it was; new code reads this table instead.
CREATE TABLE IF NOT EXISTS profiles (
    user_id       INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    display_name  TEXT    NOT NULL DEFAULT '',
    tagline       TEXT    NOT NULL DEFAULT '',
    bio           TEXT    NOT NULL DEFAULT '',
    avatar_url    TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Existing installations already have one page.  Migrate it to the account
-- that owned it before this release.
INSERT OR IGNORE INTO profiles (user_id, username, display_name, tagline, bio, avatar_url, updated_at)
SELECT user_id, username, display_name, tagline, bio, avatar_url, updated_at
FROM profile
WHERE id = 1 AND user_id IS NOT NULL;
