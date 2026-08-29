-- Accounts gain roles and badge state.
--
--   owner   exactly one; can only be changed from the CLI, never over HTTP
--   admin   can manage the page and hand out the Verified badge
--   member  an account without privileges (a demoted admin)
--
-- The Verified badge is either granted by hand or unlocked automatically once
-- the account's page passes the view threshold; see store.Badges.

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'
    CHECK (role IN ('owner', 'admin', 'member'));
ALTER TABLE users ADD COLUMN verified_at TEXT;
ALTER TABLE users ADD COLUMN verified_by TEXT NOT NULL DEFAULT '';

-- The single profile belongs to an account; its badges are that account's.
ALTER TABLE profile ADD COLUMN user_id INTEGER REFERENCES users (id) ON DELETE SET NULL;

-- Existing installs: the first account created is the owner and owns the page.
UPDATE users SET role = 'owner' WHERE id = (SELECT MIN(id) FROM users);
UPDATE profile SET user_id = (SELECT MIN(id) FROM users)
WHERE id = 1 AND user_id IS NULL AND EXISTS (SELECT 1 FROM users);

-- Small key/value bag for server-generated secrets (see view fingerprinting).
CREATE TABLE IF NOT EXISTS settings (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);
